package exchange

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/rodrigo-brito/ninjabot/pkg/model"

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

func (b *Binance) OrderOCO(side OrderSide, symbol string, size, price, stop, stopLimit float64) ([]model.Order, error) {
	order, err := b.client.NewCreateOCOService().
		Side(binance.SideType(side)).
		StopPrice(fmt.Sprintf("%.f", stop)).
		StopLimitPrice(fmt.Sprintf("%.f", stopLimit)).
		Price(fmt.Sprintf("%.f", price)).
		Quantity(fmt.Sprintf("%.f", size)).
		Symbol(symbol).
		Do(context.Background())
	if err != nil {
		return nil, err
	}

	orders := make([]model.Order, 0, len(order.Orders))
	for _, o := range order.Orders {
		orders = append(orders, model.Order{
			ExchangeID: o.OrderID,
			Date:       time.Unix(0, order.TransactionTime*int64(time.Millisecond)),
			Symbol:     symbol,
			Side:       model.SideType(side),
			Status:     model.OrderStatusTypeNew,
			Type:       "",
			Price:      0,
			Quantity:   size,
			GroupID:    &order.OrderListID,
		})
	}

	return orders, nil
}

func (b *Binance) OrderLimit(side OrderSide, symbol string, size float64, limit float64) (model.Order, error) {
	order, err := b.client.NewCreateOrderService().
		Symbol(symbol).
		Type(binance.OrderTypeLimit).
		TimeInForce(binance.TimeInForceTypeGTC).
		Side(binance.SideType(side)).
		Quantity(fmt.Sprintf("%.f", size)).
		Price(fmt.Sprintf("%.f", limit)).
		Do(context.Background())
	if err != nil {
		return model.Order{}, err
	}

	price, _ := strconv.ParseFloat(order.Price, 64)
	return model.Order{
		ID:       order.OrderID,
		Symbol:   order.Symbol,
		Date:     time.Unix(0, order.TransactTime*int64(time.Millisecond)),
		Side:     model.SideType(order.Side),
		Type:     model.OrderType(order.Type),
		Status:   model.OrderStatusType(order.Status),
		Price:    price,
		Quantity: size,
	}, nil
}

func (b *Binance) OrderMarket(side OrderSide, symbol string, size float64) (model.Order, error) {
	order, err := b.client.NewCreateOrderService().
		Symbol(symbol).
		Type(binance.OrderTypeMarket).
		TimeInForce(binance.TimeInForceTypeGTC).
		Side(binance.SideType(side)).
		Quantity(fmt.Sprintf("%.f", size)).
		Do(context.Background())
	if err != nil {
		return model.Order{}, err
	}

	price, _ := strconv.ParseFloat(order.Price, 64)
	return model.Order{
		ExchangeID: order.OrderID,
		Date:       time.Unix(0, order.TransactTime*int64(time.Millisecond)),
		Symbol:     order.Symbol,
		Side:       model.SideType(order.Side),
		Type:       model.OrderType(order.Type),
		Status:     model.OrderStatusType(order.Status),
		Price:      price,
		Quantity:   size,
	}, nil
}

func (b *Binance) Cancel(order model.Order) error {
	_, err := b.client.NewCancelOrderService().
		Symbol(order.Symbol).
		OrderID(order.ID).Do(context.Background())
	return err
}

func (b *Binance) Account() (model.Account, error) {
	// TODO: implement me
	return model.Account{}, nil
}

func (b *Binance) SubscribeCandles(pair, period string) (chan model.Candle, chan error) {
	ccandle := make(chan model.Candle)
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

func (b *Binance) LoadCandlesByLimit(ctx context.Context, pair, period string, limit int) ([]model.Candle, error) {
	candles := make([]model.Candle, 0)
	klineService := b.client.NewKlinesService()

	data, err := klineService.Symbol(pair).
		Interval(period).
		Limit(limit).
		Do(ctx)

	if err != nil {
		return nil, err
	}

	for _, d := range data {
		candles = append(candles, CandleFromKline(*d))
	}

	return candles, nil
}

func (b *Binance) LoadCandlesByPeriod(ctx context.Context, pair, period string, start, end time.Time) ([]model.Candle, error) {
	candles := make([]model.Candle, 0)
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

func CandleFromKline(k binance.Kline) model.Candle {
	candle := model.Candle{Time: time.Unix(0, k.OpenTime*int64(time.Millisecond))}
	candle.Open, _ = strconv.ParseFloat(k.Open, 64)
	candle.Close, _ = strconv.ParseFloat(k.Close, 64)
	candle.High, _ = strconv.ParseFloat(k.High, 64)
	candle.Low, _ = strconv.ParseFloat(k.Low, 64)
	candle.Volume, _ = strconv.ParseFloat(k.Volume, 64)
	candle.Complete = true
	return candle
}

func CandleFromWsKline(k binance.WsKline) model.Candle {
	candle := model.Candle{Time: time.Unix(0, k.StartTime*int64(time.Millisecond))}
	candle.Open, _ = strconv.ParseFloat(k.Open, 64)
	candle.Close, _ = strconv.ParseFloat(k.Close, 64)
	candle.High, _ = strconv.ParseFloat(k.High, 64)
	candle.Low, _ = strconv.ParseFloat(k.Low, 64)
	candle.Volume, _ = strconv.ParseFloat(k.Volume, 64)
	candle.Complete = k.IsFinal
	return candle
}

func AccountFromBinance(binanceAccount *binance.Account) (*model.Account, error) {
	var account model.Account
	for _, balance := range binanceAccount.Balances {

		free, err := strconv.ParseFloat(balance.Free, 64)
		if err != nil {
			return nil, err
		}

		lock, err := strconv.ParseFloat(balance.Free, 64)
		if err != nil {
			return nil, err
		}

		account.Balances = append(account.Balances, model.Balance{
			Tick:  balance.Asset,
			Value: free + lock,
			Lock:  lock,
		})
	}

	return &account, nil
}
