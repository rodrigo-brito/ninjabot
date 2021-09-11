package ninjabot

import (
	"github.com/rodrigo-brito/ninjabot/model"
)

type (
	Settings         = model.Settings
	TelegramSettings = model.TelegramSettings
	Dataframe        = model.Dataframe
	Series           = model.Series
	SideType         = model.SideType
	OrderType        = model.OrderType
	OrderStatusType  = model.OrderStatusType
)

var (
	SideTypeBuy                    = model.SideTypeBuy
	SideTypeSell                   = model.SideTypeSell
	OrderTypeLimit                 = model.OrderTypeLimit
	OrderTypeMarket                = model.OrderTypeMarket
	OrderTypeLimitMaker            = model.OrderTypeLimitMaker
	OrderTypeStopLoss              = model.OrderTypeStopLoss
	OrderTypeStopLossLimit         = model.OrderTypeStopLossLimit
	OrderTypeTakeProfit            = model.OrderTypeTakeProfit
	OrderTypeTakeProfitLimit       = model.OrderTypeTakeProfitLimit
	OrderStatusTypeNew             = model.OrderStatusTypeNew
	OrderStatusTypePartiallyFilled = model.OrderStatusTypePartiallyFilled
	OrderStatusTypeFilled          = model.OrderStatusTypeFilled
	OrderStatusTypeCanceled        = model.OrderStatusTypeCanceled
	OrderStatusTypePendingCancel   = model.OrderStatusTypePendingCancel
	OrderStatusTypeRejected        = model.OrderStatusTypeRejected
	OrderStatusTypeExpired         = model.OrderStatusTypeExpired
)
