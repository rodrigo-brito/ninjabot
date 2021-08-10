package notification

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"

	"github.com/rodrigo-brito/ninjabot/pkg/exchange"
	"github.com/rodrigo-brito/ninjabot/pkg/model"
	"github.com/rodrigo-brito/ninjabot/pkg/order"
	"github.com/rodrigo-brito/ninjabot/pkg/service"
)

type telegram struct {
	settings        model.Settings
	orderController *order.Controller
	defaultMenu     *tb.ReplyMarkup
	client          *tb.Bot
}

type Option func(telegram *telegram)

func NewTelegram(controller *order.Controller, settings model.Settings, options ...Option) (service.Telegram, error) {
	menu := &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	poller := &tb.LongPoller{Timeout: 10 * time.Second}

	userMiddleware := tb.NewMiddlewarePoller(poller, func(u *tb.Update) bool {
		if u.Message == nil || u.Message.Sender == nil {
			log.Error("no message, ", u)
			return false
		}

		for _, user := range settings.Telegram.Users {
			if u.Message.Sender.ID == user {
				return true
			}
		}

		log.Error("invalid user, ", u.Message)
		return false
	})

	client, err := tb.NewBot(tb.Settings{
		ParseMode: tb.ModeMarkdown,
		Token:     settings.Telegram.Token,
		Poller:    userMiddleware,
	})
	if err != nil {
		return nil, err
	}

	var (
		statusBtn  = menu.Text("/status")
		profitBtn  = menu.Text("/profit")
		balanceBtn = menu.Text("/balance")
		stopBtn    = menu.Text("/stop")
		buyBtn     = menu.Text("/buy")
		sellBtn    = menu.Text("/sell")
	)

	err = client.SetCommands([]tb.Command{
		{Text: "/help", Description: "Display help instructions"},
		{Text: "/stop", Description: "Stop buy and sell coins"},
		{Text: "/start", Description: "Start buy and sell coins"},
		{Text: "/status", Description: "Check bot status"},
		{Text: "/balance", Description: "Wallet balance"},
		{Text: "/profit", Description: "Summary of last trade results"},
		{Text: "/buy", Description: "open a buy order"},
		{Text: "/sell", Description: "open a sell order"},
	})
	if err != nil {
		return nil, err
	}

	menu.Reply(
		menu.Row(statusBtn, balanceBtn, profitBtn),
		menu.Row(stopBtn, buyBtn, sellBtn),
	)

	bot := &telegram{
		orderController: controller,
		client:          client,
		settings:        settings,
		defaultMenu:     menu,
	}

	for _, option := range options {
		option(bot)
	}

	client.Handle("/help", bot.HelpHandle)
	client.Handle("/start", bot.StartHandle)
	client.Handle("/stop", bot.StopHandle)
	client.Handle("/status", bot.StatusHandle)
	client.Handle("/balance", bot.BalanceHandle)
	client.Handle("/profit", bot.ProfitHandle)
	client.Handle("/buy", bot.BuyHandle)
	client.Handle("/sell", bot.SellHandle)

	return bot, nil
}

func (t telegram) Start() {
	go t.client.Start()
}

func (t telegram) Notify(text string) {
	for _, user := range t.settings.Telegram.Users {
		_, err := t.client.Send(&tb.User{ID: user}, text)
		if err != nil {
			log.Error(err)
		}
	}
}

func (t telegram) BalanceHandle(m *tb.Message) {
	message := "*BALANCE*\n"
	quotesValue := make(map[string]float64)

	for _, pair := range t.settings.Pairs {
		assetSymbol, quoteSymbol := exchange.SplitAssetQuote(pair)
		assetValue, quoteValue, err := t.orderController.Position(pair)
		if err != nil {
			t.OrError(err)
		}

		quotesValue[quoteSymbol] = quoteValue
		message += fmt.Sprintf("%s: `%.4f`\n", assetSymbol, assetValue)
	}

	for quote, value := range quotesValue {
		message += fmt.Sprintf("%s: `%.4f`\n", quote, value)
	}

	_, err := t.client.Send(m.Sender, message)
	if err != nil {
		log.Error(err)
	}
}

func (t telegram) HelpHandle(m *tb.Message) {
	commands, err := t.client.GetCommands()
	if err != nil {
		t.OrError(err)
	}

	lines := make([]string, 0, len(commands))
	for _, command := range commands {
		lines = append(lines, fmt.Sprintf("/%s - %s", command.Text, command.Description))
	}

	_, err = t.client.Send(m.Sender, strings.Join(lines, "\n"))
	if err != nil {
		log.Error(err)
	}
}

func (t telegram) ProfitHandle(m *tb.Message) {
	_, err := t.client.Send(m.Sender, "not implemented yet")
	if err != nil {
		log.Error(err)
	}
}

func (t telegram) BuyHandle(m *tb.Message) {
	_, err := t.client.Send(m.Sender, "not implemented yet")
	if err != nil {
		log.Error(err)
	}
}

func (t telegram) StatusHandle(m *tb.Message) {
	_, err := t.client.Send(m.Sender, "not implemented yet")
	if err != nil {
		log.Error(err)
	}
}

func (t telegram) SellHandle(m *tb.Message) {
	_, err := t.client.Send(m.Sender, "not implemented yet")
	if err != nil {
		log.Error(err)
	}
}

func (t telegram) StartHandle(m *tb.Message) {
	_, err := t.client.Send(m.Sender, "Bot started", t.defaultMenu)
	if err != nil {
		log.Error(err)
	}
}

func (t telegram) StopHandle(m *tb.Message) {
	_, err := t.client.Send(m.Sender, "Bot stopped. To start again, use /start", t.defaultMenu)
	if err != nil {
		log.Error(err)
	}
}

func (t telegram) OnOrder(order model.Order) {
	title := ""
	switch order.Status {
	case model.OrderStatusTypeFilled:
		title = fmt.Sprintf("✅ ORDER FILLED - %s", order.Symbol)
	case model.OrderStatusTypeNew:
		title = fmt.Sprintf("🆕 NEW ORDER - %s", order.Symbol)
	case model.OrderStatusTypeCanceled, model.OrderStatusTypeRejected:
		title = fmt.Sprintf("❌ ORDER CANCELED / REJECTED - %s", order.Symbol)
	}
	message := fmt.Sprintf("%s\n-----\n%s", title, order)
	t.Notify(message)
}

func (t telegram) OrError(err error) {
	title := "🛑 ERROR"
	message := fmt.Sprintf("%s\n-----\n%s", title, err)
	t.Notify(message)
}
