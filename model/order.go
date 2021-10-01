package model

import (
	"fmt"
	"time"
)

type SideType string
type OrderType string
type OrderStatusType string

var (
	SideTypeBuy  SideType = "BUY"
	SideTypeSell SideType = "SELL"

	OrderTypeLimit           OrderType = "LIMIT"
	OrderTypeMarket          OrderType = "MARKET"
	OrderTypeLimitMaker      OrderType = "LIMIT_MAKER"
	OrderTypeStopLoss        OrderType = "STOP_LOSS"
	OrderTypeStopLossLimit   OrderType = "STOP_LOSS_LIMIT"
	OrderTypeTakeProfit      OrderType = "TAKE_PROFIT"
	OrderTypeTakeProfitLimit OrderType = "TAKE_PROFIT_LIMIT"

	OrderStatusTypeNew             OrderStatusType = "NEW"
	OrderStatusTypePartiallyFilled OrderStatusType = "PARTIALLY_FILLED"
	OrderStatusTypeFilled          OrderStatusType = "FILLED"
	OrderStatusTypeCanceled        OrderStatusType = "CANCELED"
	OrderStatusTypePendingCancel   OrderStatusType = "PENDING_CANCEL"
	OrderStatusTypeRejected        OrderStatusType = "REJECTED"
	OrderStatusTypeExpired         OrderStatusType = "EXPIRED"
)

type Order struct {
	ID         int64           `db:"id"`
	ExchangeID int64           `db:"exchange_id"`
	Symbol     string          `db:"symbol"`
	Side       SideType        `db:"side"`
	Type       OrderType       `db:"type"`
	Status     OrderStatusType `db:"status"`
	Price      float64         `db:"price"`
	Quantity   float64         `db:"quantity"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

	// OCO Orders only
	Stop    *float64 `db:"stop"`
	GroupID *int64   `db:"group_id"`

	// Internal use (Plot)
	Profit float64
	Candle Candle
}

func (o Order) String() string {
	return fmt.Sprintf("[%s] %s %s | ID: %d, Type: %s, %f x $%f (~$%.f)",
		o.Status, o.Side, o.Symbol, o.ID, o.Type, o.Quantity, o.Price, o.Quantity*o.Price)
}
