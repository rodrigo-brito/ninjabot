package plot

import (
	"github.com/StudioSol/set"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestChart_orderStringByPair(t *testing.T) {
	c, err := NewChart()
	require.NoErrorf(t, err, "check error when initial chart")

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
