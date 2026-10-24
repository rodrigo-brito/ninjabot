package backtest

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/markcheno/go-talib"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

// emaCross mirrors the strategy behind the golden numbers of ninjabot_test.go.
type emaCross struct{ timeframe string }

func (e emaCross) Timeframe() string { return e.timeframe }
func (e emaCross) WarmupPeriod() int { return 10 }

func (e emaCross) Indicators(df *model.Dataframe) []strategy.ChartIndicator {
	df.Metadata["ema9"] = talib.Ema(df.Close, 9)
	return nil
}

func (e *emaCross) OnCandle(df *model.Dataframe, broker service.Broker) {
	closePrice := df.Close.Last(0)
	assetPosition, quotePosition, err := broker.Position(df.Pair)
	if err != nil {
		panic(err)
	}
	if quotePosition > 0 && df.Close.Crossover(df.Metadata["ema9"]) {
		if _, err := broker.CreateOrderMarket(model.SideTypeBuy, df.Pair, quotePosition/closePrice*0.5); err != nil {
			panic(err)
		}
	}
	if assetPosition > 0 && df.Close.Crossunder(df.Metadata["ema9"]) {
		if _, err := broker.CreateOrderMarket(model.SideTypeSell, df.Pair, assetPosition); err != nil {
			panic(err)
		}
	}
}

// ocoStrategy buys on every EMA crossover and protects the position with an
// OCO: take profit at +3%, stop at -2%. It exercises the asynchronous fills.
type ocoStrategy struct{}

func (ocoStrategy) Timeframe() string { return "4h" }
func (ocoStrategy) WarmupPeriod() int { return 10 }

func (ocoStrategy) Indicators(df *model.Dataframe) []strategy.ChartIndicator {
	df.Metadata["ema9"] = talib.Ema(df.Close, 9)
	return nil
}

func (ocoStrategy) OnCandle(df *model.Dataframe, broker service.Broker) {
	closePrice := df.Close.Last(0)
	assetPosition, quotePosition, err := broker.Position(df.Pair)
	if err != nil {
		panic(err)
	}
	if assetPosition > 0 || quotePosition < 10 || !df.Close.Crossover(df.Metadata["ema9"]) {
		return
	}

	amount := quotePosition * 0.4 / closePrice
	if _, err := broker.CreateOrderMarket(model.SideTypeBuy, df.Pair, amount); err != nil {
		panic(err)
	}
	if _, err := broker.CreateOrderOCO(model.SideTypeSell, df.Pair, amount,
		closePrice*1.03, closePrice*0.98, closePrice*0.98); err != nil {
		panic(err)
	}
}

func runEngine(t testing.TB, ds *Dataset, s strategy.Strategy, fee float64) *Result {
	t.Helper()
	return runEngineConfig(t, ds, Config{Strategy: s, MakerFee: fee, TakerFee: fee, NoStatistics: true})
}

func runEngineConfig(t testing.TB, ds *Dataset, cfg Config) *Result {
	t.Helper()
	cfg.Quote = "USDT"
	cfg.Balances = map[string]float64{"USDT": 10000}
	engine, err := New(ds, cfg)
	require.NoError(t, err)
	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	return result
}

func summarize(result *Result) map[string][3]float64 { // pair -> profit, wins, losses
	out := make(map[string][3]float64)
	for _, trade := range result.Trades {
		s := out[trade.Pair]
		s[0] += trade.ProfitValue
		if trade.ProfitPercent >= 0 {
			s[1]++
		} else {
			s[2]++
		}
		out[trade.Pair] = s
	}
	return out
}

func TestEngine_MatchesLegacyGolden(t *testing.T) {
	result := runEngine(t, sampleDataset(t), &emaCross{timeframe: "1d"}, 0)

	summary := summarize(result)
	require.InDelta(t, 5340.224, summary["BTCUSDT"][0], 0.001)
	require.Equal(t, 5.0, summary["BTCUSDT"][1])
	require.Equal(t, 3.0, summary["BTCUSDT"][2])
	require.InDelta(t, 7590.7381, summary["ETHUSDT"][0], 0.001)
	require.Equal(t, 7.0, summary["ETHUSDT"][1])
	require.Equal(t, 9.0, summary["ETHUSDT"][2])
}

func TestEngine_OneEquityPointPerTimestamp(t *testing.T) {
	ds := sampleDataset(t)
	result := runEngine(t, ds, &emaCross{timeframe: "1h"}, 0.001)

	times := make(map[int64]bool)
	for _, pair := range ds.Pairs() {
		candles, err := ds.Candles(pair, "1h")
		require.NoError(t, err)
		for _, candle := range candles {
			times[candle.Time.Unix()] = true
		}
	}
	require.Len(t, result.Equity, len(times))
	for i := 1; i < len(result.Equity); i++ {
		require.Greater(t, result.Equity[i].Time, result.Equity[i-1].Time)
	}
}

