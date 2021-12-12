package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOrder_String(t *testing.T) {
	order := Order{
		ID:         1,
		ExchangeID: 2,
		Pair:       "BTCUSDT",
		Side:       SideTypeSell,
		Type:       OrderTypeLimit,
		Status:     OrderStatusTypeFilled,
		Price:      10,
		Quantity:   1,
		CreatedAt:  time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	require.Equal(t, "[FILLED] SELL BTCUSDT | ID: 1, Type: LIMIT, 1.000000 x $10.000000 (~$10)", order.String())
}

func TestOrder_CSVRow(t *testing.T) {
	order := Order{
		ID:         1,
		ExchangeID: 2,
		Pair:       "BTCUSDT",
		Side:       SideTypeSell,
		Type:       OrderTypeLimit,
		Status:     OrderStatusTypeFilled,
		Price:      8.586639,
		Quantity:   582.3,
		CreatedAt:  time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	expectOrder := []string{"FILLED", "SELL", "BTCUSDT", "2", "LIMIT", "582.300000", "8.586639", "5000.00", "2020-01-01 00:00:00 +0000 UTC"}
	require.Equal(t, expectOrder, order.CSVRow())
}
