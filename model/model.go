package model

import (
	"fmt"
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

func (c Candle) Less(j Item) bool {
	if j.(Candle).Time.Equal(c.Time) {
		return c.Pair < j.(Candle).Pair
	}
	return c.Time.Before(j.(Candle).Time)
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
