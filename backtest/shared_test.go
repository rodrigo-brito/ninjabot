package backtest

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

// allIn buys with its whole quote balance on an EMA crossover and sells on
// the crossunder: it relies on the scoped broker to see only its budget.
type allIn struct{ emaCross }

func (s *allIn) OnCandle(df *model.Dataframe, broker service.Broker) {
	asset, quote, err := broker.Position(df.Pair)
	if err != nil {
		panic(err)
	}
	price := df.Close.Last(0)
	if quote > 10 && df.Close.Crossover(df.Metadata["ema9"]) {
		_, _ = broker.CreateOrderMarket(model.SideTypeBuy, df.Pair, quote/price*0.99)
	}
	if asset > 0 && df.Close.Crossunder(df.Metadata["ema9"]) {
		_, _ = broker.CreateOrderMarket(model.SideTypeSell, df.Pair, asset)
	}
}

func TestRunShared_Attribution(t *testing.T) {
	ds := sampleDataset(t)
	result, err := RunShared(context.Background(), ds, SharedConfig{
		Quote: "USDT", Capital: 10000, MakerFee: 0.001, TakerFee: 0.001,
		Strategies: []SharedStrategy{
			{Tag: "fast", Strategy: &allIn{emaCross{timeframe: "4h"}}, Budget: 5000},
			{Tag: "slow", Strategy: &allIn{emaCross{timeframe: "1d"}}, Pairs: []string{"BTCUSDT"}, Budget: 3000},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Tags, 2)

	account := result.Account
	fast, slow := result.Tags[0].Result, result.Tags[1].Result
	require.Equal(t, "4h", account.Config.Timeframe, "the finest timeframe prices the account")
	require.NotEmpty(t, fast.Trades)
	require.NotEmpty(t, slow.Trades)
	require.Equal(t, []string{"BTCUSDT"}, slow.Config.Pairs)
	require.Equal(t, len(account.Trades), len(fast.Trades)+len(slow.Trades))
	for _, trade := range slow.Trades {
		require.Equal(t, "slow", trade.Tag)
		require.Equal(t, "BTCUSDT", trade.Pair)
	}

	// the tags and the unallocated cash add up to the account, at every bar
	require.Equal(t, len(account.Equity), len(fast.Equity))
	require.Equal(t, len(account.Equity), len(slow.Equity))
	for i, point := range account.Equity {
		sum := 2000 + fast.Equity[i].Value + slow.Equity[i].Value
		require.InDelta(t, point.Value, sum, 1e-6, "bar %d", i)
	}
	require.InDelta(t, account.Wallet.Final, 2000+fast.Wallet.Final+slow.Wallet.Final, 1e-6)
	require.InDelta(t, account.Wallet.Fees, fast.Wallet.Fees+slow.Wallet.Fees, 1e-6)
	require.True(t, math.Abs(fast.Wallet.NetProfit) > 0)
	require.NotNil(t, account.Portfolio)
}

func TestRunShared_Deterministic(t *testing.T) {
	ds := sampleDataset(t)
	run := func() string {
		result, err := RunShared(context.Background(), ds, SharedConfig{
			Quote: "USDT", Capital: 10000, NoStatistics: true,
			Strategies: []SharedStrategy{
				{Tag: "a", Strategy: &allIn{emaCross{timeframe: "4h"}}, Budget: 5000},
				{Tag: "b", Strategy: &allIn{emaCross{timeframe: "4h"}}, Budget: 5000},
			},
		})
		require.NoError(t, err)
		// two identical strategies with identical budgets split the account
		require.Equal(t, len(result.Tags[0].Result.Trades), len(result.Tags[1].Result.Trades))
		return hashResult(t, result.Account)
	}
	require.Equal(t, run(), run())
}

func TestRunShared_Validation(t *testing.T) {
	ds := sampleDataset(t)
	s := &allIn{emaCross{timeframe: "4h"}}
	for _, cfg := range []SharedConfig{
		{Capital: 1, Strategies: []SharedStrategy{{Tag: "a", Strategy: s}}},
		{Quote: "USDT", Strategies: []SharedStrategy{{Tag: "a", Strategy: s}}},
		{Quote: "USDT", Capital: 1},
		{Quote: "USDT", Capital: 1, Strategies: []SharedStrategy{{Tag: "a", Strategy: s}, {Tag: "a", Strategy: s}}},
		{Quote: "USDT", Capital: 1, Strategies: []SharedStrategy{{Tag: "a", Strategy: s, Timeframe: "x"}}},
		{Quote: "BTC", Capital: 1, Strategies: []SharedStrategy{{Tag: "a", Strategy: s}}},
		{Quote: "USDT", Capital: 1, Strategies: []SharedStrategy{{Tag: "a", Strategy: s, Pairs: []string{"XRPUSDT"}}}},
	} {
		_, err := RunShared(context.Background(), ds, cfg)
		require.Error(t, err)
	}
}

var _ strategy.Strategy = &allIn{}
