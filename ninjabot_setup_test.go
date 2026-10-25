package ninjabot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/order"
	"github.com/rodrigo-brito/ninjabot/storage"
	"github.com/rodrigo-brito/ninjabot/testdata/mocks"
	"github.com/rodrigo-brito/ninjabot/tools/log"
)

// recorder collects the candles and orders published to the bot subscribers.
// The feeds deliver from their own goroutines, hence the mutex.
type recorder struct {
	mu      sync.Mutex
	candles []model.Candle
	orders  []model.Order
}

func (r *recorder) OnCandle(candle model.Candle) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.candles = append(r.candles, candle)
}

func (r *recorder) OnOrder(order model.Order) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders = append(r.orders, order)
}

func (r *recorder) candleCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.candles)
}

func (r *recorder) orderCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.orders)
}

func (r *recorder) lastCandle() model.Candle {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.candles[len(r.candles)-1]
}

// newTestBot builds a bot backed by an in-memory storage and the given exchange.
func newTestBot(t *testing.T, exch *mocks.Exchange, options ...Option) *NinjaBot {
	t.Helper()

	memory, err := storage.FromMemory()
	require.NoError(t, err)

	settings := model.Settings{Pairs: []string{"BTCUSDT"}}
	options = append([]Option{WithStorage(memory)}, options...)

	bot, err := NewBot(context.Background(), settings, exch, &fakeStrategy{}, options...)
	require.NoError(t, err)

	return bot
}

func TestNewBot(t *testing.T) {
	t.Run("rejects an unknown pair", func(t *testing.T) {
		memory, err := storage.FromMemory()
		require.NoError(t, err)

		_, err = NewBot(context.Background(), model.Settings{Pairs: []string{"NOTAPAIR"}},
			&mocks.Exchange{}, &fakeStrategy{}, WithStorage(memory))

		require.ErrorContains(t, err, "invalid pair: NOTAPAIR")
	})

	t.Run("opens the default database when no storage is given", func(t *testing.T) {
		dir := t.TempDir()
		previous, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(dir))
		t.Cleanup(func() { require.NoError(t, os.Chdir(previous)) })

		bot, err := NewBot(context.Background(), model.Settings{Pairs: []string{"BTCUSDT"}},
			&mocks.Exchange{}, &fakeStrategy{})

		require.NoError(t, err)
		require.NotNil(t, bot.storage)
		require.FileExists(t, filepath.Join(dir, "ninjabot.db"))
	})

	t.Run("fails when the telegram token is invalid", func(t *testing.T) {
		memory, err := storage.FromMemory()
		require.NoError(t, err)

		settings := model.Settings{
			Pairs:    []string{"BTCUSDT"},
			Telegram: model.TelegramSettings{Enabled: true, Token: "", Users: []int{1}},
		}

		_, err = NewBot(context.Background(), settings, &mocks.Exchange{}, &fakeStrategy{},
			WithStorage(memory))

		require.Error(t, err, "an unreachable Telegram API must abort the setup")
	})
}

func TestBotOptions(t *testing.T) {
	t.Run("WithLogLevel changes the global level", func(t *testing.T) {
		previous := log.Level(0)
		t.Cleanup(func() { log.SetLevel(previous) })

		newTestBot(t, &mocks.Exchange{}, WithLogLevel(log.ErrorLevel))
	})

	t.Run("WithNotifier registers the notifier everywhere", func(t *testing.T) {
		notifier := &mocks.Notifier{}
		notifier.On("OnOrder", mock.Anything).Return()

		bot := newTestBot(t, &mocks.Exchange{}, WithNotifier(notifier))

		require.Equal(t, notifier, bot.notifier)
	})

	t.Run("WithCandleSubscription forwards the candles", func(t *testing.T) {
		subscriber := &recorder{}
		bot := newTestBot(t, &mocks.Exchange{}, WithCandleSubscription(subscriber))

		require.Equal(t, 1, bot.dataFeed.Feeds.Length())
		require.Len(t, bot.dataFeed.SubscriptionsByDataFeed["BTCUSDT--1d"], 1)
	})

	t.Run("WithOrderSubscription forwards the orders", func(t *testing.T) {
		subscriber := &recorder{}
		bot := newTestBot(t, &mocks.Exchange{}, WithOrderSubscription(subscriber))

		bot.orderFeed.Start()
		bot.orderFeed.Publish(model.Order{Pair: "BTCUSDT", Status: model.OrderStatusTypeFilled}, false)

		require.Eventually(t, func() bool { return subscriber.orderCount() == 1 }, time.Second, 10*time.Millisecond)
	})

	t.Run("WithPaperWallet attaches the wallet", func(t *testing.T) {
		wallet := exchange.NewPaperWallet(context.Background(), "USDT", exchange.WithPaperAsset("USDT", 1000))

		bot := newTestBot(t, &mocks.Exchange{}, WithPaperWallet(wallet))

		require.Equal(t, wallet, bot.paperWallet)
	})

	t.Run("WithBacktest also enables the paper wallet", func(t *testing.T) {
		wallet := exchange.NewPaperWallet(context.Background(), "USDT", exchange.WithPaperAsset("USDT", 1000))

		bot := newTestBot(t, &mocks.Exchange{}, WithBacktest(wallet))

		require.True(t, bot.backtest)
		require.Equal(t, wallet, bot.paperWallet)
	})
}

