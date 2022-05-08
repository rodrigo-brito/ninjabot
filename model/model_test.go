package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCandle_ToSlice(t *testing.T) {
	candle := Candle{
		Pair:     "BTCUSDT",
		Time:     time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		Open:     10000.1,
		Close:    10000.1,
		Low:      10000.1,
		High:     10000.1,
		Volume:   10000.1,
		Trades:   10000,
		Complete: true,
	}
	require.Equal(t, []string{"1609459200", "10000.1", "10000.1", "10000.1", "10000.1",
		"10000.1", "10000"}, candle.ToSlice(1))
}

func TestCandle_Less(t *testing.T) {
	now := time.Now()

	t.Run("equal time", func(t *testing.T) {
		candle := Candle{Time: now, UpdatedAt: now, Pair: "A"}
		item := Item(Candle{Time: now, UpdatedAt: now.Add(time.Minute), Pair: "B"})
		require.True(t, candle.Less(item))
	})

	t.Run("candle after item", func(t *testing.T) {
		candle := Candle{Time: now.Add(time.Minute), Pair: "A"}
		item := Item(Candle{Time: now, Pair: "B"})
		require.False(t, candle.Less(item))
	})
}

func TestAccount_Balance(t *testing.T) {
	account := Account{}
	require.Equal(t, Balance{}, account.Balance("A"))

	account.Balances = []Balance{{Tick: "B", Free: 1.1, Lock: 1.3}}
	require.Equal(t, Balance{}, account.Balance("A"))
	require.Equal(t, Balance{Tick: "B", Free: 1.1, Lock: 1.3}, account.Balance("B"))
}

func TestHeikinAshi_CalculateHeikinAshi(t *testing.T) {
	ha := NewHeikinAshi()

	if (HeikinAshi{}.PreviousHACandle != Candle{}) {
		t.Errorf("PreviousCandle should be empty")
	}

	// BTC-USDT weekly candles from Binance from 2017-08-14 to 2017-10-09
	// First market candles were used to easily test accuracy against TradingView without having to download all market data.
	candles := []Candle{
		{Open: 4261.48, Close: 4086.29, High: 4485.39, Low: 3850.00},
		{Open: 4069.13, Close: 4310.01, High: 4453.91, Low: 3400.00},
		{Open: 4310.01, Close: 4509.08, High: 4939.19, Low: 4124.54},
		{Open: 4505.00, Close: 4130.37, High: 4788.59, Low: 3603.00},
		{Open: 4153.62, Close: 3699.99, High: 4394.59, Low: 2817.00},
		{Open: 3690.00, Close: 3660.02, High: 4123.20, Low: 3505.55},
		{Open: 3660.02, Close: 4378.48, High: 4406.52, Low: 3653.69},
		{Open: 4400.00, Close: 4640.00, High: 4658.00, Low: 4110.00},
		{Open: 4640.00, Close: 5709.99, High: 5922.30, Low: 4550.00},
	}

	var results []Candle

	for _, candle := range candles {
		haCandle := ha.CalculateHeikinAshi(candle)
		results = append(results, haCandle)
	}

	// Expected values taken from TradingView.
	// Source: Binance BTC-USDT
	expected := []Candle{
		{Open: 4173.885, Close: 4170.79, High: 4485.39, Low: 3850},
		{Open: 4172.3375, Close: 4058.2625000000003, High: 4453.91, Low: 3400},
		{Open: 4115.3, Close: 4470.705, High: 4939.19, Low: 4124.54},
		{Open: 4293.0025000000005, Close: 4256.74, High: 4788.59, Low: 3603},
		{Open: 4274.87125, Close: 3766.2999999999997, High: 4394.59, Low: 2817},
		{Open: 4020.5856249999997, Close: 3744.6925, High: 4123.2, Low: 3505.55},
		{Open: 3882.6390625, Close: 4024.6775000000002, High: 4406.52, Low: 3653.69},
		{Open: 3953.65828125, Close: 4452, High: 4658, Low: 4110},
		{Open: 4202.829140625, Close: 5205.5725, High: 5922.3, Low: 4550},
	}

	if len(expected) != len(results) {
		t.Errorf("Expected %d HA candles. Got %d.", len(expected), len(results))
	}

	for index, expectedHaCandle := range expected {
		require.Equal(t, expectedHaCandle.Open, results[index].Open)
		require.Equal(t, expectedHaCandle.Close, results[index].Close)
		require.Equal(t, expectedHaCandle.High, results[index].High)
		require.Equal(t, expectedHaCandle.Low, results[index].Low)
	}
}
