package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	tb "gopkg.in/tucnak/telebot.v2"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/order"
	"github.com/rodrigo-brito/ninjabot/storage"
	"github.com/rodrigo-brito/ninjabot/testdata/mocks"
)

const telegramUserID = 42

// telegramServer is a fake Telegram Bot API. It records every outgoing call so
// the tests can assert on what the bot replied.
type telegramServer struct {
	*httptest.Server

	mu    sync.Mutex
	calls map[string][]map[string]string

	// commands is what getMyCommands returns.
	commands []tb.Command
	// failOn makes the given method reply with an API error.
	failOn string
}

func newTelegramServer(t *testing.T) *telegramServer {
	t.Helper()

	server := &telegramServer{calls: map[string][]map[string]string{}}
	server.Server = httptest.NewServer(http.HandlerFunc(server.handle))

	previous := telegramAPIURL
	telegramAPIURL = server.URL
	t.Cleanup(func() {
		telegramAPIURL = previous
		server.Close()
	})

	return server
}

func (s *telegramServer) handle(w http.ResponseWriter, r *http.Request) {
	method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]

	body, _ := io.ReadAll(r.Body)
	params := map[string]string{}
	if err := json.Unmarshal(body, &params); err != nil {
		_ = r.ParseForm()
		for key := range r.Form {
			params[key] = r.Form.Get(key)
		}
	}

	s.mu.Lock()
	s.calls[method] = append(s.calls[method], params)
	failOn := s.failOn
	commands := s.commands
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	if failOn == method {
		_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: fake failure"}`))
		return
	}

	switch method {
	case "getMe":
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"username":"ninjabot"}}`))
	case "getMyCommands":
		payload, _ := json.Marshal(commands)
		_, _ = fmt.Fprintf(w, `{"ok":true,"result":%s}`, payload)
	case "getUpdates":
		_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
	default:
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1,"chat":{"id":42},"text":"ok"}}`))
	}
}

// sent returns the text of every message the bot sent.
func (s *telegramServer) sent() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	messages := make([]string, 0, len(s.calls["sendMessage"]))
	for _, call := range s.calls["sendMessage"] {
		messages = append(messages, call["text"])
	}

	return messages
}

func (s *telegramServer) lastSent(t *testing.T) string {
	t.Helper()

	messages := s.sent()
	require.NotEmpty(t, messages, "the bot did not reply")

	return messages[len(messages)-1]
}

// newTestTelegram wires a telegram notifier to a fake API and a controller
// backed by the given exchange mock.
func newTestTelegram(t *testing.T, server *telegramServer, exch *mocks.Exchange) (*telegram, *order.Controller) {
	t.Helper()

	memory, err := storage.FromMemory()
	require.NoError(t, err)

	controller := order.NewController(context.Background(), exch, memory, order.NewOrderFeed())

	settings := model.Settings{
		Pairs: []string{"BTCUSDT"},
		Telegram: model.TelegramSettings{
			Enabled: true,
			Token:   "token",
			Users:   []int{telegramUserID},
		},
	}

	notifier, err := NewTelegram(controller, settings)
	require.NoError(t, err)

	return notifier.(*telegram), controller
}

// message builds an incoming command from the authorized user.
func message(text string) *tb.Message {
	return &tb.Message{
		Text:   text,
		Sender: &tb.User{ID: telegramUserID},
		Chat:   &tb.Chat{ID: telegramUserID},
	}
}

func TestNewTelegram(t *testing.T) {
	t.Run("registers the bot commands", func(t *testing.T) {
		server := newTelegramServer(t)
		newTestTelegram(t, server, &mocks.Exchange{})

		server.mu.Lock()
		defer server.mu.Unlock()
		require.Len(t, server.calls["setMyCommands"], 1)
	})

	t.Run("fails when the token is rejected", func(t *testing.T) {
		server := newTelegramServer(t)
		server.failOn = "getMe"

		_, err := NewTelegram(&order.Controller{}, model.Settings{})

		require.Error(t, err)
	})

	t.Run("fails when the commands cannot be registered", func(t *testing.T) {
		server := newTelegramServer(t)
		server.failOn = "setMyCommands"

		_, err := NewTelegram(&order.Controller{}, model.Settings{})

		require.Error(t, err)
	})

	t.Run("applies the options", func(t *testing.T) {
		server := newTelegramServer(t)

		var applied bool
		_, err := NewTelegram(&order.Controller{}, model.Settings{}, func(*telegram) { applied = true })

		require.NoError(t, err)
		require.True(t, applied)
		require.NotNil(t, server)
	})
}

