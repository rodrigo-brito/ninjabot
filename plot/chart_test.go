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
	pair := "ETHUSDT"
	c, err := NewChart()
	require.NoErrorf(t, err, "error when initial chart")

	//feed candle and order 1
	candle := model.Candle{
		Pair:     "ETHUSDT",
		Time:     time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC),
		Open:     3057.67,
		Close:    3059.37,
		Low:      3011.00,
		High:     3115.51,
		Volume:   87666.8,
		Trades:   205069,
		Complete: true,
	}
	c.OnCandle(candle)

	order := model.Order{
		ID:         1,
		ExchangeID: 1,
		Pair:       "ETHUSDT",
		Side:       "BUY",
		Type:       "MARKET",
		Status:     "FILLED",
		Price:      3059.37,
		Quantity:   1.634323,
		CreatedAt:  time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC),
		Stop:       nil,
		GroupID:    nil,
		RefPrice:   0,
		Profit:     0,
		Candle:     model.Candle{},
	}
	c.OnOrder(order)
	expectOrder := Order{
		ID:        1,
		Side:      "BUY",
		Type:      "MARKET",
		Status:    "FILLED",
		Price:     3059.37,
		Quantity:  1.634323,
		CreatedAt: time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC),
		RefPrice:  0,
	}
	require.Equal(t, &expectOrder, c.orderByID[order.ID])

	//feed candle and oco order
	candle2 := model.Candle{
		Pair:     "ETHUSDT",
		Time:     time.Date(2021, 9, 28, 8, 0, 0, 0, time.UTC),
		Open:     2894.18,
		Close:    2926.80,
		Low:      2876.12,
		High:     2940.74,
		Volume:   88470.1,
		Trades:   161354,
		Complete: true,
	}
	c.OnCandle(candle2)

	groupID := int64(3)
	limitMakerOrder := model.Order{
		ID:         3,
		ExchangeID: 3,
		CreatedAt:  time.Date(2021, 9, 28, 8, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2021, 9, 28, 8, 0, 0, 0, time.UTC),
		Pair:       pair,
		Side:       "SELL",
		Type:       model.OrderTypeLimitMaker,
		Status:     model.OrderStatusTypeNew,
		Price:      2926.00,
		Quantity:   1.634323,
		GroupID:    &groupID,
		RefPrice:   3059.37,
	}
	c.OnOrder(limitMakerOrder)
	expectLimitMakerOrder := Order{
		ID:        3,
		Side:      "SELL",
		Type:      "LIMIT_MAKER",
		Status:    "NEW",
		Price:     2926,
		Quantity:  1.634323,
		CreatedAt: time.Date(2021, 9, 28, 8, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2021, 9, 28, 8, 0, 0, 0, time.UTC),
		OCOGroup:  &groupID,
		RefPrice:  3059.37,
	}
	require.Equal(t, &expectLimitMakerOrder, c.orderByID[limitMakerOrder.ID])

	stop := 2900.00
	stopOrder := model.Order{
		ID:         4,
		ExchangeID: 3,
		CreatedAt:  time.Date(2021, 9, 28, 8, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2021, 9, 28, 8, 0, 0, 0, time.UTC),
		Pair:       pair,
		Side:       "SELL",
		Type:       model.OrderTypeStopLoss,
		Status:     model.OrderStatusTypeNew,
		Price:      3000.00,
		Stop:       &stop,
		Quantity:   1.634323,
		GroupID:    &groupID,
		RefPrice:   3059.37,
	}
	c.OnOrder(stopOrder)
	expectStopOrder := Order{
		ID:        4,
		Side:      "SELL",
		Type:      "STOP_LOSS",
		Status:    "NEW",
		Price:     3000,
		Quantity:  1.634323,
		CreatedAt: time.Date(2021, 9, 28, 8, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2021, 9, 28, 8, 0, 0, 0, time.UTC),
		Stop:      &stop,
		OCOGroup:  &groupID,
		RefPrice:  3059.37,
	}
	require.Equal(t, &expectStopOrder, c.orderByID[stopOrder.ID])

	//test candles by pair
	expectCandleByPair := []Candle{
		{
			Time:   time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC),
			Open:   3057.67,
			Close:  3059.37,
			High:   3115.51,
			Low:    3011.00,
			Volume: 87666.8,
			Orders: []Order{
				expectOrder,
			},
		},
		{
			Time:   time.Date(2021, 9, 28, 8, 0, 0, 0, time.UTC),
			Open:   2894.18,
			Close:  2926.80,
			High:   2940.74,
			Low:    2876.12,
			Volume: 88470.1,
			Orders: []Order{
				expectLimitMakerOrder,
				expectStopOrder,
			},
		},
	}
	candles := c.candlesByPair(pair)
	require.Equal(t, expectCandleByPair, candles)

	//test shapes by pare
	expectShapesByPair := []Shape{
		{
			StartX: time.Date(2021, 9, 28, 8, 0, 0, 0, time.UTC),
			EndX:   time.Date(2021, 9, 28, 8, 0, 0, 0, time.UTC),
			StartY: 3059.37,
			EndY:   2926,
			Color:  "rgba(0, 255, 0, 0.3)",
		},
		{
			StartX: time.Date(2021, 9, 28, 8, 0, 0, 0, time.UTC),
			EndX:   time.Date(2021, 9, 28, 8, 0, 0, 0, time.UTC),
			StartY: 3059.37,
			EndY:   3000,
			Color:  "rgba(255, 0, 0, 0.3)",
		},
	}
	shaped := c.shapesByPair(pair)
	require.Equal(t, expectShapesByPair, shaped)

	//test equity value by pair
	expectAsset := []assetValue{}
	expectQuote := []assetValue{}
	asset, quote := c.equityValuesByPair(pair)
	require.Equal(t, expectAsset, asset)
	require.Equal(t, expectQuote, quote)

	//test indicator by pair
	expectIndicators := []plotIndicator{}
	indicators := c.indicatorsByPair(pair)
	require.Equal(t, expectIndicators, indicators)
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