// TestEngine_OCOGolden records the result of a strategy that relies on
// asynchronous fills. The legacy loop applied those fills from a 1s ticker,
// out of order with the market orders; the engine applies them on the candle
// they happen, so these numbers are the reference from ROADMAP 1.3 on.
func TestEngine_OCOGolden(t *testing.T) {
	result := runEngine(t, sampleDataset(t), ocoStrategy{}, 0.001)

	summary := summarize(result)
	require.Len(t, result.Trades, 172)
	require.InDelta(t, -291.7582, summary["BTCUSDT"][0], 0.001)
	require.Equal(t, [2]float64{37, 50}, [2]float64{summary["BTCUSDT"][1], summary["BTCUSDT"][2]})
	require.InDelta(t, -1267.1108, summary["ETHUSDT"][0], 0.001)
	require.Equal(t, [2]float64{32, 53}, [2]float64{summary["ETHUSDT"][1], summary["ETHUSDT"][2]})
	require.InDelta(t, 8441.1310, result.Equity[len(result.Equity)-1].Value, 0.001)

	// every OCO pair ends with one leg filled and the other canceled, except
	// possibly the last one, still open at the end
	filled, canceled := 0, 0
	for _, o := range result.Orders {
		if o.GroupID == nil {
			continue
		}
		switch o.Status {
		case model.OrderStatusTypeFilled:
			filled++
		case model.OrderStatusTypeCanceled:
			canceled++
		}
	}
	require.InDelta(t, filled, canceled, 2)
	require.Equal(t, len(result.Trades), filled)

	// trades are chronological
	for i := 1; i < len(result.Trades); i++ {
		require.False(t, result.Trades[i].ClosedAt.Before(result.Trades[i-1].ClosedAt))
	}
}

func hashResult(t testing.TB, result *Result) string {
	t.Helper()
	data, err := json.Marshal(result)
	require.NoError(t, err)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func TestEngine_Deterministic(t *testing.T) {
	ds := sampleDataset(t)
	goroutines := runtime.NumGoroutine()

	reference := hashResult(t, runEngine(t, ds, ocoStrategy{}, 0.001))
	require.Equal(t, reference, hashResult(t, runEngine(t, ds, ocoStrategy{}, 0.001)))
	require.Equal(t, goroutines, runtime.NumGoroutine(), "a run leaves no goroutine behind")

	var wg sync.WaitGroup
	hashes := make([]string, 4)
	for i := range hashes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hashes[i] = hashResult(t, runEngine(t, ds, ocoStrategy{}, 0.001))
		}()
	}
	wg.Wait()
	for _, hash := range hashes {
		require.Equal(t, reference, hash)
	}
}

func TestEngine_Validation(t *testing.T) {
	ds := sampleDataset(t)
	_, err := New(ds, Config{Quote: "USDT"})
	require.Error(t, err)
	_, err = New(ds, Config{Strategy: ocoStrategy{}})
	require.Error(t, err)
	_, err = New(ds, Config{Strategy: ocoStrategy{}, Quote: "BTC"})
	require.Error(t, err)
	_, err = New(ds, Config{Strategy: ocoStrategy{}, Quote: "USDT", Pairs: []string{"XRPUSDT"}})
	require.Error(t, err)
}

func TestEngine_Cancel(t *testing.T) {
	engine, err := New(sampleDataset(t), Config{Strategy: ocoStrategy{}, Quote: "USDT",
		Balances: map[string]float64{"USDT": 100}})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = engine.Run(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestEngine_Subscribers(t *testing.T) {
	var candles, orders, progress int
	engine, err := New(sampleDataset(t), Config{
		Strategy:          ocoStrategy{},
		Quote:             "USDT",
		Balances:          map[string]float64{"USDT": 10000},
		CandleSubscribers: []func(model.Candle){func(model.Candle) { candles++ }},
		OrderSubscribers:  []func(model.Order){func(model.Order) { orders++ }},
		Progress:          func(done, total int) { progress = done },
	})
	require.NoError(t, err)
	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, result.Period.Candles, candles)
	require.Equal(t, result.Period.Candles, progress)
	require.Positive(t, orders)
}

func BenchmarkEngine_2Pairs_1h(b *testing.B) {
	ds := sampleDataset(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runEngine(b, ds, &emaCross{timeframe: "1h"}, 0.001)
	}
}

func BenchmarkEngine_WithStatistics(b *testing.B) {
	ds := sampleDataset(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		runEngineConfig(b, ds, Config{Strategy: &emaCross{timeframe: "1h"}, MakerFee: 0.001, TakerFee: 0.001})
	}
}