func TestTelegramStartAndNotify(t *testing.T) {
	t.Run("greets every configured user on start", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, _ := newTestTelegram(t, server, &mocks.Exchange{})

		notifier.Start()

		require.Contains(t, server.sent(), "Bot initialized.")
	})

	t.Run("Notify reaches every configured user", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, _ := newTestTelegram(t, server, &mocks.Exchange{})

		notifier.Notify("hello")

		require.Contains(t, server.sent(), "hello")
	})

	t.Run("logs send failures instead of panicking", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, _ := newTestTelegram(t, server, &mocks.Exchange{})
		server.failOn = "sendMessage"

		require.NotPanics(t, func() {
			notifier.Notify("hello")
			notifier.Start()
		})
	})
}

func TestTelegramBalanceHandle(t *testing.T) {
	t.Run("reports the balance of every pair", func(t *testing.T) {
		server := newTelegramServer(t)
		exch := &mocks.Exchange{}
		exch.On("Account").Return(model.Account{Balances: []model.Balance{
			{Asset: "BTC", Free: 1, Lock: 0.5},
			{Asset: "USDT", Free: 1000},
		}}, nil)
		exch.On("LastQuote", mock.Anything, "BTCUSDT").Return(100.0, nil)

		notifier, _ := newTestTelegram(t, server, exch)
		notifier.BalanceHandle(message("/balance"))

		reply := server.lastSent(t)
		require.Contains(t, reply, "*BALANCE*")
		require.Contains(t, reply, "BTC: `1.5000` ≅ `150.00` USDT")
		require.Contains(t, reply, "USDT: `1000.0000`")
		require.Contains(t, reply, "Total: `1150.0000`")
	})

	t.Run("reports an account failure", func(t *testing.T) {
		server := newTelegramServer(t)
		exch := &mocks.Exchange{}
		exch.On("Account").Return(model.Account{}, errors.New("exchange down"))

		notifier, _ := newTestTelegram(t, server, exch)
		notifier.BalanceHandle(message("/balance"))

		require.Contains(t, server.lastSent(t), "exchange down")
	})

	t.Run("reports a quote failure", func(t *testing.T) {
		server := newTelegramServer(t)
		exch := &mocks.Exchange{}
		exch.On("Account").Return(model.Account{Balances: []model.Balance{{Asset: "BTC", Free: 1}}}, nil)
		exch.On("LastQuote", mock.Anything, "BTCUSDT").Return(0.0, errors.New("no quote"))

		notifier, _ := newTestTelegram(t, server, exch)
		notifier.BalanceHandle(message("/balance"))

		require.Contains(t, server.lastSent(t), "no quote")
	})
}

func TestTelegramHelpHandle(t *testing.T) {
	t.Run("lists the registered commands", func(t *testing.T) {
		server := newTelegramServer(t)
		server.commands = []tb.Command{{Text: "help", Description: "Display help instructions"}}

		notifier, _ := newTestTelegram(t, server, &mocks.Exchange{})
		notifier.HelpHandle(message("/help"))

		require.Contains(t, server.lastSent(t), "/help - Display help instructions")
	})

	t.Run("reports a failure to read the commands", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, _ := newTestTelegram(t, server, &mocks.Exchange{})
		server.failOn = "getMyCommands"

		notifier.HelpHandle(message("/help"))

		require.Contains(t, server.lastSent(t), "ERROR")
	})
}

func TestTelegramProfitHandle(t *testing.T) {
	t.Run("says so when there are no trades", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, _ := newTestTelegram(t, server, &mocks.Exchange{})

		notifier.ProfitHandle(message("/profit"))

		require.Equal(t, "No trades registered.", server.lastSent(t))
	})

	t.Run("summarizes each pair", func(t *testing.T) {
		server := newTelegramServer(t)
		exch := &mocks.Exchange{}
		notifier, controller := newTestTelegram(t, server, exch)

		controller.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 100})

		// A filled order registers a result summary for the pair.
		exch.On("CreateOrderMarket", model.SideTypeBuy, "BTCUSDT", 1.0).
			Return(model.Order{ID: 1, Pair: "BTCUSDT", Side: model.SideTypeBuy, Quantity: 1, Price: 100,
				Status: model.OrderStatusTypeFilled, Type: model.OrderTypeMarket}, nil)
		_, err := controller.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
		require.NoError(t, err)

		notifier.ProfitHandle(message("/profit"))

		require.Contains(t, server.lastSent(t), "*PAIR*: `BTCUSDT`")
	})
}

