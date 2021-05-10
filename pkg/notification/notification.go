package notification

import "github.com/rodrigo-brito/ninjabot/pkg/model"

type Notifier interface {
	Notify(string)
	OnOrder(order model.Order)
	OrError(err error)
}
