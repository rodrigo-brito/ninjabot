package exchange

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/rodrigo-brito/ninjabot/pkg/model"

	"github.com/adshao/go-binance/v2"
)

type TradeLimit struct {
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

type Binance struct {
	ctx    context.Context
	client *binance.Client
	limits map[string]TradeLimit
}

type BinanceOption func(*Binance)

func NewBinance(ctx context.Context, apiKey, secretKey string, options ...BinanceOption) (*Binance, error) {
	client := binance.NewClient(apiKey, secretKey)
	err := client.NewPingService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("binance ping fail: %w", err)
	}

	info, err := client.NewExchangeInfoService().Do(ctx)
	if err != nil {
		return nil, err
	}

	// Initialize with orders precision and assets limits
	limits := make(map[string]TradeLimit)
	for _, info := range info.Symbols {
		tradeLimits := TradeLimit{}
		for _, filter := range info.Filters {
			if typ, ok := filter["filterType"]; ok {
				if typ == string(binance.SymbolFilterTypeLotSize) {
					tradeLimits.MinQuantity, _ = strconv.ParseFloat(filter["minQty"].(string), 64)
					tradeLimits.MaxQuantity, _ = strconv.ParseFloat(filter["maxQty"].(string), 64)
					tradeLimits.StepSize, _ = strconv.ParseFloat(filter["stepSize"].(string), 64)
					tradeLimits.QtyDecimalPrecision = model.NumDecPlaces(tradeLimits.StepSize)
				}

				if typ == string(binance.SymbolFilterTypePriceFilter) {
					tradeLimits.MinPrice, _ = strconv.ParseFloat(filter["minPrice"].(string), 64)
					tradeLimits.MaxPrice, _ = strconv.ParseFloat(filter["maxPrice"].(string), 64)
					tradeLimits.TickSize, _ = strconv.ParseFloat(filter["tickSize"].(string), 64)
					tradeLimits.PriceDecimalPrecision = model.NumDecPlaces(tradeLimits.TickSize)
				}
			}
		}
		limits[info.Symbol] = tradeLimits
	}

	exchange := &Binance{
		ctx:    ctx,
		client: client,
		limits: limits,
	}
	for _, option := range options {
		option(exchange)
	}
	return exchange, nil
}

func Debug() BinanceOption {
	return func(b *Binance) {
		b.client.Debug = true
	}
}

func (b *Binance) OrderOCO(side model.SideType, symbol string, size, price, stop, stopLimit float64) ([]model.Order, error) {
	order, err := b.client.NewCreateOCOService().
		Side(binance.SideType(side)).
		StopPrice(fmt.Sprintf("%f", stop)).
		StopLimitPrice(fmt.Sprintf("%f", stopLimit)).
		StopLimitTimeInForce(binance.TimeInForceTypeGTC).
		Price(fmt.Sprintf("%f", price)).
		Quantity(fmt.Sprintf("%f", size)).
		Symbol(symbol).
		Do(b.ctx)
	if err != nil {
		return nil, err
	}

	orders := make([]model.Order, 0, len(order.Orders))
	for _, order := range order.OrderReports {
		price, _ := strconv.ParseFloat(order.Price, 64)
		item := model.Order{
			ExchangeID: order.OrderID,
			Date:       time.Unix(0, order.TransactionTime*int64(time.Millisecond)),
			Symbol:     symbol,
			Side:       model.SideType(order.Side),
			Type:       model.OrderType(order.Type),
			Status:     model.OrderStatusType(order.Status),
			Price:      price,
			Quantity:   size,
			GroupID:    &order.OrderListID,
		}

		if item.Type == model.OrderTypeStopLossLimit || item.Type == model.OrderTypeStopLoss {
			item.Stop = &stop
		}

		orders = append(orders, item)
	}

	return orders, nil
}

func (b *Binance) formatPrice(symbol string, value float64) string {
	precision := -1
	if limits, ok := b.limits[symbol]; ok {
		precision = int(limits.PriceDecimalPrecision)
	}
	return strconv.FormatFloat(value, 'f', precision, 64)
}

func (b *Binance) formatQuantity(symbol string, value float64) string {
	precision := -1
	if limits, ok := b.limits[symbol]; ok {
		precision = int(limits.QtyDecimalPrecision)
	}
	return strconv.FormatFloat(value, 'f', precision, 64)
}

func (b *Binance) OrderLimit(side model.SideType, symbol string, quantity float64, limit float64) (model.Order, error) {
	order, err := b.client.NewCreateOrderService().
		Symbol(symbol).
		Type(binance.OrderTypeLimit).
		TimeInForce(binance.TimeInForceTypeGTC).
		Side(binance.SideType(side)).
		Quantity(b.formatQuantity(symbol, quantity)).
		Price(b.formatPrice(symbol, limit)).
		Do(b.ctx)
	if err != nil {
		return model.Order{}, err
	}

	price, _ := strconv.ParseFloat(order.Price, 64)
	quantity, _ = strconv.ParseFloat(order.OrigQuantity, 64)

	return model.Order{
		ExchangeID: order.OrderID,
		Date:       time.Unix(0, order.TransactTime*int64(time.Millisecond)),
		Symbol:     symbol,
		Side:       model.SideType(order.Side),
		Type:       model.OrderType(order.Type),
		Status:     model.OrderStatusType(order.Status),
		Price:      price,
		Quantity:   quantity,
	}, nil
}

func (b *Binance) OrderMarket(side model.SideType, symbol string, quantity float64) (model.Order, error) {
	order, err := b.client.NewCreateOrderService().
		Symbol(symbol).
		Type(binance.OrderTypeMarket).
		Side(binance.SideType(side)).
		Quantity(b.formatQuantity(symbol, quantity)).
		NewOrderRespType(binance.NewOrderRespTypeFULL).
		Do(b.ctx)
	if err != nil {
		return model.Order{}, err
	}

	cost, _ := strconv.ParseFloat(order.CummulativeQuoteQuantity, 64)
	quantity, _ = strconv.ParseFloat(order.ExecutedQuantity, 64)
	return model.Order{
		ExchangeID: order.OrderID,
		Date:       time.Unix(0, order.TransactTime*int64(time.Millisecond)),
		Symbol:     order.Symbol,
		Side:       model.SideType(order.Side),
		Type:       model.OrderType(order.Type),
		Status:     model.OrderStatusType(order.Status),
		Price:      cost / quantity,
		Quantity:   quantity,
	}, nil
}

func (b *Binance) Cancel(order model.Order) error {
	_, err := b.client.NewCancelOrderService().
		Symbol(order.Symbol).
		OrderID(order.ExchangeID).
		Do(b.ctx)
	return err
}

func (b *Binance) Account() (model.Account, error) {
	acc, err := b.client.NewGetAccountService().Do(b.ctx)
	if err != nil {
		return model.Account{}, err
	}

	balances := make([]model.Balance, 0)
	for _, balance := range acc.Balances {
		free, _ := strconv.ParseFloat(balance.Free, 64)
		locked, _ := strconv.ParseFloat(balance.Locked, 64)
		balances = append(balances, model.Balance{
			Tick:  balance.Asset,
			Value: free + locked,
			Lock:  locked,
		})
	}

	return model.Account{
		Balances: balances,
	}, nil
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
	candle.Trades = k.TradeNum
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
	candle.Trades = k.TradeNum
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