func TestTelegramStatusHandle(t *testing.T) {
	server := newTelegramServer(t)
	notifier, controller := newTestTelegram(t, server, &mocks.Exchange{})

	controller.Start()
	defer controller.Stop()

	notifier.StatusHandle(message("/status"))

	require.Equal(t, "Status: `running`", server.lastSent(t))
}

func TestTelegramStartStopHandle(t *testing.T) {
	t.Run("starts a stopped bot", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, controller := newTestTelegram(t, server, &mocks.Exchange{})

		notifier.StartHandle(message("/start"))
		defer controller.Stop()

		require.Equal(t, "Bot started.", server.lastSent(t))
		require.Equal(t, order.StatusRunning, controller.Status())
	})

	t.Run("says the bot is already running", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, controller := newTestTelegram(t, server, &mocks.Exchange{})

		controller.Start()
		defer controller.Stop()
		notifier.StartHandle(message("/start"))

		require.Equal(t, "Bot is already running.", server.lastSent(t))
	})

	t.Run("stops a running bot and warns about the open position", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, controller := newTestTelegram(t, server, &mocks.Exchange{})

		controller.Start()
		notifier.StopHandle(message("/stop"))

		require.Contains(t, server.lastSent(t), "Bot stopped.")
		require.Contains(t, server.lastSent(t), "left unprotected")
		require.Equal(t, order.StatusStopped, controller.Status())
	})

	t.Run("says the bot is already stopped", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, controller := newTestTelegram(t, server, &mocks.Exchange{})

		controller.Start()
		controller.Stop()
		notifier.StopHandle(message("/stop"))

		require.Equal(t, "Bot is already stopped.", server.lastSent(t))
	})
}

func TestTelegramBuyHandle(t *testing.T) {
	t.Run("creates a market order for a fixed amount", func(t *testing.T) {
		server := newTelegramServer(t)
		exch := &mocks.Exchange{}
		exch.On("CreateOrderMarketQuote", model.SideTypeBuy, "BTCUSDT", 100.0).
			Return(model.Order{ID: 1, Pair: "BTCUSDT", Side: model.SideTypeBuy, Quantity: 1, Price: 100,
				Status: model.OrderStatusTypeFilled, Type: model.OrderTypeMarket}, nil)

		notifier, _ := newTestTelegram(t, server, exch)
		notifier.BuyHandle(message("/buy BTCUSDT 100"))

		exch.AssertExpectations(t)
	})

	t.Run("converts a percentage of the quote balance", func(t *testing.T) {
		server := newTelegramServer(t)
		exch := &mocks.Exchange{}
		exch.On("Position", "BTCUSDT").Return(0.0, 1000.0, nil)
		exch.On("CreateOrderMarketQuote", model.SideTypeBuy, "BTCUSDT", 500.0).
			Return(model.Order{ID: 1, Pair: "BTCUSDT", Side: model.SideTypeBuy, Quantity: 5, Price: 100,
				Status: model.OrderStatusTypeFilled, Type: model.OrderTypeMarket}, nil)

		notifier, _ := newTestTelegram(t, server, exch)
		notifier.BuyHandle(message("/buy BTCUSDT 50%"))

		exch.AssertExpectations(t)
	})

	t.Run("rejects a malformed command", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, _ := newTestTelegram(t, server, &mocks.Exchange{})

		notifier.BuyHandle(message("/buy"))

		require.Contains(t, server.lastSent(t), "Invalid command.")
	})

	t.Run("rejects a zero amount", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, _ := newTestTelegram(t, server, &mocks.Exchange{})

		notifier.BuyHandle(message("/buy BTCUSDT 0"))

		require.Equal(t, "Invalid amount", server.lastSent(t))
	})

	t.Run("reports a position failure", func(t *testing.T) {
		server := newTelegramServer(t)
		exch := &mocks.Exchange{}
		exch.On("Position", "BTCUSDT").Return(0.0, 0.0, errors.New("no position"))

		notifier, _ := newTestTelegram(t, server, exch)
		notifier.BuyHandle(message("/buy BTCUSDT 50%"))

		require.Contains(t, server.lastSent(t), "no position")
	})

	t.Run("warns when the bot is stopped", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, controller := newTestTelegram(t, server, &mocks.Exchange{})

		controller.Start()
		controller.Stop()
		notifier.BuyHandle(message("/buy BTCUSDT 100"))

		require.Contains(t, server.lastSent(t), "Bot is stopped, no order was created.")
	})

	t.Run("logs other order failures", func(t *testing.T) {
		server := newTelegramServer(t)
		exch := &mocks.Exchange{}
		exch.On("CreateOrderMarketQuote", model.SideTypeBuy, "BTCUSDT", 100.0).
			Return(model.Order{}, errors.New("insufficient funds"))

		notifier, _ := newTestTelegram(t, server, exch)
		before := len(server.sent())
		notifier.BuyHandle(message("/buy BTCUSDT 100"))

		require.Len(t, server.sent(), before, "a plain order failure is only logged")
	})
}

