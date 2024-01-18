package exchange

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/common"
	"github.com/jpillora/backoff"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/tools/log"
)

type MetadataFetchers func(pair string, t time.Time) (string, float64)

type Binance struct {
	ctx        context.Context
	client     *binance.Client
	assetsInfo map[string]model.AssetInfo
	HeikinAshi bool
	Testnet    bool

	APIKey    string
	APISecret string

	MetadataFetchers []MetadataFetchers
}

type BinanceOption func(*Binance)

// WithBinanceCredentials will set Binance credentials
func WithBinanceCredentials(key, secret string) BinanceOption {
	return func(b *Binance) {
		b.APIKey = key
		b.APISecret = secret
	}
}

// WithBinanceHeikinAshiCandle will convert candle to Heikin Ashi
func WithBinanceHeikinAshiCandle() BinanceOption {
	return func(b *Binance) {
		b.HeikinAshi = true
	}
}

// WithMetadataFetcher will execute a function after receive a new candle and include additional
// information to candle's metadata
func WithMetadataFetcher(fetcher MetadataFetchers) BinanceOption {
	return func(b *Binance) {
		b.MetadataFetchers = append(b.MetadataFetchers, fetcher)
	}
}

// WithTestNet activate Bianance testnet
func WithTestNet() BinanceOption {
	return func(b *Binance) {
		binance.UseTestnet = true
	}
}

// NewBinance create a new Binance exchange instance
func NewBinance(ctx context.Context, options ...BinanceOption) (*Binance, error) {
	binance.WebsocketKeepalive = true
	exchange := &Binance{ctx: ctx}
	for _, option := range options {
		option(exchange)
	}

	exchange.client = binance.NewClient(exchange.APIKey, exchange.APISecret)
	err := exchange.client.NewPingService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("binance ping fail: %w", err)
	}

	results, err := exchange.client.NewExchangeInfoService().Do(ctx)
	if err != nil {
		return nil, err
	}

	// Initialize with orders precision and assets limits
	exchange.assetsInfo = make(map[string]model.AssetInfo)
	for _, info := range results.Symbols {
		tradeLimits := model.AssetInfo{
			BaseAsset:          info.BaseAsset,
			QuoteAsset:         info.QuoteAsset,
			BaseAssetPrecision: info.BaseAssetPrecision,
			QuotePrecision:     info.QuotePrecision,
		}
		for _, filter := range info.Filters {
			if typ, ok := filter["filterType"]; ok {
				if typ == string(binance.SymbolFilterTypeLotSize) {
					tradeLimits.MinQuantity, _ = strconv.ParseFloat(filter["minQty"].(string), 64)
					tradeLimits.MaxQuantity, _ = strconv.ParseFloat(filter["maxQty"].(string), 64)
					tradeLimits.StepSize, _ = strconv.ParseFloat(filter["stepSize"].(string), 64)
				}

				if typ == string(binance.SymbolFilterTypePriceFilter) {
					tradeLimits.MinPrice, _ = strconv.ParseFloat(filter["minPrice"].(string), 64)
					tradeLimits.MaxPrice, _ = strconv.ParseFloat(filter["maxPrice"].(string), 64)
					tradeLimits.TickSize, _ = strconv.ParseFloat(filter["tickSize"].(string), 64)
				}
			}
		}
		exchange.assetsInfo[info.Symbol] = tradeLimits
	}

	log.Info("[SETUP] Using Binance exchange")

	return exchange, nil
}

func (b *Binance) LastQuote(ctx context.Context, pair string) (float64, error) {
	candles, err := b.CandlesByLimit(ctx, pair, "1m", 1)
	if err != nil || len(candles) < 1 {
		return 0, err
	}
	return candles[0].Close, nil
}

func (b *Binance) AssetsInfo(pair string) model.AssetInfo {
	return b.assetsInfo[pair]
}

func (b *Binance) validate(pair string, quantity float64) error {
	info, ok := b.assetsInfo[pair]
	if !ok {
		return ErrInvalidAsset
	}

	if quantity > info.MaxQuantity || quantity < info.MinQuantity {
		return &OrderError{
			Err:      fmt.Errorf("%w: min: %f max: %f", ErrInvalidQuantity, info.MinQuantity, info.MaxQuantity),
			Pair:     pair,
			Quantity: quantity,
		}
	}

	return nil
}

