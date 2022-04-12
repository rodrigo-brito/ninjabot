package model

import (
	"fmt"
	"strconv"
	"time"
)

type TelegramSettings struct {
	Enabled bool
	Token   string
	Users   []int
}

type Settings struct {
	Pairs    []string
	Telegram TelegramSettings
}

type Balance struct {
	Tick string
	Free float64
	Lock float64
}

type Assets struct {
	Pair      string
	AssetTick string
	AssetSize float64
	QuoteTick string
	QuoteSize float64
}

type AssetInfo struct {
	BaseAsset  string
	QuoteAsset string

	MinPrice    float64
	MaxPrice    float64
	MinQuantity float64
	MaxQuantity float64
	StepSize    float64
	TickSize    float64

	// Number of decimal places
	QtyDecimalPrecision   int64
	PriceDecimalPrecision int64
}

type Dataframe struct {
	Pair string

	Close  Series
	Open   Series
	High   Series
	Low    Series
	Volume Series

	Time       []time.Time
	LastUpdate time.Time

	// Custom user metadata
	Metadata map[string]Series
}

type Candle struct {
	Pair     string
	Time     time.Time
	Open     float64
	Close    float64
	Low      float64
	High     float64
	Volume   float64
	Trades   int64
	Complete bool
}

func (c Candle) ToSlice(precision int) []string {
	return []string{
		fmt.Sprintf("%d", c.Time.Unix()),
		strconv.FormatFloat(c.Open, 'f', precision, 64),
		strconv.FormatFloat(c.Close, 'f', precision, 64),
		strconv.FormatFloat(c.Low, 'f', precision, 64),
		strconv.FormatFloat(c.High, 'f', precision, 64),
		fmt.Sprintf("%.1f", c.Volume),
		fmt.Sprintf("%d", c.Trades),
	}
}

func (c Candle) Less(j Item) bool {
	if j.(Candle).Time.Equal(c.Time) {
		return c.Pair < j.(Candle).Pair
	}
	return c.Time.Before(j.(Candle).Time)
}

type Account struct {
	Balances []Balance
}

func (a Account) Balance(assetTick, quoteTick string) (Balance, Balance) {
	var assetBalance, quoteBalance Balance
	var isSetAsset, isSetQuote bool

	for _, balance := range a.Balances {
		switch balance.Tick {
		case assetTick:
			assetBalance = balance
			isSetAsset = true
		case quoteTick:
			quoteBalance = balance
			isSetQuote = true
		}

		if isSetAsset && isSetQuote {
			break
		}
	}

	return assetBalance, quoteBalance
}

func (a Account) Equity() float64 {
	var total float64

	for _, balance := range a.Balances {
		total += balance.Free
		total += balance.Lock
	}

	return total
}
