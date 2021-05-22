package notification

import (
	"fmt"
	"net/smtp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/rodrigo-brito/ninjabot/pkg/model"
)

type Mail struct {
	SMTPServerAddress string
	Email             string
	Password          string
}

func (t Mail) Notify(text string) {
	err := smtp.SendMail(t.SMTPServerAddress,
		smtp.PlainAuth("", t.Email, t.Password, domain(t.SMTPServerAddress)),
		t.Email, []string{t.Email}, []byte(text))
	if err != nil {
		log.
			WithError(err).
			Errorf("notification/mail: couldnt send mail")
	}
}

func (t Mail) OnOrder(order model.Order) {
	title := ""
	switch order.Status {
	case model.OrderStatusTypeFilled:
		title = fmt.Sprintf("✅ ORDER FILLED - %s", order.Symbol)
	case model.OrderStatusTypeNew:
		title = fmt.Sprintf("🆕 NEW ORDER - %s", order.Symbol)
	case model.OrderStatusTypeCanceled, model.OrderStatusTypeRejected:
		title = fmt.Sprintf("❌ ORDER CANCELED / REJECTED - %s", order.Symbol)
	}
	message := fmt.Sprintf("%s\n-----\n%s", title, order)
	t.Notify(message)
}

func (t Mail) OrError(err error) {
	title := "🛑 ERROR"
	message := fmt.Sprintf("%s\n-----\n%s", title, err)
	t.Notify(message)
}

func domain(url string) string {
	splitted := strings.Split(url, ":")
	if len(splitted) == 0 {
		return url
	}

	domain := splitted[0]

	return domain
}