func (b *Binance) CreateOrderOCO(side model.SideType, pair string,
	quantity, price, stop, stopLimit float64) ([]model.Order, error) {

	// validate stop
	err := b.validate(pair, quantity)
	if err != nil {
		return nil, err
	}

	ocoOrder, err := b.client.NewCreateOCOService().
		Side(binance.SideType(side)).
		Quantity(b.formatQuantity(pair, quantity)).
		Price(b.formatPrice(pair, price)).
		StopPrice(b.formatPrice(pair, stop)).
		StopLimitPrice(b.formatPrice(pair, stopLimit)).
		StopLimitTimeInForce(binance.TimeInForceTypeGTC).
		Symbol(pair).
		Do(b.ctx)
	if err != nil {
		return nil, err
	}

	orders := make([]model.Order, 0, len(ocoOrder.Orders))
	for _, order := range ocoOrder.OrderReports {
		price, _ := strconv.ParseFloat(order.Price, 64)
		quantity, _ := strconv.ParseFloat(order.OrigQuantity, 64)
		item := model.Order{
			ExchangeID: order.OrderID,
			CreatedAt:  time.Unix(0, ocoOrder.TransactionTime*int64(time.Millisecond)),
			UpdatedAt:  time.Unix(0, ocoOrder.TransactionTime*int64(time.Millisecond)),
			Pair:       pair,
			Side:       model.SideType(order.Side),
			Type:       model.OrderType(order.Type),
			Status:     model.OrderStatusType(order.Status),
			Price:      price,
			Quantity:   quantity,
			GroupID:    &order.OrderListID,
		}

		if item.Type == model.OrderTypeStopLossLimit || item.Type == model.OrderTypeStopLoss {
			item.Stop = &stop
		}

		orders = append(orders, item)
	}

	return orders, nil
}

func (b *Binance) CreateOrderStop(pair string, quantity float64, limit float64) (model.Order, error) {
	err := b.validate(pair, quantity)
	if err != nil {
		return model.Order{}, err
	}

	order, err := b.client.NewCreateOrderService().Symbol(pair).
		Type(binance.OrderTypeStopLoss).
		TimeInForce(binance.TimeInForceTypeGTC).
		Side(binance.SideTypeSell).
		Quantity(b.formatQuantity(pair, quantity)).
		Price(b.formatPrice(pair, limit)).
		Do(b.ctx)
	if err != nil {
		return model.Order{}, err
	}

	price, _ := strconv.ParseFloat(order.Price, 64)
	quantity, _ = strconv.ParseFloat(order.OrigQuantity, 64)

	return model.Order{
		ExchangeID: order.OrderID,
		CreatedAt:  time.Unix(0, order.TransactTime*int64(time.Millisecond)),
		UpdatedAt:  time.Unix(0, order.TransactTime*int64(time.Millisecond)),
		Pair:       pair,
		Side:       model.SideType(order.Side),
		Type:       model.OrderType(order.Type),
		Status:     model.OrderStatusType(order.Status),
		Price:      price,
		Quantity:   quantity,
	}, nil
}

