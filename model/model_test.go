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
	require.Equal(t, []string{"1609459200", "10000.100000", "10000.100000", "10000.100000", "10000.100000",
		"10000.1", "10000"}, candle.ToSlice())
}

func TestCandle_Less(t *testing.T) {
	now := time.Now()

	t.Run("equal time", func(t *testing.T) {
		candle := Candle{Time: now, Pair: "A"}
		item := Item(Candle{Time: now, Pair: "B"})
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
