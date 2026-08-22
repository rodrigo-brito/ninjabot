package ui

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/order"
)

// OrderController is the subset of *order.Controller used by the dashboard
// control panel (start/stop and manual market orders).
type OrderController interface {
	Status() order.Status
	Start()
	Stop()
	Position(pair string) (asset, quote float64, err error)
	CreateOrderMarket(side model.SideType, pair string, size float64) (model.Order, error)
	CreateOrderMarketQuote(side model.SideType, pair string, amount float64) (model.Order, error)
}

// SetOrderController enables the bot control panel on the dashboard:
// start/stop and manual buy/sell orders, mirroring the Telegram commands.
// Call it before Start, typically with bot.Controller(). Without a controller
// the control endpoints report disabled and the panel is hidden.
//
// Anyone with access to the dashboard port can operate the bot once this is
// set; keep the port private (localhost, VPN or an authenticating proxy).
func (c *Chart) SetOrderController(controller OrderController) {
	c.mu.Lock()
	c.controller = controller
	c.mu.Unlock()
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

func (c *Chart) controlsResponse() ControlsResponse {
	c.mu.Lock()
	controller := c.controller
	c.mu.Unlock()

	if controller == nil {
		return ControlsResponse{}
	}
	return ControlsResponse{Enabled: true, Status: string(controller.Status())}
}

// requireController returns the configured controller or replies 404.
func (c *Chart) requireController(w http.ResponseWriter) (OrderController, bool) {
	c.mu.Lock()
	controller := c.controller
	c.mu.Unlock()

	if controller == nil {
		writeJSONError(w, http.StatusNotFound, "bot controls are not enabled")
		return nil, false
	}
	return controller, true
}

func (c *Chart) handleControls(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, c.controlsResponse())
}

func (c *Chart) handleControlStart(w http.ResponseWriter, _ *http.Request) {
	controller, ok := c.requireController(w)
	if !ok {
		return
	}

	controller.Start()
	response := c.controlsResponse()
	c.events.publish("controls", response)
	writeJSON(w, response)
}

func (c *Chart) handleControlStop(w http.ResponseWriter, _ *http.Request) {
	controller, ok := c.requireController(w)
	if !ok {
		return
	}

	controller.Stop()
	response := c.controlsResponse()
	c.events.publish("controls", response)
	writeJSON(w, response)
}

func (c *Chart) handleControlOrder(w http.ResponseWriter, r *http.Request) {
	controller, ok := c.requireController(w)
	if !ok {
		return
	}

	var request ControlOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	pair := strings.ToUpper(strings.TrimSpace(request.Pair))
	c.mu.Lock()
	_, known := c.candles[pair]
	c.mu.Unlock()
	if !known {
		writeJSONError(w, http.StatusNotFound, "unknown pair")
		return
	}

	if request.Amount <= 0 {
		writeJSONError(w, http.StatusBadRequest, "amount must be positive")
		return
	}
	if request.Percent && request.Amount > 100 {
		writeJSONError(w, http.StatusBadRequest, "percentage cannot exceed 100")
		return
	}

	var side model.SideType
	switch strings.ToLower(request.Side) {
	case "buy":
		side = model.SideTypeBuy
	case "sell":
		side = model.SideTypeSell
	default:
		writeJSONError(w, http.StatusBadRequest, `side must be "buy" or "sell"`)
		return
	}

	created, err := createControlOrder(controller, side, pair, request.Amount, request.Percent)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, order.ErrBotStopped) {
			status = http.StatusConflict
		}
		writeJSONError(w, status, err.Error())
		return
	}

	c.mu.Lock()
	dto := c.toOrder(created, c.candles[pair])
	c.mu.Unlock()
	writeJSON(w, dto)
}

// createControlOrder mirrors the Telegram /buy and /sell semantics: amounts
// are in quote currency, percentages consume the available balance — quote
// balance for buys, asset balance for sells.
func createControlOrder(controller OrderController, side model.SideType, pair string,
	amount float64, percent bool) (model.Order, error) {
	if percent {
		asset, quote, err := controller.Position(pair)
		if err != nil {
			return model.Order{}, err
		}

		if side == model.SideTypeSell {
			return controller.CreateOrderMarket(side, pair, amount*asset/100.0)
		}
		amount = amount * quote / 100.0
	}

	return controller.CreateOrderMarketQuote(side, pair, amount)
}
