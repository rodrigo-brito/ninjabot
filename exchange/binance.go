package exchange

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"

	"github.com/adshao/go-binance/v2"
	"github.com/jpillora/backoff"
	log "github.com/sirupsen/logrus"
)

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

type UserInfo struct {
	MakerCommission float64
	TakerCommission float64
}

type Binance struct {
	ctx        context.Context
	client     *binance.Client
	assetsInfo map[string]AssetInfo
	userInfo   UserInfo

	APIKey    string
	APISecret string
}

type BinanceOption func(*Binance)

func WithBinanceCredentials(key, secret string) BinanceOption {
	return func(b *Binance) {
		b.APIKey = key
		b.APISecret = secret
	}
}

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

	// If user credentials are present
	if exchange.APIKey != "" && exchange.APISecret != "" {
		// Initialize user capabilities and fees
		acc, err := exchange.client.NewGetAccountService().Do(ctx)
		if err != nil {
			return nil, err
		}

		exchange.userInfo = UserInfo{
			MakerCommission: float64(acc.MakerCommission) / 10000.0,
			TakerCommission: float64(acc.TakerCommission) / 10000.0,
		}
	}

	results, err := exchange.client.NewExchangeInfoService().Do(ctx)
	if err != nil {
		return nil, err
	}

	// Initialize with orders precision and assets limits
	exchange.assetsInfo = make(map[string]AssetInfo)
	for _, info := range results.Symbols {
		tradeLimits := AssetInfo{
			BaseAsset:  info.BaseAsset,
			QuoteAsset: info.QuoteAsset,
		}
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
		exchange.assetsInfo[info.Symbol] = tradeLimits
	}

	log.Info("[SETUP] Using Binance exchange")

	return exchange, nil
}

func (b *Binance) validate(side model.SideType, typ model.OrderType, pair string, quantity float64,
	value *float64) error {

	info, ok := b.assetsInfo[pair]
	if !ok {
		return ErrInvalidAsset
	}

	if quantity > info.MaxQuantity || quantity < info.MinQuantity {
		return fmt.Errorf("%w: min: %f max: %f", ErrInvalidQuantity, info.MinQuantity, info.MaxQuantity)
	}

	account, err := b.Account()
	if err != nil {
		return err
	}

	commissionFactor := 1 + b.userInfo.MakerCommission
	if typ == model.OrderTypeMarket || typ == model.OrderTypeLimitMaker ||
		typ == model.OrderTypeStopLoss || typ == model.OrderTypeTakeProfit {
		commissionFactor = 1 + b.userInfo.TakerCommission
	}

	if side == model.SideTypeBuy {
		if value == nil {
			candles, err := b.CandlesByLimit(b.ctx, pair, "1m", 1)
			if err != nil {
				return err
			}
			value = &candles[0].Close
		}

		if value != nil && account.Balance(info.QuoteAsset).Free < quantity*(*value)*commissionFactor {
			return ErrInsufficientFunds
		}
	}

	if side == model.SideTypeSell && account.Balance(info.BaseAsset).Free < quantity*commissionFactor {
		return ErrInsufficientFunds
	}

	return nil
}

