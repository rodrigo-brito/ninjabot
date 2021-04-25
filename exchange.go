package main

type Exchange interface {
	Broker
	Account() (Account, error)
	SubscribeCandles(pair, period string) (<-chan Candle, <-chan error)
}

type Broker interface {
	Buy(tick string, size float64) Order
	Sell(tick string, size float64) Order
	Cancel(Order)
}