func (b *Binance) formatPrice(pair string, value float64) string {
	if info, ok := b.assetsInfo[pair]; ok {
		value = common.AmountToLotSize(info.TickSize, info.QuotePrecision, value)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func (b *Binance) formatQuantity(pair string, value float64) string {
	if info, ok := b.assetsInfo[pair]; ok {
		value = common.AmountToLotSize(info.StepSize, info.BaseAssetPrecision, value)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func (b *Binance) CreateOrderLimit(side model.SideType, pair string,
	quantity float64, limit float64) (model.Order, error) {

	err := b.validate(pair, quantity)
	if err != nil {
		return model.Order{}, err
	}

	order, err := b.client.NewCreateOrderService().
		Symbol(pair).
		Type(binance.OrderTypeLimit).
		TimeInForce(binance.TimeInForceTypeGTC).
		Side(binance.SideType(side)).
		Quantity(b.formatQuantity(pair, quantity)).
		Price(b.formatPrice(pair, limit)).
		Do(b.ctx)
	if err != nil {
		return model.Order{}, err
	}

	price, err := strconv.ParseFloat(order.Price, 64)
	if err != nil {
		return model.Order{}, err
	}

	quantity, err = strconv.ParseFloat(order.OrigQuantity, 64)
	if err != nil {
		return model.Order{}, err
	}

	return model.Order{
		ExchangeID: order.OrderID,
		CreatedAt:  time.Unix(0, order.TransactTime*int64(time.Millisecond)),
		UpdatedAt:  time.Unix(0, order.TransactTime*int64(time.Millisecond)),
		Pair:       pair,
		Side:       model.SideType(order.Side),
		Type:       model.OrderType(order.Type),
		Status:     model.OrderStatusType(order.Status),
		Price:      price,
		Quantity:   quantity,
	}, nil
}

func (b *Binance) CreateOrderMarket(side model.SideType, pair string, quantity float64) (model.Order, error) {
	err := b.validate(pair, quantity)
	if err != nil {
		return model.Order{}, err
	}

	order, err := b.client.NewCreateOrderService().
		Symbol(pair).
		Type(binance.OrderTypeMarket).
		Side(binance.SideType(side)).
		Quantity(b.formatQuantity(pair, quantity)).
		NewOrderRespType(binance.NewOrderRespTypeFULL).
		Do(b.ctx)
	if err != nil {
		return model.Order{}, err
	}

	cost, err := strconv.ParseFloat(order.CummulativeQuoteQuantity, 64)
	if err != nil {
		return model.Order{}, err
	}

	quantity, err = strconv.ParseFloat(order.ExecutedQuantity, 64)
	if err != nil {
		return model.Order{}, err
	}

	return model.Order{
		ExchangeID: order.OrderID,
		CreatedAt:  time.Unix(0, order.TransactTime*int64(time.Millisecond)),
		UpdatedAt:  time.Unix(0, order.TransactTime*int64(time.Millisecond)),
		Pair:       order.Symbol,
		Side:       model.SideType(order.Side),
		Type:       model.OrderType(order.Type),
		Status:     model.OrderStatusType(order.Status),
		Price:      cost / quantity,
		Quantity:   quantity,
	}, nil
}

func (b *Binance) CreateOrderMarketQuote(side model.SideType, pair string, quantity float64) (model.Order, error) {
	err := b.validate(pair, quantity)
	if err != nil {
		return model.Order{}, err
	}

	order, err := b.client.NewCreateOrderService().
		Symbol(pair).
		Type(binance.OrderTypeMarket).
		Side(binance.SideType(side)).
		QuoteOrderQty(b.formatQuantity(pair, quantity)).
		NewOrderRespType(binance.NewOrderRespTypeFULL).
		Do(b.ctx)
	if err != nil {
		return model.Order{}, err
	}

	cost, err := strconv.ParseFloat(order.CummulativeQuoteQuantity, 64)
	if err != nil {
		return model.Order{}, err
	}

	quantity, err = strconv.ParseFloat(order.ExecutedQuantity, 64)
	if err != nil {
		return model.Order{}, err
	}

	return model.Order{
		ExchangeID: order.OrderID,
		CreatedAt:  time.Unix(0, order.TransactTime*int64(time.Millisecond)),
		UpdatedAt:  time.Unix(0, order.TransactTime*int64(time.Millisecond)),
		Pair:       order.Symbol,
		Side:       model.SideType(order.Side),
		Type:       model.OrderType(order.Type),
		Status:     model.OrderStatusType(order.Status),
		Price:      cost / quantity,
		Quantity:   quantity,
	}, nil
}

func (b *Binance) Cancel(order model.Order) error {
	_, err := b.client.NewCancelOrderService().
		Symbol(order.Pair).
		OrderID(order.ExchangeID).
		Do(b.ctx)
	return err
}

func (b *Binance) Orders(pair string, limit int) ([]model.Order, error) {
	result, err := b.client.NewListOrdersService().
		Symbol(pair).
		Limit(limit).
		Do(b.ctx)

	if err != nil {
		return nil, err
	}

	orders := make([]model.Order, 0)
	for _, order := range result {
		orders = append(orders, newOrder(order))
	}
	return orders, nil
}

func (b *Binance) Order(pair string, id int64) (model.Order, error) {
	order, err := b.client.NewGetOrderService().
		Symbol(pair).
		OrderID(id).
		Do(b.ctx)

	if err != nil {
		return model.Order{}, err
	}

	return newOrder(order), nil
}

func newOrder(order *binance.Order) model.Order {
	var price float64
	cost, _ := strconv.ParseFloat(order.CummulativeQuoteQuantity, 64)
	quantity, _ := strconv.ParseFloat(order.ExecutedQuantity, 64)
	if cost > 0 && quantity > 0 {
		price = cost / quantity
	} else {
		price, _ = strconv.ParseFloat(order.Price, 64)
		quantity, _ = strconv.ParseFloat(order.OrigQuantity, 64)
	}

	return model.Order{
		ExchangeID: order.OrderID,
		Pair:       order.Symbol,
		CreatedAt:  time.Unix(0, order.Time*int64(time.Millisecond)),
		UpdatedAt:  time.Unix(0, order.UpdateTime*int64(time.Millisecond)),
		Side:       model.SideType(order.Side),
		Type:       model.OrderType(order.Type),
		Status:     model.OrderStatusType(order.Status),
		Price:      price,
		Quantity:   quantity,
	}
}

func (b *Binance) Account() (model.Account, error) {
	acc, err := b.client.NewGetAccountService().Do(b.ctx)
	if err != nil {
		return model.Account{}, err
	}

	balances := make([]model.Balance, 0)
	for _, balance := range acc.Balances {
		free, err := strconv.ParseFloat(balance.Free, 64)
		if err != nil {
			return model.Account{}, err
		}
		locked, err := strconv.ParseFloat(balance.Locked, 64)
		if err != nil {
			return model.Account{}, err
		}
		balances = append(balances, model.Balance{
			Asset: balance.Asset,
			Free:  free,
			Lock:  locked,
		})
	}

	return model.Account{
		Balances: balances,
	}, nil
}

//func (b *Binance) Position(pair string) (position float64, err error) {
//	assetTick, quoteTick := SplitAssetQuote(pair)
//	acc, err := b.Account()
//	if err != nil {
//		return 0, 0, err
//	}
//
//	assetBalance, quoteBalance := acc.Balance(assetTick, quoteTick)
//
//	return assetBalance.Free + assetBalance.Lock, quoteBalance.Free + quoteBalance.Lock, nil
//}

func (b *Binance) CandlesSubscription(ctx context.Context, pair, period string) (chan model.Candle, chan error) {
	ccandle := make(chan model.Candle)
	cerr := make(chan error)
	ha := model.NewHeikinAshi()

	go func() {
		ba := &backoff.Backoff{
			Min: 100 * time.Millisecond,
			Max: 1 * time.Second,
		}

		for {
			done, _, err := binance.WsKlineServe(pair, period, func(event *binance.WsKlineEvent) {
				ba.Reset()
				candle := CandleFromWsKline(pair, event.Kline)

				if candle.Complete && b.HeikinAshi {
					candle = candle.ToHeikinAshi(ha)
				}

				if candle.Complete {
					// fetch aditional data if needed
					for _, fetcher := range b.MetadataFetchers {
						key, value := fetcher(pair, candle.Time)
						candle.Metadata[key] = value
					}
				}

				ccandle <- candle

			}, func(err error) {
				cerr <- err
			})
			if err != nil {
				cerr <- err
				close(cerr)
				close(ccandle)
				return
			}

			select {
			case <-ctx.Done():
				close(cerr)
				close(ccandle)
				return
			case <-done:
				time.Sleep(ba.Duration())
			}
		}
	}()

	return ccandle, cerr
}

func (b *Binance) CandlesByLimit(ctx context.Context, pair, period string, limit int) ([]model.Candle, error) {
	candles := make([]model.Candle, 0)
	klineService := b.client.NewKlinesService()
	ha := model.NewHeikinAshi()

	data, err := klineService.Symbol(pair).
		Interval(period).
		Limit(limit + 1).
		Do(ctx)

	if err != nil {
		return nil, err
	}

	for _, d := range data {
		candle := CandleFromKline(pair, *d)

		if b.HeikinAshi {
			candle = candle.ToHeikinAshi(ha)
		}

		candles = append(candles, candle)
	}

	// discard last candle, because it is incomplete
	return candles[:len(candles)-1], nil
}

func (b *Binance) CandlesByPeriod(ctx context.Context, pair, period string,
	start, end time.Time) ([]model.Candle, error) {

	candles := make([]model.Candle, 0)
	klineService := b.client.NewKlinesService()
	ha := model.NewHeikinAshi()

	data, err := klineService.Symbol(pair).
		Interval(period).
		StartTime(start.UnixNano() / int64(time.Millisecond)).
		EndTime(end.UnixNano() / int64(time.Millisecond)).
		Do(ctx)

	if err != nil {
		return nil, err
	}

	for _, d := range data {
		candle := CandleFromKline(pair, *d)

		if b.HeikinAshi {
			candle = candle.ToHeikinAshi(ha)
		}

		candles = append(candles, candle)
	}

	return candles, nil
}

func CandleFromKline(pair string, k binance.Kline) model.Candle {
	t := time.Unix(0, k.OpenTime*int64(time.Millisecond))
	candle := model.Candle{Pair: pair, Time: t, UpdatedAt: t}
	candle.Open, _ = strconv.ParseFloat(k.Open, 64)
	candle.Close, _ = strconv.ParseFloat(k.Close, 64)
	candle.High, _ = strconv.ParseFloat(k.High, 64)
	candle.Low, _ = strconv.ParseFloat(k.Low, 64)
	candle.Volume, _ = strconv.ParseFloat(k.Volume, 64)
	candle.Complete = true
	candle.Metadata = make(map[string]float64)
	return candle
}

func CandleFromWsKline(pair string, k binance.WsKline) model.Candle {
	t := time.Unix(0, k.StartTime*int64(time.Millisecond))
	candle := model.Candle{Pair: pair, Time: t, UpdatedAt: t}
	candle.Open, _ = strconv.ParseFloat(k.Open, 64)
	candle.Close, _ = strconv.ParseFloat(k.Close, 64)
	candle.High, _ = strconv.ParseFloat(k.High, 64)
	candle.Low, _ = strconv.ParseFloat(k.Low, 64)
	candle.Volume, _ = strconv.ParseFloat(k.Volume, 64)
	candle.Complete = k.IsFinal
	candle.Metadata = make(map[string]float64)
	return candle
}
