package exchange

import (
	"context"
	"fmt"
	"github.com/rodrigo-brito/ninjabot/service"
	"strconv"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/common"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/jpillora/backoff"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/tools/log"
)

var _ service.Feeder = &BinanceFuture{}

type MarginType = futures.MarginType

var (
	MarginTypeIsolated MarginType = "ISOLATED"
	MarginTypeCrossed  MarginType = "CROSSED"

	ErrNoNeedChangeMarginType int64 = -4046
)

type PairOption struct {
	Pair       string
	Leverage   int
	MarginType futures.MarginType
}

type BinanceFuture struct {
	ctx        context.Context
	client     *futures.Client
	assetsInfo map[string]model.AssetInfo
	HeikinAshi bool
	Testnet    bool

	APIKey    string
	APISecret string

	MetadataFetchers []MetadataFetchers
	PairOptions      []PairOption
}

type BinanceFutureOption func(*BinanceFuture)

// WithBinanceFuturesHeikinAshiCandle will use Heikin Ashi candle instead of regular candle
func WithBinanceFuturesHeikinAshiCandle() BinanceFutureOption {
	return func(b *BinanceFuture) {
		b.HeikinAshi = true
	}
}

// WithBinanceFutureCredentials will set the credentials for Binance Futures
func WithBinanceFutureCredentials(key, secret string) BinanceFutureOption {
	return func(b *BinanceFuture) {
		b.APIKey = key
		b.APISecret = secret
	}
}

// WithBinanceFutureLeverage will set the leverage for a pair
func WithBinanceFutureLeverage(pair string, leverage int, marginType MarginType) BinanceFutureOption {
	return func(b *BinanceFuture) {
		b.PairOptions = append(b.PairOptions, PairOption{
			Pair:       strings.ToUpper(pair),
			Leverage:   leverage,
			MarginType: marginType,
		})
	}
}

// NewBinanceFuture will create a new BinanceFuture instance
func NewBinanceFuture(ctx context.Context, options ...BinanceFutureOption) (*BinanceFuture, error) {
	binance.WebsocketKeepalive = true
	exchange := &BinanceFuture{ctx: ctx}
	for _, option := range options {
		option(exchange)
	}

	exchange.client = futures.NewClient(exchange.APIKey, exchange.APISecret)
	err := exchange.client.NewPingService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("binance ping fail: %w", err)
	}

	results, err := exchange.client.NewExchangeInfoService().Do(ctx)
	if err != nil {
		return nil, err
	}

	// Set leverage and margin type
	for _, option := range exchange.PairOptions {
		_, err = exchange.client.NewChangeLeverageService().Symbol(option.Pair).Leverage(option.Leverage).Do(ctx)
		if err != nil {
			return nil, err
		}

		err = exchange.client.NewChangeMarginTypeService().Symbol(option.Pair).MarginType(option.MarginType).Do(ctx)
		if err != nil {
			if apiError, ok := err.(*common.APIError); !ok || apiError.Code != ErrNoNeedChangeMarginType {
				return nil, err
			}
		}
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

	log.Info("[SETUP] Using Binance Futures exchange")

	return exchange, nil
}

func (b *BinanceFuture) LastQuote(ctx context.Context, pair string) (float64, error) {
	candles, err := b.CandlesByLimit(ctx, pair, "1m", 1)
	if err != nil || len(candles) < 1 {
		return 0, err
	}
	return candles[0].Close, nil
}

func (b *BinanceFuture) AssetsInfo(pair string) model.AssetInfo {
	return b.assetsInfo[pair]
}

