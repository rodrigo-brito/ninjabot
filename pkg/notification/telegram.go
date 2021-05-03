package notification

import (
	"fmt"
	"net/http"
	"net/url"

	log "github.com/sirupsen/logrus"

	"github.com/rodrigo-brito/ninjabot/pkg/model"
)

type Telegram struct {
	ID        string
	Key       string
	ChannelID string
}

func NewTelegram(id string, key string, channel string) Telegram {
	return Telegram{
		ID:        id,
		Key:       key,
		ChannelID: channel,
	}
}

func (t Telegram) Notify(text string) {
	baseUrl, err := url.Parse(fmt.Sprintf("https://api.telegram.org/bot%s:%s/sendMessage", t.ID, t.Key))
	if err != nil {
		log.Error(err)
	}

	params := url.Values{}
	params.Add("chat_id", t.ChannelID)
	params.Add("text", text)

	baseUrl.RawQuery = params.Encode()
	resp, err := http.Get(baseUrl.String())
	if err != nil {
		log.Error(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		log.Errorf("notification/telegram: %d - %s", resp.StatusCode, resp.Status)
	}
}

func (t Telegram) NotifyOrder(order model.Order) {
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

func (t Telegram) NotifyError(err error) {
	title := "🛑 ERROR"
	message := fmt.Sprintf("%s\n-----\n%s", title, err)
	t.Notify(message)
}
