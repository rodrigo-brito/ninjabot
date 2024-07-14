package tools

import (
	"errors"

	"github.com/rodrigo-brito/ninjabot"
	"github.com/rodrigo-brito/ninjabot/model"
)

type TrailingStop struct {
	current float64
	stop    float64
	active  bool
	side    ninjabot.SideType
}

func NewTrailingStop() *TrailingStop {
	return &TrailingStop{}
}

func (t *TrailingStop) Start(side ninjabot.SideType, current, stop float64) error {
	if side == model.SideTypeBuy && stop > current {
		return errors.New("stop > current in long position")
	}
	if side == model.SideTypeSell && stop < current {
		return errors.New("stop < current in short position")
	}

	t.stop = stop
	t.current = current
	t.active = true
	t.side = side

	return nil
}

func (t *TrailingStop) Stop() {
	t.active = false
}

func (t *TrailingStop) Active() bool {
	return t.active
}

func (t *TrailingStop) Update(current float64) bool {
	if !t.active {
		return false
	}

	if t.side == model.SideTypeBuy {
		return t.updateLong(current)
	}

	return t.updateShort(current)
}

func (t *TrailingStop) updateLong(current float64) bool {
	if current > t.current {
		diff := current - t.current
		t.stop = t.stop + diff
		t.current = current
		return false
	}

	t.current = current
	return current <= t.stop
}

func (t *TrailingStop) updateShort(current float64) bool {
	if current < t.current {
		diff := t.current - current
		t.stop = t.stop - diff
		t.current = current
		return false
	}

	t.current = current
	return current >= t.stop
}
