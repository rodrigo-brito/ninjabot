package notification

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/order"
	"github.com/rodrigo-brito/ninjabot/service"
)

var (
	buyRegexp  = regexp.MustCompile(`/buy\s+(?P<pair>\w+)\s+(?P<amount>[0-9]+(?:\.\d+)?)(?P<percent>%)?`)
	sellRegexp = regexp.MustCompile(`/sell\s+(?P<pair>\w+)\s+(?P<amount>[0-9]+(?:\.\d+)?)(?P<percent>%)?`)
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
		startBtn   = menu.Text("/start")
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
		menu.Row(startBtn, stopBtn, buyBtn, sellBtn),
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
	for _, id := range t.settings.Telegram.Users {
		_, err := t.client.Send(&tb.User{ID: id}, "Bot initialized.", t.defaultMenu)
		if err != nil {
			log.Error(err)
		}
	}
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
	if len(t.orderController.Results) == 0 {
		_, err := t.client.Send(m.Sender, "No trades registered.")
		if err != nil {
			log.Error(err)
		}
		return
	}

	for pair, summary := range t.orderController.Results {
		_, err := t.client.Send(m.Sender, fmt.Sprintf("*PAIR*: `%s`\n`%s`", pair, summary.String()))
		if err != nil {
			log.Error(err)
		}
	}
}

func (t telegram) BuyHandle(m *tb.Message) {
	match := buyRegexp.FindStringSubmatch(m.Text)
	if len(match) == 0 {
		_, err := t.client.Send(m.Sender, "Invalid command.\nExamples of usage:\n`/buy BTCUSDT 100`\n\n`/buy BTCUSDT 50%`")
		if err != nil {
			log.Error(err)
		}
		return
	}

	command := make(map[string]string)
	for i, name := range buyRegexp.SubexpNames() {
		if i != 0 && name != "" {
			command[name] = match[i]
		}
	}

	pair := strings.ToUpper(command["pair"])
	amount, err := strconv.ParseFloat(command["amount"], 64)
	if err != nil {
		t.OrError(err)
		return
	} else if amount <= 0 {
		_, err := t.client.Send(m.Sender, "Invalid amount")
		if err != nil {
			log.Error(err)
		}
		return
	}

	if command["percent"] != "" {
		_, quote, err := t.orderController.Position(pair)
		if err != nil {
			t.OrError(err)
			return
		}

		amount = amount * quote / 100.0
	}

	order, err := t.orderController.CreateOrderMarketQuote(model.SideTypeBuy, pair, amount)
	if err != nil {
		t.OrError(err)
		return
	}
	t.OnOrder(order)
}

func (t telegram) SellHandle(m *tb.Message) {
	match := sellRegexp.FindStringSubmatch(m.Text)
	if len(match) == 0 {
		_, err := t.client.Send(m.Sender, "Invalid command.\nExample of usage:\n`/sell BTCUSDT 100`\n\n`/sell BTCUSDT 50%")
		if err != nil {
			log.Error(err)
		}
		return
	}

	command := make(map[string]string)
	for i, name := range sellRegexp.SubexpNames() {
		if i != 0 && name != "" {
			command[name] = match[i]
		}
	}

	pair := strings.ToUpper(command["pair"])
	amount, err := strconv.ParseFloat(command["amount"], 64)
	if err != nil {
		t.OrError(err)
		return
	} else if amount <= 0 {
		_, err := t.client.Send(m.Sender, "Invalid amount")
		if err != nil {
			log.Error(err)
		}
		return
	}

	if command["percent"] != "" {
		asset, _, err := t.orderController.Position(pair)
		if err != nil {
			t.OrError(err)
			return
		}

		amount = amount * asset / 100.0
		order, err := t.orderController.CreateOrderMarket(model.SideTypeSell, pair, amount)
		if err != nil {
			t.OrError(err)
			return
		}
		t.OnOrder(order)
		return
	}

	order, err := t.orderController.CreateOrderMarketQuote(model.SideTypeSell, pair, amount)
	if err != nil {
		t.OrError(err)
		return
	}
	t.OnOrder(order)
}

func (t telegram) StatusHandle(m *tb.Message) {
	status := t.orderController.Status()
	_, err := t.client.Send(m.Sender, fmt.Sprintf("Status: `%s`", status))
	if err != nil {
		log.Error(err)
	}
}

func (t telegram) StartHandle(m *tb.Message) {
	if t.orderController.Status() == order.StatusRunning {
		_, err := t.client.Send(m.Sender, "Bot is already running.", t.defaultMenu)
		if err != nil {
			log.Error(err)
		}
		return
	}

	t.orderController.Start()
	_, err := t.client.Send(m.Sender, "Bot started.", t.defaultMenu)
	if err != nil {
		log.Error(err)
	}
}

func (t telegram) StopHandle(m *tb.Message) {
	if t.orderController.Status() == order.StatusStopped {
		_, err := t.client.Send(m.Sender, "Bot is already stopped.", t.defaultMenu)
		if err != nil {
			log.Error(err)
		}
		return
	}

	t.orderController.Stop()
	_, err := t.client.Send(m.Sender, "Bot stopped.", t.defaultMenu)
	if err != nil {
		log.Error(err)
	}
}

func (t telegram) OnOrder(order model.Order) {
	title := ""
	switch order.Status {
	case model.OrderStatusTypeFilled:
		title = fmt.Sprintf("✅ ORDER FILLED - %s", order.Pair)
	case model.OrderStatusTypeNew:
		title = fmt.Sprintf("🆕 NEW ORDER - %s", order.Pair)
	case model.OrderStatusTypeCanceled, model.OrderStatusTypeRejected:
		title = fmt.Sprintf("❌ ORDER CANCELED / REJECTED - %s", order.Pair)
	}
	message := fmt.Sprintf("%s\n-----\n%s", title, order)
	t.Notify(message)
}

func (t telegram) OrError(err error) {
	log.Error(err)
	title := "🛑 ERROR"
	message := fmt.Sprintf("%s\n-----\n%s", title, err)
	t.Notify(message)
}
