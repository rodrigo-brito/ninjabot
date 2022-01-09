package plot

import (
	"github.com/StudioSol/set"
	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestChart_CandleAndOrder(t *testing.T) {
	c, err := NewChart()
	require.NoErrorf(t, err, "error when initial chart")

	candle := model.Candle{
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
	c.OnCandle(candle)

	order := model.Order{
		ID:         1,
		ExchangeID: 1,
		Pair:       "BTCUSDT",
		Side:       "SELL",
		Type:       "MARKET",
		Status:     "FILLED",
		Price:      10000.1,
		Quantity:   1,
		CreatedAt:  time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		Stop:       nil,
		GroupID:    nil,
		RefPrice:   0,
		Profit:     0,
		Candle:     model.Candle{},
	}
	c.OnOrder(order)

	expectOrder := Order{
		ID:        1,
		Side:      "SELL",
		Type:      "MARKET",
		Status:    "FILLED",
		Price:     10000.1,
		Quantity:  1,
		CreatedAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	require.Equal(t, &expectOrder, c.orderByID[order.ID])

	expectCandleByPair := []Candle{
		{
			Time:   time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			Open:   10000.1,
			Close:  10000.1,
			High:   10000.1,
			Low:    10000.1,
			Volume: 10000.1,
			Orders: []Order{
				expectOrder,
			},
		},
	}
	actual := c.candlesByPair(candle.Pair)
	require.Equal(t, expectCandleByPair, actual)
}

func TestChart_WithPort(t *testing.T) {
	port := 8081
	c, err := NewChart(WithPort(port))
	require.NoErrorf(t, err, "error when initial chart")

	require.Equal(t, port, c.port)

}

func TestChart_WithPaperWallet(t *testing.T) {
	wallet := &exchange.PaperWallet{}
	c, err := NewChart(WithPaperWallet(wallet))
	require.NoErrorf(t, err, "error when initial chart")

	require.Equal(t, wallet, c.paperWallet)

}

func TestChart_WithDebug(t *testing.T) {
	c, err := NewChart(WithDebug())
	require.NoErrorf(t, err, "error when initial chart")

	require.Equal(t, true, c.debug)

}

func TestChart_WithIndicator(t *testing.T) {
	var indicator []Indicator
	c, err := NewChart(WithIndicators(indicator...))
	require.NoErrorf(t, err, "error when initial chart")

	require.Equal(t, indicator, c.indicators)
}

func TestChart_OrderStringByPair(t *testing.T) {
	c, err := NewChart()
	require.NoErrorf(t, err, "error when initial chart")

	pair1 := "ETHUSDT"
	pair2 := "BNBUSDT"
	order1 := &Order{
		ID:        1,
		Side:      "SELL",
		Type:      "MARKET",
		Status:    "FILLED",
		Price:     3059.37,
		Quantity:  4783.34,
		CreatedAt: time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC),
	}
	order2 := &Order{
		ID:        2,
		Side:      "BUY",
		Type:      "MARKET",
		Status:    "FILLED",
		Price:     3607.42,
		Quantity:  0.75152,
		CreatedAt: time.Date(2021, 10, 13, 20, 0, 0, 0, time.UTC),
	}

	order3 := &Order{
		ID:        13,
		Side:      "BUY",
		Type:      "MARKET",
		Status:    "FILLED",
		Price:     470,
		Quantity:  12.08324,
		CreatedAt: time.Date(2021, 10, 13, 20, 0, 0, 0, time.UTC),
	}
	c.ordersByPair[pair1] = set.NewLinkedHashSetINT64()
	c.ordersByPair[pair1].Add(order1.ID)
	c.orderByID[order1.ID] = order1

	c.ordersByPair[pair1].Add(order2.ID)
	c.orderByID[order2.ID] = order2

	c.ordersByPair[pair2] = set.NewLinkedHashSetINT64()
	c.ordersByPair[pair2].Add(order3.ID)
	c.orderByID[order3.ID] = order3

	expectPair1 := [][]string{
		{
			"FILLED", "SELL", "1", "MARKET", "4783.340000", "3059.370000",
			"14634006.90", "2021-09-26 20:00:00 +0000 UTC",
		},
		{
			"FILLED", "BUY", "2", "MARKET", "0.751520", "3607.420000",
			"2711.05", "2021-10-13 20:00:00 +0000 UTC",
		},
	}

	ordersPair1 := c.orderStringByPair(pair1)
	require.Equal(t, expectPair1, ordersPair1)

	expectPair2 := [][]string{
		{
			"FILLED", "BUY", "13", "MARKET", "12.083240", "470.000000",
			"5679.12", "2021-10-13 20:00:00 +0000 UTC",
		},
	}
	ordersPair2 := c.orderStringByPair(pair2)
	require.Equal(t, expectPair2, ordersPair2)
}