func (b *BinanceFuture) validate(pair string, quantity float64) error {
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

func (b *BinanceFuture) CreateOrderOCO(_ model.SideType, _ string,
	_, _, _, _ float64) ([]model.Order, error) {
	panic("not implemented")
}

func (b *BinanceFuture) CreateOrderStop(pair string, quantity float64, limit float64) (model.Order, error) {
	err := b.validate(pair, quantity)
	if err != nil {
		return model.Order{}, err
	}

	order, err := b.client.NewCreateOrderService().Symbol(pair).
		Type(futures.OrderTypeStopMarket).
		TimeInForce(futures.TimeInForceTypeGTC).
		Side(futures.SideTypeSell).
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
		CreatedAt:  time.Unix(0, order.UpdateTime*int64(time.Millisecond)),
		UpdatedAt:  time.Unix(0, order.UpdateTime*int64(time.Millisecond)),
		Pair:       pair,
		Side:       model.SideType(order.Side),
		Type:       model.OrderType(order.Type),
		Status:     model.OrderStatusType(order.Status),
		Price:      price,
		Quantity:   quantity,
	}, nil
}

func (b *BinanceFuture) formatPrice(pair string, value float64) string {
	if info, ok := b.assetsInfo[pair]; ok {
		value = common.AmountToLotSize(info.TickSize, info.QuotePrecision, value)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func (b *BinanceFuture) formatQuantity(pair string, value float64) string {
	if info, ok := b.assetsInfo[pair]; ok {
		value = common.AmountToLotSize(info.StepSize, info.BaseAssetPrecision, value)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func (b *BinanceFuture) CreateOrderLimit(side model.SideType, pair string,
	quantity float64, limit float64) (model.Order, error) {

	err := b.validate(pair, quantity)
	if err != nil {
		return model.Order{}, err
	}

	order, err := b.client.NewCreateOrderService().
		Symbol(pair).
		Type(futures.OrderTypeLimit).
		TimeInForce(futures.TimeInForceTypeGTC).
		Side(futures.SideType(side)).
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
		CreatedAt:  time.Unix(0, order.UpdateTime*int64(time.Millisecond)),
		UpdatedAt:  time.Unix(0, order.UpdateTime*int64(time.Millisecond)),
		Pair:       pair,
		Side:       model.SideType(order.Side),
		Type:       model.OrderType(order.Type),
		Status:     model.OrderStatusType(order.Status),
		Price:      price,
		Quantity:   quantity,
	}, nil
}

func (b *BinanceFuture) CreateOrderMarket(side model.SideType, pair string, quantity float64) (model.Order, error) {
	err := b.validate(pair, quantity)
	if err != nil {
		return model.Order{}, err
	}

	order, err := b.client.NewCreateOrderService().
		Symbol(pair).
		Type(futures.OrderTypeMarket).
		Side(futures.SideType(side)).
		Quantity(b.formatQuantity(pair, quantity)).
		NewOrderResponseType(futures.NewOrderRespTypeRESULT).
		Do(b.ctx)
	if err != nil {
		return model.Order{}, err
	}

	cost, err := strconv.ParseFloat(order.CumQuote, 64)
	if err != nil {
		return model.Order{}, err
	}

	quantity, err = strconv.ParseFloat(order.ExecutedQuantity, 64)
	if err != nil {
		return model.Order{}, err
	}

	return model.Order{
		ExchangeID: order.OrderID,
		CreatedAt:  time.Unix(0, order.UpdateTime*int64(time.Millisecond)),
		UpdatedAt:  time.Unix(0, order.UpdateTime*int64(time.Millisecond)),
		Pair:       order.Symbol,
		Side:       model.SideType(order.Side),
		Type:       model.OrderType(order.Type),
		Status:     model.OrderStatusType(order.Status),
		Price:      cost / quantity,
		Quantity:   quantity,
	}, nil
}

func (b *BinanceFuture) CreateOrderMarketQuote(_ model.SideType, _ string, _ float64) (model.Order, error) {
	panic("not implemented")
}

func (b *BinanceFuture) Cancel(order model.Order) error {
	_, err := b.client.NewCancelOrderService().
		Symbol(order.Pair).
		OrderID(order.ExchangeID).
		Do(b.ctx)
	return err
}

func (b *BinanceFuture) Orders(pair string, limit int) ([]model.Order, error) {
	result, err := b.client.NewListOrdersService().
		Symbol(pair).
		Limit(limit).
		Do(b.ctx)

	if err != nil {
		return nil, err
	}

	orders := make([]model.Order, 0)
	for _, order := range result {
		orders = append(orders, newFutureOrder(order))
	}
	return orders, nil
}

func (b *BinanceFuture) Order(pair string, id int64) (model.Order, error) {
	order, err := b.client.NewGetOrderService().
		Symbol(pair).
		OrderID(id).
		Do(b.ctx)

	if err != nil {
		return model.Order{}, err
	}

	return newFutureOrder(order), nil
}

func newFutureOrder(order *futures.Order) model.Order {
	var (
		price float64
		err   error
	)
	cost, _ := strconv.ParseFloat(order.CumQuote, 64)
	quantity, _ := strconv.ParseFloat(order.ExecutedQuantity, 64)
	if cost > 0 && quantity > 0 {
		price = cost / quantity
	} else {
		price, err = strconv.ParseFloat(order.Price, 64)
		log.CheckErr(log.WarnLevel, err)
		quantity, err = strconv.ParseFloat(order.OrigQuantity, 64)
		log.CheckErr(log.WarnLevel, err)
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

func (b *BinanceFuture) Account() (model.Account, error) {
	acc, err := b.client.NewGetAccountService().Do(b.ctx)
	if err != nil {
		return model.Account{}, err
	}

	balances := make([]model.Balance, 0)
	for _, position := range acc.Positions {
		free, err := strconv.ParseFloat(position.PositionAmt, 64)
		if err != nil {
			return model.Account{}, err
		}

		if free == 0 {
			continue
		}

		leverage, err := strconv.ParseFloat(position.Leverage, 64)
		if err != nil {
			return model.Account{}, err
		}

		if position.PositionSide == futures.PositionSideTypeShort {
			free = -free
		}

		asset, _ := SplitAssetQuote(position.Symbol)

		balances = append(balances, model.Balance{
			Asset:    asset,
			Free:     free,
			Leverage: leverage,
		})
	}

	for _, asset := range acc.Assets {
		free, err := strconv.ParseFloat(asset.WalletBalance, 64)
		if err != nil {
			return model.Account{}, err
		}

		if free == 0 {
			continue
		}

		balances = append(balances, model.Balance{
			Asset: asset.Asset,
			Free:  free,
		})
	}

	return model.Account{
		Balances: balances,
	}, nil
}

func (b *BinanceFuture) Position(pair string) (asset, quote float64, err error) {
	assetTick, quoteTick := SplitAssetQuote(pair)
	acc, err := b.Account()
	if err != nil {
		return 0, 0, err
	}

	assetBalance, quoteBalance := acc.Balance(assetTick, quoteTick)

	return assetBalance.Free + assetBalance.Lock, quoteBalance.Free + quoteBalance.Lock, nil
}

func (b *BinanceFuture) CandlesSubscriptionCombined(ctx context.Context,
	pairs map[string]string) (chan model.Candle, chan error) {
	ccandle := make(chan model.Candle)
	cerr := make(chan error)
	ha := model.NewHeikinAshi()

	go b.candleSubscription(ctx, ha, ccandle, cerr, "", "", pairs, true)

	return ccandle, cerr
}

func (b *BinanceFuture) CandlesSubscription(ctx context.Context, pair, period string) (chan model.Candle, chan error) {
	ccandle := make(chan model.Candle)
	cerr := make(chan error)
	ha := model.NewHeikinAshi()

	go b.candleSubscription(ctx, ha, ccandle, cerr, pair, period, nil, false)

	return ccandle, cerr
}

func (b *BinanceFuture) CandlesByLimit(ctx context.Context, pair, period string, limit int) ([]model.Candle, error) {
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
		candle := FutureCandleFromKline(pair, *d)

		if b.HeikinAshi {
			candle = candle.ToHeikinAshi(ha)
		}

		candles = append(candles, candle)
	}

	// discard last candle, because it is incomplete
	return candles[:len(candles)-1], nil
}

func (b *BinanceFuture) CandlesByPeriod(ctx context.Context, pair, period string,
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
		candle := FutureCandleFromKline(pair, *d)

		if b.HeikinAshi {
			candle = candle.ToHeikinAshi(ha)
		}

		candles = append(candles, candle)
	}

	return candles, nil
}

func (b *BinanceFuture) candleHandler(ba *backoff.Backoff, ha *model.HeikinAshi,
	ccandle chan<- model.Candle) futures.WsKlineHandler {
	return func(event *futures.WsKlineEvent) {
		ba.Reset()
		candle := FutureCandleFromWsKline(event.Symbol, event.Kline)

		if candle.Complete && b.HeikinAshi {
			candle = candle.ToHeikinAshi(ha)
		}

		if candle.Complete {
			// fetch aditional data if needed
			for _, fetcher := range b.MetadataFetchers {
				key, value := fetcher(event.Symbol, candle.Time)
				candle.Metadata[key] = value
			}
		}

		ccandle <- candle
	}
}

func (b *BinanceFuture) candleSubscription(ctx context.Context, ha *model.HeikinAshi,
	ccandle chan<- model.Candle, cerr chan<- error, pair, period string, pairs map[string]string, combined bool) {
	ba := &backoff.Backoff{
		Min: 100 * time.Millisecond,
		Max: 1 * time.Second,
	}

	for {
		handler := b.candleHandler(ba, ha, ccandle)

		var done chan struct{}
		var err error

		if combined {
			done, _, err = futures.WsCombinedKlineServe(pairs, handler, func(err error) {
				cerr <- err
			})
		} else {
			done, _, err = futures.WsKlineServe(pair, period, handler, func(err error) {
				cerr <- err
			})
		}
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
}

func FutureCandleFromKline(pair string, k futures.Kline) model.Candle {
	var err error
	t := time.Unix(0, k.OpenTime*int64(time.Millisecond))
	candle := model.Candle{Pair: pair, Time: t, UpdatedAt: t}
	candle.Open, err = strconv.ParseFloat(k.Open, 64)
	log.CheckErr(log.WarnLevel, err)
	candle.Close, err = strconv.ParseFloat(k.Close, 64)
	log.CheckErr(log.WarnLevel, err)
	candle.High, err = strconv.ParseFloat(k.High, 64)
	log.CheckErr(log.WarnLevel, err)
	candle.Low, err = strconv.ParseFloat(k.Low, 64)
	log.CheckErr(log.WarnLevel, err)
	candle.Volume, err = strconv.ParseFloat(k.Volume, 64)
	log.CheckErr(log.WarnLevel, err)
	candle.Complete = true
	candle.Metadata = make(map[string]float64)
	return candle
}

func FutureCandleFromWsKline(pair string, k futures.WsKline) model.Candle {
	var err error
	t := time.Unix(0, k.StartTime*int64(time.Millisecond))
	candle := model.Candle{Pair: pair, Time: t, UpdatedAt: t}
	candle.Open, err = strconv.ParseFloat(k.Open, 64)
	log.CheckErr(log.WarnLevel, err)
	candle.Close, err = strconv.ParseFloat(k.Close, 64)
	log.CheckErr(log.WarnLevel, err)
	candle.High, err = strconv.ParseFloat(k.High, 64)
	log.CheckErr(log.WarnLevel, err)
	candle.Low, err = strconv.ParseFloat(k.Low, 64)
	log.CheckErr(log.WarnLevel, err)
	candle.Volume, err = strconv.ParseFloat(k.Volume, 64)
	log.CheckErr(log.WarnLevel, err)
	candle.Complete = k.IsFinal
	candle.Metadata = make(map[string]float64)
	return candle
}