func TestTelegramSellHandle(t *testing.T) {
	t.Run("creates a market order for a fixed amount", func(t *testing.T) {
		server := newTelegramServer(t)
		exch := &mocks.Exchange{}
		exch.On("CreateOrderMarketQuote", model.SideTypeSell, "BTCUSDT", 100.0).
			Return(model.Order{ID: 1, Pair: "BTCUSDT", Side: model.SideTypeSell, Quantity: 1, Price: 100,
				Status: model.OrderStatusTypeFilled, Type: model.OrderTypeMarket}, nil)

		notifier, _ := newTestTelegram(t, server, exch)
		notifier.SellHandle(message("/sell BTCUSDT 100"))

		exch.AssertExpectations(t)
	})

	t.Run("converts a percentage of the asset balance", func(t *testing.T) {
		server := newTelegramServer(t)
		exch := &mocks.Exchange{}
		exch.On("Position", "BTCUSDT").Return(2.0, 0.0, nil)
		exch.On("CreateOrderMarket", model.SideTypeSell, "BTCUSDT", 1.0).
			Return(model.Order{ID: 1, Pair: "BTCUSDT", Side: model.SideTypeSell, Quantity: 1, Price: 100,
				Status: model.OrderStatusTypeFilled, Type: model.OrderTypeMarket}, nil)

		notifier, _ := newTestTelegram(t, server, exch)
		notifier.SellHandle(message("/sell BTCUSDT 50%"))

		exch.AssertExpectations(t)
	})

	t.Run("rejects a malformed command", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, _ := newTestTelegram(t, server, &mocks.Exchange{})

		notifier.SellHandle(message("/sell"))

		require.Contains(t, server.lastSent(t), "Invalid command.")
	})

	t.Run("rejects a zero amount", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, _ := newTestTelegram(t, server, &mocks.Exchange{})

		notifier.SellHandle(message("/sell BTCUSDT 0"))

		require.Equal(t, "Invalid amount", server.lastSent(t))
	})

	t.Run("gives up silently when the position is unknown", func(t *testing.T) {
		server := newTelegramServer(t)
		exch := &mocks.Exchange{}
		exch.On("Position", "BTCUSDT").Return(0.0, 0.0, errors.New("no position"))

		notifier, _ := newTestTelegram(t, server, exch)
		before := len(server.sent())
		notifier.SellHandle(message("/sell BTCUSDT 50%"))

		require.Len(t, server.sent(), before)
	})

	t.Run("warns when the bot is stopped", func(t *testing.T) {
		server := newTelegramServer(t)
		exch := &mocks.Exchange{}
		exch.On("Position", "BTCUSDT").Return(2.0, 0.0, nil)

		notifier, controller := newTestTelegram(t, server, exch)
		controller.Start()
		controller.Stop()

		notifier.SellHandle(message("/sell BTCUSDT 100"))
		require.Contains(t, server.lastSent(t), "Bot is stopped, no order was created.")

		notifier.SellHandle(message("/sell BTCUSDT 50%"))
		require.Contains(t, server.lastSent(t), "Bot is stopped, no order was created.")
	})
}

func TestTelegramOnOrder(t *testing.T) {
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
			server := newTelegramServer(t)
			notifier, _ := newTestTelegram(t, server, &mocks.Exchange{})

			notifier.OnOrder(model.Order{ID: 1, Pair: "BTCUSDT", Status: tt.status,
				Side: model.SideTypeBuy, Type: model.OrderTypeLimit, Price: 100, Quantity: 1})

			require.Contains(t, server.lastSent(t), tt.wantTitle)
		})
	}
}

