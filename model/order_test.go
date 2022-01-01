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
		Pair:       "BNBUSDT",
		Side:       SideTypeSell,
		Type:       OrderTypeLimit,
		Status:     OrderStatusTypeFilled,
		Price:      10,
		Quantity:   1,
		CreatedAt:  time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	require.Equal(t, "[FILLED] SELL BNBUSDT | ID: 1, Type: LIMIT, 1.000000 x $10.000000 (~$10)", order.String())
}