func (b *Binance) CreateOrderOCO(side model.SideType, pair string,
	quantity, price, stop, stopLimit float64) ([]model.Order, error) {

	// validate stop
	err := b.validate(side, model.OrderTypeStopLossLimit, pair, quantity, &stopLimit)
	if err != nil {
		return nil, err
	}

	// validate take profit
	err = b.validate(side, model.OrderTypeLimitMaker, pair, quantity, &price)
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

func (b *Binance) OrderStop(pair string, quantity float64, limit float64) (model.Order, error) {
	err := b.validate(model.SideTypeSell, model.OrderTypeStopLoss, pair, quantity, &limit)
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
	precision := -1
	if limits, ok := b.assetsInfo[pair]; ok {
		precision = int(limits.PriceDecimalPrecision)
	}
	return strconv.FormatFloat(value, 'f', precision, 64)
}

func (b *Binance) formatQuantity(pair string, value float64) string {
	precision := -1
	if limits, ok := b.assetsInfo[pair]; ok {
		precision = int(limits.QtyDecimalPrecision)
	}
	return strconv.FormatFloat(value, 'f', precision, 64)
}

func (b *Binance) CreateOrderLimit(side model.SideType, pair string,
	quantity float64, limit float64) (model.Order, error) {

	err := b.validate(side, model.OrderTypeLimit, pair, quantity, &limit)
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

func (b *Binance) CreateOrderMarket(side model.SideType, pair string, quantity float64) (model.Order, error) {
	err := b.validate(side, model.OrderTypeMarket, pair, quantity, nil)
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

	cost, _ := strconv.ParseFloat(order.CummulativeQuoteQuantity, 64)
	quantity, _ = strconv.ParseFloat(order.ExecutedQuantity, 64)
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
	err := b.validate(side, model.OrderTypeMarket, pair, quantity, nil)
	if err != nil {
		return model.Order{}, err
	}

	order, err := b.client.NewCreateOrderService().
		Symbol(pair).
		Type(binance.OrderTypeMarket).
		Side(binance.SideType(side)).
		QuoteOrderQty(fmt.Sprintf("%.f", quantity)).
		NewOrderRespType(binance.NewOrderRespTypeFULL).
		Do(b.ctx)
	if err != nil {
		return model.Order{}, err
	}

	cost, _ := strconv.ParseFloat(order.CummulativeQuoteQuantity, 64)
	quantity, _ = strconv.ParseFloat(order.ExecutedQuantity, 64)
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
		free, _ := strconv.ParseFloat(balance.Free, 64)
		locked, _ := strconv.ParseFloat(balance.Locked, 64)
		balances = append(balances, model.Balance{
			Tick: balance.Asset,
			Free: free,
			Lock: locked,
		})
	}

	return model.Account{
		Balances: balances,
	}, nil
}

func (b *Binance) Position(pair string) (asset, quote float64, err error) {
	assetTick, quoteTick := SplitAssetQuote(pair)
	acc, err := b.Account()
	if err != nil {
		return 0, 0, err
	}

	assetBalance := acc.Balance(assetTick)
	quoteBalance := acc.Balance(quoteTick)

	return assetBalance.Free + assetBalance.Lock, quoteBalance.Free + quoteBalance.Lock, nil
}

func (b *Binance) CandlesSubscription(ctx context.Context, pair, period string) (chan model.Candle, chan error) {
	ccandle := make(chan model.Candle)
	cerr := make(chan error)

	go func() {
		b := &backoff.Backoff{
			Min: 100 * time.Millisecond,
			Max: 1 * time.Second,
		}
		for {
			done, _, err := binance.WsKlineServe(pair, period, func(event *binance.WsKlineEvent) {
				b.Reset()
				ccandle <- CandleFromWsKline(pair, event.Kline)
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
				time.Sleep(b.Duration())
			}
		}
	}()

	return ccandle, cerr
}

func (b *Binance) CandlesByLimit(ctx context.Context, pair, period string, limit int) ([]model.Candle, error) {
	candles := make([]model.Candle, 0)
	klineService := b.client.NewKlinesService()

	data, err := klineService.Symbol(pair).
		Interval(period).
		Limit(limit + 1).
		Do(ctx)

	if err != nil {
		return nil, err
	}

	for _, d := range data {
		candles = append(candles, CandleFromKline(pair, *d))
	}

	// discard last candle, because it is incomplete
	return candles[:len(candles)-1], nil
}

func (b *Binance) CandlesByPeriod(ctx context.Context, pair, period string,
	start, end time.Time) ([]model.Candle, error) {

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
		candles = append(candles, CandleFromKline(pair, *d))
	}

	return candles, nil
}

func CandleFromKline(pair string, k binance.Kline) model.Candle {
	candle := model.Candle{Pair: pair, Time: time.Unix(0, k.OpenTime*int64(time.Millisecond))}
	candle.Open, _ = strconv.ParseFloat(k.Open, 64)
	candle.Close, _ = strconv.ParseFloat(k.Close, 64)
	candle.High, _ = strconv.ParseFloat(k.High, 64)
	candle.Low, _ = strconv.ParseFloat(k.Low, 64)
	candle.Volume, _ = strconv.ParseFloat(k.Volume, 64)
	candle.Trades = k.TradeNum
	candle.Complete = true
	return candle
}

func CandleFromWsKline(pair string, k binance.WsKline) model.Candle {
	candle := model.Candle{Pair: pair, Time: time.Unix(0, k.StartTime*int64(time.Millisecond))}
	candle.Open, _ = strconv.ParseFloat(k.Open, 64)
	candle.Close, _ = strconv.ParseFloat(k.Close, 64)
	candle.High, _ = strconv.ParseFloat(k.High, 64)
	candle.Low, _ = strconv.ParseFloat(k.Low, 64)
	candle.Volume, _ = strconv.ParseFloat(k.Volume, 64)
	candle.Trades = k.TradeNum
	candle.Complete = k.IsFinal
	return candle
}
