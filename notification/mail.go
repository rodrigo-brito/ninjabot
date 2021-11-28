package notification

import (
	"fmt"
	"net/smtp"

	log "github.com/sirupsen/logrus"

	"github.com/rodrigo-brito/ninjabot/model"
)

type Mail struct {
	auth smtp.Auth

	smtpServerPort    int
	smtpServerAddress string

	to   string
	from string
}

func (t Mail) Notify(text string) {
	serverAddress := fmt.Sprintf(
		"%s:%d",
		t.smtpServerAddress,
		t.smtpServerPort)

	message := fmt.Sprintf(
		`To: "User" <%s>\nFrom: "NinjaBot" <%s>\n%s`,
		t.to,
		t.from,
		text,
	)

	err := smtp.SendMail(
		serverAddress,
		t.auth,
		t.from,
		[]string{t.to},
		[]byte(message))
	if err != nil {
		log.
			WithError(err).
			Errorf("notification/mail: couldnt send mail")
	}
}

func (t Mail) OnOrder(order model.Order) {
	title := ""
	switch order.Status {
	case model.OrderStatusTypeFilled:
		title = fmt.Sprintf("✅ ORDER FILLED - %s", order.Pair)
	case model.OrderStatusTypeNew:
		title = fmt.Sprintf("🆕 NEW ORDER - %s", order.Pair)
	case model.OrderStatusTypeCanceled, model.OrderStatusTypeRejected:
		title = fmt.Sprintf("❌ ORDER CANCELED / REJECTED - %s", order.Pair)
	}

	message := fmt.Sprintf("Subject: %s\nOrder %s", title, order)

	t.Notify(message)
}

func (t Mail) OnError(err error) {
	message := fmt.Sprintf("Subject: 🛑 ERROR\nError %s", err)
	t.Notify(message)
}

type MailParams struct {
	SMTPServerPort    int
	SMTPServerAddress string

	To       string
	From     string
	Password string
}

func NewMail(params MailParams) Mail {
	return Mail{
		from:              params.From,
		to:                params.To,
		smtpServerPort:    params.SMTPServerPort,
		smtpServerAddress: params.SMTPServerAddress,
		auth: smtp.PlainAuth(
			"",
			params.From,
			params.Password,
			params.SMTPServerAddress,
		),
	}
}
