package ninjabot

import (
	"context"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/storage"
)

// runSampleBacktest replays the two 1h samples of testdata with fakeStrategy
// and returns the bot after the run.
func runSampleBacktest(tb testing.TB, timeframe string) *NinjaBot {
	tb.Helper()
	ctx := context.Background()

	store, err := storage.FromMemory()
	require.NoError(tb, err)

	csvFeed, err := exchange.NewCSVFeed(timeframe,
		exchange.PairFeed{Pair: "BTCUSDT", File: "testdata/btc-1h.csv", Timeframe: "1h"},
		exchange.PairFeed{Pair: "ETHUSDT", File: "testdata/eth-1h.csv", Timeframe: "1h"},
	)
	require.NoError(tb, err)

	wallet := exchange.NewPaperWallet(ctx, "USDT",
		exchange.WithPaperAsset("USDT", 10000),
		exchange.WithPaperFee(0.001, 0.001),
		exchange.WithDataFeed(csvFeed),
	)

	bot, err := NewBot(ctx, Settings{Pairs: []string{"BTCUSDT", "ETHUSDT"}}, wallet, &timeframeStrategy{timeframe: timeframe},
		WithStorage(store),
		WithBacktest(wallet),
		WithLogLevel(log.ErrorLevel),
	)
	require.NoError(tb, err)
	require.NoError(tb, bot.Run(ctx))
	return bot
}

// timeframeStrategy is fakeStrategy on a configurable timeframe.
type timeframeStrategy struct {
	fakeStrategy
	timeframe string
}

func (s timeframeStrategy) Timeframe() string { return s.timeframe }

// BenchmarkBot_Run_2Pairs_1h is the end-to-end baseline of the backtest:
// 2 pairs x ~4.3k hourly candles through NewBot + Run.
func BenchmarkBot_Run_2Pairs_1h(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		runSampleBacktest(b, "1h")
	}
}

// TestBacktest_RunTwice documents the determinism of the current backtest:
// strategies that only send market orders produce identical trades and
// equity on every run.
func TestBacktest_RunTwice(t *testing.T) {
	first := runSampleBacktest(t, "1h")
	second := runSampleBacktest(t, "1h")

	require.NotEmpty(t, first.Controller().Trades())
	require.Equal(t, first.Controller().Trades(), second.Controller().Trades())
	require.Equal(t, first.paperWallet.EquityValues(), second.paperWallet.EquityValues())

	result := first.Result()
	require.NotNil(t, result)
	require.Equal(t, first.Controller().Trades(), result.Trades)
	require.Equal(t, "1h", result.Config.Timeframe)
}
