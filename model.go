package main

import "time"

type Settings struct {
	Pairs []string
}

type Balance struct {
	Tick  string
	Value float64
	Lock  float64
}

type Dataframe struct {
	Time     []time.Time
	Close    []float64
	Open     []float64
	High     []float64
	Low      []float64
	Volume   []float64
	Metadata map[string][]float64
}

type Candle struct {
	Time     time.Time
	Open     float64
	Close    float64
	High     float64
	Low      float64
	Volume   float64
	Complete bool
}

type Order struct{}

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
