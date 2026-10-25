package notification

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
)

// fakeSMTP is a minimal SMTP server that records the messages it receives.
type fakeSMTP struct {
	host string
	port int

	mu       sync.Mutex
	messages []string
	listener net.Listener
}

// startFakeSMTP listens on a random loopback port until the test ends.
func startFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	address := listener.Addr().(*net.TCPAddr)
	server := &fakeSMTP{host: "127.0.0.1", port: address.Port, listener: listener}

	go server.serve()
	t.Cleanup(func() { _ = listener.Close() })

	return server
}

func (s *fakeSMTP) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeSMTP) handle(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	write := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }

	write("220 fake ESMTP")

	var body bytes.Buffer
	inData := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		if inData {
			if line == "." {
				inData = false
				s.mu.Lock()
				s.messages = append(s.messages, body.String())
				s.mu.Unlock()
				body.Reset()
				write("250 OK")
				continue
			}
			body.WriteString(line + "\n")
			continue
		}

		switch {
		case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
			// AUTH PLAIN over loopback is accepted by net/smtp without TLS.
			write("250-fake")
			write("250 AUTH PLAIN")
		case strings.HasPrefix(line, "AUTH"):
			write("235 Authentication successful")
		case strings.HasPrefix(line, "MAIL FROM"), strings.HasPrefix(line, "RCPT TO"):
			write("250 OK")
		case strings.HasPrefix(line, "DATA"):
			inData = true
			write("354 End data with <CR><LF>.<CR><LF>")
		case strings.HasPrefix(line, "QUIT"):
			write("221 Bye")
			return
		default:
			write("250 OK")
		}
	}
}

func (s *fakeSMTP) received() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.messages...)
}

func newTestMail(t *testing.T) (Mail, *fakeSMTP) {
	t.Helper()

	server := startFakeSMTP(t)
	mail := NewMail(MailParams{
		SMTPServerAddress: server.host,
		SMTPServerPort:    server.port,
		To:                "user@example.com",
		From:              "bot@example.com",
		Password:          "secret",
	})

	return mail, server
}

func TestNewMail(t *testing.T) {
	mail := NewMail(MailParams{
		SMTPServerAddress: "smtp.example.com",
		SMTPServerPort:    587,
		To:                "user@example.com",
		From:              "bot@example.com",
		Password:          "secret",
	})

	require.Equal(t, "smtp.example.com", mail.smtpServerAddress)
	require.Equal(t, 587, mail.smtpServerPort)
	require.Equal(t, "user@example.com", mail.to)
	require.Equal(t, "bot@example.com", mail.from)
	require.NotNil(t, mail.auth)
}

func TestMailNotify(t *testing.T) {
	t.Run("delivers the message to the SMTP server", func(t *testing.T) {
		mail, server := newTestMail(t)

		mail.Notify("hello")

		messages := server.received()
		require.Len(t, messages, 1)
		require.Contains(t, messages[0], "hello")
		require.Contains(t, messages[0], "user@example.com")
		require.Contains(t, messages[0], "bot@example.com")
	})

	t.Run("logs an error when the server is unreachable", func(t *testing.T) {
		var buffer bytes.Buffer
		logger := logrus.StandardLogger()
		previous := logger.Out
		logger.Out = &buffer
		t.Cleanup(func() { logger.Out = previous })

		mail := NewMail(MailParams{
			SMTPServerAddress: "127.0.0.1",
			SMTPServerPort:    1, // nothing is listening here
			To:                "user@example.com",
			From:              "bot@example.com",
		})

		mail.Notify("hello")

		require.Contains(t, buffer.String(), "couldnt send mail")
	})
}

func TestMailOnOrder(t *testing.T) {
	tests := []struct {
		name      string
		status    model.OrderStatusType
		wantTitle string
	}{
		{"filled", model.OrderStatusTypeFilled, "✅ ORDER FILLED - BTCUSDT"},
		{"new", model.OrderStatusTypeNew, "🆕 NEW ORDER - BTCUSDT"},
		{"canceled", model.OrderStatusTypeCanceled, "❌ ORDER CANCELED / REJECTED - BTCUSDT"},
		{"rejected", model.OrderStatusTypeRejected, "❌ ORDER CANCELED / REJECTED - BTCUSDT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mail, server := newTestMail(t)

			mail.OnOrder(model.Order{
				ID:       1,
				Pair:     "BTCUSDT",
				Side:     model.SideTypeBuy,
				Type:     model.OrderTypeLimit,
				Status:   tt.status,
				Price:    100,
				Quantity: 1,
			})

			messages := server.received()
			require.Len(t, messages, 1)
			require.Contains(t, messages[0], "Subject: "+tt.wantTitle)
		})
	}

	t.Run("leaves the title empty for unhandled statuses", func(t *testing.T) {
		mail, server := newTestMail(t)

		mail.OnOrder(model.Order{Pair: "BTCUSDT", Status: model.OrderStatusTypePendingCancel})

		messages := server.received()
		require.Len(t, messages, 1)
		require.Contains(t, messages[0], "Subject: \n")
	})
}

func TestMailOnError(t *testing.T) {
	mail, server := newTestMail(t)

	mail.OnError(errors.New("something went wrong"))

	messages := server.received()
	require.Len(t, messages, 1)
	require.Contains(t, messages[0], "Subject: 🛑 ERROR")
	require.Contains(t, messages[0], "something went wrong")
}

// Guards against a port formatting regression in the server address.
func TestMailServerAddress(t *testing.T) {
	mail, server := newTestMail(t)

	mail.Notify("hello")

	require.Contains(t, server.received()[0], "hello")
	require.NotEmpty(t, strconv.Itoa(mail.smtpServerPort))
}