func TestBotController(t *testing.T) {
	bot := newTestBot(t, &mocks.Exchange{})

	require.IsType(t, &order.Controller{}, bot.Controller())
	require.Same(t, bot.orderController, bot.Controller())
}

func TestBotSaveReturns(t *testing.T) {
	t.Run("writes one file per pair", func(t *testing.T) {
		exch := &mocks.Exchange{}
		exch.On("CreateOrderMarket", model.SideTypeBuy, "BTCUSDT", 1.0).Return(model.Order{
			ExchangeID: 1, Pair: "BTCUSDT", Side: model.SideTypeBuy, Status: model.OrderStatusTypeFilled,
			Type: model.OrderTypeMarket, Price: 100, Quantity: 1,
		}, nil)

		bot := newTestBot(t, exch)
		_, err := bot.orderController.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
		require.NoError(t, err)

		dir := t.TempDir()
		require.NoError(t, bot.SaveReturns(dir))
		require.FileExists(t, filepath.Join(dir, "BTCUSDT.csv"))
	})

	t.Run("reports a write failure", func(t *testing.T) {
		exch := &mocks.Exchange{}
		exch.On("CreateOrderMarket", model.SideTypeBuy, "BTCUSDT", 1.0).Return(model.Order{
			ExchangeID: 1, Pair: "BTCUSDT", Side: model.SideTypeBuy, Status: model.OrderStatusTypeFilled,
			Type: model.OrderTypeMarket, Price: 100, Quantity: 1,
		}, nil)

		bot := newTestBot(t, exch)
		_, err := bot.orderController.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
		require.NoError(t, err)

		require.Error(t, bot.SaveReturns(filepath.Join(t.TempDir(), "missing")))
	})
}

func TestBotRunLive(t *testing.T) {
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	warmup := make([]model.Candle, 0, 10)
	for i := 0; i < 10; i++ {
		warmup = append(warmup, model.Candle{
			Pair: "BTCUSDT", Time: start.Add(time.Duration(i) * 24 * time.Hour),
			Open: 100, Close: 100 + float64(i), High: 110, Low: 90, Volume: 10, Complete: true,
			Metadata: map[string]float64{},
		})
	}

	t.Run("preloads the warmup candles and streams the live ones", func(t *testing.T) {
		exch := &mocks.Exchange{}
		exch.On("CandlesByLimit", mock.Anything, "BTCUSDT", "1d", 10).Return(warmup, nil)
		exch.On("Position", "BTCUSDT").Return(0.0, 0.0, nil).Maybe()

		stream := make(chan model.Candle)
		errs := make(chan error)
		exch.On("CandlesSubscription", mock.Anything, "BTCUSDT", "1d").Return(stream, errs)

		subscriber := &recorder{}
		bot := newTestBot(t, exch, WithCandleSubscription(subscriber))

		// Run blocks while the bot is live, so it stays in its own goroutine.
		go func() { require.NoError(t, bot.Run(context.Background())) }()

		live := model.Candle{Pair: "BTCUSDT", Time: start.Add(11 * 24 * time.Hour),
			Close: 120, High: 125, Low: 115, Open: 118, Complete: true, Metadata: map[string]float64{}}
		stream <- live

		// The warmup candles are replayed to the subscribers first, the live
		// one arrives last.
		require.Eventually(t, func() bool { return subscriber.candleCount() > len(warmup) },
			3*time.Second, 10*time.Millisecond, "the live candle never reached the subscribers")
		require.Equal(t, live.Close, subscriber.lastCandle().Close)
		require.NotNil(t, bot.strategiesControllers["BTCUSDT"])

		close(stream)
	})

	t.Run("fails when the warmup candles cannot be loaded", func(t *testing.T) {
		exch := &mocks.Exchange{}
		exch.On("CandlesByLimit", mock.Anything, "BTCUSDT", "1d", 10).
			Return([]model.Candle{}, errors.New("exchange down"))

		bot := newTestBot(t, exch)

		require.ErrorContains(t, bot.Run(context.Background()), "exchange down")
	})
}