func TestTelegramOnError(t *testing.T) {
	t.Run("formats a plain error", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, _ := newTestTelegram(t, server, &mocks.Exchange{})

		notifier.OnError(errors.New("something went wrong"))

		reply := server.lastSent(t)
		require.Contains(t, reply, "🛑 ERROR")
		require.Contains(t, reply, "something went wrong")
	})

	t.Run("details an order error", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, _ := newTestTelegram(t, server, &mocks.Exchange{})

		notifier.OnError(&exchange.OrderError{
			Err:      errors.New("invalid quantity"),
			Pair:     "BTCUSDT",
			Quantity: 1.5,
		})

		reply := server.lastSent(t)
		require.Contains(t, reply, "Pair: BTCUSDT")
		require.Contains(t, reply, "Quantity: 1.5000")
		require.Contains(t, reply, "invalid quantity")
	})
}

func TestAuthorizedUser(t *testing.T) {
	settings := model.Settings{Telegram: model.TelegramSettings{Users: []int{telegramUserID}}}
	allowed := authorizedUser(settings)

	tests := []struct {
		name   string
		update *tb.Update
		want   bool
	}{
		{
			name:   "accepts a configured user",
			update: &tb.Update{Message: message("/status")},
			want:   true,
		},
		{
			name:   "rejects another user",
			update: &tb.Update{Message: &tb.Message{Sender: &tb.User{ID: 99}}},
			want:   false,
		},
		{
			name:   "rejects an update without a message",
			update: &tb.Update{},
			want:   false,
		},
		{
			name:   "rejects a message without a sender",
			update: &tb.Update{Message: &tb.Message{}},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, allowed(tt.update))
		})
	}
}

// Every handler must survive a Telegram outage: the failure is logged and the
// bot keeps running.
func TestTelegramHandlersSurviveSendFailures(t *testing.T) {
	handlers := map[string]func(*telegram, *tb.Message){
		"/balance": (*telegram).BalanceHandle,
		"/help":    (*telegram).HelpHandle,
		"/profit":  (*telegram).ProfitHandle,
		"/status":  (*telegram).StatusHandle,
		"/start":   (*telegram).StartHandle,
		"/stop":    (*telegram).StopHandle,
		"/buy":     (*telegram).BuyHandle,
		"/sell":    (*telegram).SellHandle,
	}

	for command, handle := range handlers {
		t.Run(command, func(t *testing.T) {
			server := newTelegramServer(t)
			exch := &mocks.Exchange{}
			exch.On("Account").Return(model.Account{}, nil).Maybe()
			exch.On("LastQuote", mock.Anything, mock.Anything).Return(100.0, nil).Maybe()

			notifier, controller := newTestTelegram(t, server, exch)
			if command == "/stop" {
				controller.Start()
			}
			server.failOn = "sendMessage"

			require.NotPanics(t, func() { handle(notifier, message(command)) })
		})
	}
}

// The bot is already running / already stopped replies also go through a send
// that can fail.
func TestTelegramStartStopSurviveSendFailures(t *testing.T) {
	t.Run("already running", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, controller := newTestTelegram(t, server, &mocks.Exchange{})

		controller.Start()
		defer controller.Stop()
		server.failOn = "sendMessage"

		require.NotPanics(t, func() { notifier.StartHandle(message("/start")) })
	})

	t.Run("already stopped", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, controller := newTestTelegram(t, server, &mocks.Exchange{})

		controller.Start()
		controller.Stop()
		server.failOn = "sendMessage"

		require.NotPanics(t, func() { notifier.StopHandle(message("/stop")) })
	})

	t.Run("stopped bot warning", func(t *testing.T) {
		server := newTelegramServer(t)
		notifier, controller := newTestTelegram(t, server, &mocks.Exchange{})

		controller.Start()
		controller.Stop()
		server.failOn = "sendMessage"

		require.NotPanics(t, func() { notifier.BuyHandle(message("/buy BTCUSDT 100")) })
	})
}

func TestTelegramProfitHandleSendFailure(t *testing.T) {
	server := newTelegramServer(t)
	exch := &mocks.Exchange{}
	exch.On("CreateOrderMarket", model.SideTypeBuy, "BTCUSDT", 1.0).
		Return(model.Order{ID: 1, Pair: "BTCUSDT", Side: model.SideTypeBuy, Quantity: 1, Price: 100,
			Status: model.OrderStatusTypeFilled, Type: model.OrderTypeMarket}, nil)

	notifier, controller := newTestTelegram(t, server, exch)
	controller.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 100})
	_, err := controller.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
	require.NoError(t, err)

	server.failOn = "sendMessage"

	require.NotPanics(t, func() { notifier.ProfitHandle(message("/profit")) })
}
