package model

import (
	"fmt"
	"time"

	"github.com/rodrigo-brito/ninjabot/pkg/series"
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

type Settings struct {
	Pairs []string
}

type Balance struct {
	Tick string
	Free float64
	Lock float64
}

type Dataframe struct {
	Pair string

	Close  series.Series
	Open   series.Series
	High   series.Series
	Low    series.Series
	Volume series.Series

	Time       []time.Time
	LastUpdate time.Time

	// Custom user metadata
	Metadata map[string]series.Series
}

type Candle struct {
	Symbol   string
	Time     time.Time
	Open     float64
	Close    float64
	Low      float64
	High     float64
	Volume   float64
	Trades   int64
	Complete bool
}

func (c Candle) ToSlice() []string {
	return []string{
		fmt.Sprintf("%d", c.Time.Unix()),
		fmt.Sprintf("%f", c.Open),
		fmt.Sprintf("%f", c.Close),
		fmt.Sprintf("%f", c.Low),
		fmt.Sprintf("%f", c.High),
		fmt.Sprintf("%.1f", c.Volume),
		fmt.Sprintf("%d", c.Trades),
	}
}

type Order struct {
	ID         int64
	ExchangeID int64
	Date       time.Time
	Symbol     string
	Side       SideType
	Type       OrderType
	Status     OrderStatusType
	Price      float64
	Quantity   float64

	// OCO Orders only
	Stop    *float64
	GroupID *int64

	// Internal use (Plot)
	Profit float64
}

func (o Order) String() string {
	return fmt.Sprintf("%s %s | ID: %d, Type: %s - %f x $%f (%s)",
		o.Side, o.Symbol, o.ID, o.Type, o.Quantity, o.Price, o.Status)
}

type Account struct {
	Balances []Balance
}

func (a Account) Balance(tick string) Balance {
	for _, balance := range a.Balances {
		if balance.Tick == tick {
			return balance
		}
	}
	return Balance{}
}
