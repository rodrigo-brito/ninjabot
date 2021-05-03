package notification

import "github.com/rodrigo-brito/ninjabot/pkg/model"

type Notifier interface {
	NotifyOrder(order model.Order)
	NotifyError(err error)
}
