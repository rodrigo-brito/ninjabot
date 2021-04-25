package main

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2"
)

type Binance struct {
	client *binance.Client
}

type BinanceOption func(*Binance)

func NewBinance(apiKey, secretKey string, options ...BinanceOption) *Binance {
	client := binance.NewClient(apiKey, secretKey)
	exchange := &Binance{
		client: client,
	}
	for _, option := range options {
		option(exchange)
	}
	return exchange
}

func Debug() BinanceOption {
	return func(b *Binance) {
		b.client.Debug = true
	}
}

func WithLogger(logger *log.Logger) BinanceOption {
	return func(b *Binance) {
		b.client.Logger = logger
	}
}

func (b *Binance) Buy(pair string, size float64) Order {
	panic("implement me")
}

func (b *Binance) Sell(pair string, size float64) Order {
	panic("implement me")
}

func (b *Binance) Cancel(order Order) {
	panic("implement me")
}

func (b *Binance) Account() (Account, error) {
	panic("implement me")
}

func (b *Binance) SubscribeCandles(pair, period string) (<-chan Candle, <-chan error) {
	ccandle := make(chan Candle)
	cerr := make(chan error)

	go func() {
		done, _, err := binance.WsKlineServe(pair, period, func(event *binance.WsKlineEvent) {
			ccandle <- CandleFromWsKline(event.Kline)
		}, func(err error) {
			cerr <- err
		})
		if err != nil {
			cerr <- err
			close(cerr)
			close(ccandle)
			return
		}
		<-done
		close(cerr)
		close(ccandle)
	}()

	return ccandle, cerr
}

func (b *Binance) LoadCandles(ctx context.Context, pair, period string, start, end time.Time) ([]Candle, error) {
	candles := make([]Candle, 0)
	klineService := b.client.NewKlinesService()

	data, err := klineService.Symbol(pair).
		Interval(period).
		StartTime(start.UnixNano() / int64(time.Millisecond)).
		EndTime(end.UnixNano() / int64(time.Millisecond)).
		Do(ctx)

	if err != nil {
		return nil, err
	}

	for _, d := range data {
		candles = append(candles, CandleFromKline(*d))
	}

	return candles, nil
}

func CandleFromKline(k binance.Kline) Candle {
	candle := Candle{Time: time.Unix(0, k.OpenTime*int64(time.Millisecond))}
	candle.Open, _ = strconv.ParseFloat(k.Open, 64)
	candle.Close, _ = strconv.ParseFloat(k.Close, 64)
	candle.High, _ = strconv.ParseFloat(k.High, 64)
	candle.Low, _ = strconv.ParseFloat(k.Low, 64)
	candle.Volume, _ = strconv.ParseFloat(k.Volume, 64)
	candle.Complete = true
	return candle
}

func CandleFromWsKline(k binance.WsKline) Candle {
	candle := Candle{Time: time.Unix(0, k.StartTime*int64(time.Millisecond))}
	candle.Open, _ = strconv.ParseFloat(k.Open, 64)
	candle.Close, _ = strconv.ParseFloat(k.Close, 64)
	candle.High, _ = strconv.ParseFloat(k.High, 64)
	candle.Low, _ = strconv.ParseFloat(k.Low, 64)
	candle.Volume, _ = strconv.ParseFloat(k.Volume, 64)
	candle.Complete = k.IsFinal
	return candle
}

func AccountFromBinance(binanceAccount *binance.Account) (*Account, error) {
	var account Account
	for _, balance := range binanceAccount.Balances {

		free, err := strconv.ParseFloat(balance.Free, 64)
		if err != nil {
			return nil, err
		}

		lock, err := strconv.ParseFloat(balance.Free, 64)
		if err != nil {
			return nil, err
		}

		account.Balances = append(account.Balances, Balance{
			Tick:  balance.Asset,
			Value: free + lock,
			Lock:  lock,
		})
	}

	return &account, nil
}
