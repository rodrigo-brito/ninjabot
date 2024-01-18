package model

import (
	"time"
)

type MarginMode string

const (
	Isolated MarginMode = "ISOLATED"
	Cross    MarginMode = "CROSS"

	MAKER_FEE = 0.02
	TAKER_FEE = 0.04
)

type Position struct {
	ID            int64    `db:"id" json:"id" gorm:"primaryKey,autoIncrement"`
	ExchangeID    int64    `db:"exchange_id" json:"exchange_id"`
	Pair          string   `db:"pair" json:"pair"`
	Side          SideType `db:"side" json:"side"`
	EntryPrice    float64  `db:"entry_price" json:"entry_price"`
	AvgPrice      float64  `db:"avg_price" json:"avg_price"`
	Quantity      float64  `db:"quantity" json:"quantity"` //Asset e.g BTC
	Size          float64  `db:"size" json:"size"`         // Quote e.g USDT
	Active        bool     `db:"active" json:"active"`
	Safe          bool     `db:"safe" json:"safe"`
	Leverage      int
	CurrentCandle *Candle
	MarginMode    MarginMode `db:"margin_mode" json:"margin_mode"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
	ClosedAt  time.Time `db:"closed_at" json:"closed_at"`
}

func (p *Position) Closed() bool {
	return p.ClosedAt.IsZero()
}

func NewPosition(pair string, side SideType, size float64, leverage int, candle Candle) Position {
	position := Position{
		Pair:       pair,
		Size:       size,
		Quantity:   size * float64(leverage),
		Leverage:   leverage,
		Side:       side,
		CreatedAt:  candle.Time,
		EntryPrice: candle.Close,
		Safe:       false,
	}

	return position
}

type Result struct {
	Pair          string
	ProfitPercent float64
	ProfitValue   float64
	Side          SideType
	Duration      time.Duration
	CreatedAt     time.Time
}

func (p *Position) OppositeSide() SideType {
	if p.Side == SideTypeBuy {
		return SideTypeSell
	}

	return SideTypeBuy
}

func (p *Position) Update(order *Order) (result *Result, finished bool) {
	price := order.Price
	if order.Type == OrderTypeStopLoss || order.Type == OrderTypeStopLossLimit {
		price = *order.Stop
	}

	if p.Side == order.Side {
		// Increase the position
		p.AvgPrice = (p.AvgPrice*p.Quantity + price*order.Quantity) / (p.Quantity + order.Quantity)
		p.Quantity += order.Quantity
	} else {
		if p.Quantity == order.Quantity {
			// Close the position if the order quantity is opposite, and it's equal with position quantity
			finished = true
			//p.ClosedAt = p.CurrentCandle.Time
		} else if p.Quantity > order.Quantity {
			// Reduce the position quantity
			p.Quantity -= order.Quantity
		} else {
			// When the order quantity is higher than position quantity and the side is opposite,
			// Change the position side
			p.Quantity = order.Quantity - p.Quantity
			p.Side = order.Side
			p.CreatedAt = order.CreatedAt
			p.AvgPrice = price
		}

		// For that case when we just reduce the position not close it entirely ?! TBD
		//quantity := math.Min(p.Quantity, order.Quantity)

		if order.Reason == "LIQUIDATED" {
			order.Profit = -1
			order.ProfitValue = -p.Size
		} else {
			// Because we already multiply by 100 in the chart.js file (TBD) /100
			order.Profit = p.ROI()
			order.ProfitValue = p.UnrealizedPNL()
		}

		//order.Profit = (price - p.AvgPrice) / p.AvgPrice
		//order.ProfitValue = (price - p.AvgPrice) * quantity

		result = &Result{
			CreatedAt:     order.CreatedAt,
			Pair:          order.Pair,
			Duration:      order.CreatedAt.Sub(p.CreatedAt),
			ProfitPercent: order.Profit,
			ProfitValue:   order.ProfitValue,
			Side:          p.Side,
		}

		return result, finished
	}

	return nil, false
}

func (p *Position) Profit() float64 {
	//return float64(p.NumericSide()) * (p.MarketPrice() - p.EntryPrice) * p.Size
	return (1.0/p.EntryPrice - 1.0/p.MarketPrice()) * p.Size * p.NumericSide()
}

func (p *Position) UnrealizedPNL() float64 {
	return p.Quantity * float64(p.Leverage) * p.NumericSide() * (p.MarketPrice() - p.EntryPrice)
}

func (p *Position) EntryMargin() float64 {
	return p.Quantity * float64(p.Leverage) * p.MarketPrice() * p.imr()
}

func (p *Position) NumericSide() float64 {
	if p.Side == SideTypeBuy {
		return 1.0
	}

	return -1.0
}

func (p *Position) MarketPrice() float64 {
	return p.CurrentCandle.Close
}

//func (p *Position) Amount() float64 {
//	return p.Quantity / p.EntryPrice
//}

// ROI Return of investment
func (p *Position) ROI() float64 {
	return p.UnrealizedPNL() / p.EntryMargin() * 100
}

func (p *Position) InitialMargin() float64 {
	return p.Quantity * p.EntryPrice * (1.0 / float64(p.Leverage))
}

func (p *Position) Commission() float64 {
	openFee := p.Quantity * p.EntryPrice * MAKER_FEE / 100
	closeFee := p.Quantity * p.MarketPrice() * TAKER_FEE / 100

	return openFee + closeFee
}

func (p *Position) Duration() time.Duration {
	return p.CurrentCandle.Time.Sub(p.CreatedAt)
}

func (p *Position) Liquidated() bool {
	return p.ROI() <= 100
}

func (p *Position) imr() float64 {
	return 1.0 / float64(p.Leverage)
}
