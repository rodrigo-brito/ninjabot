package main

import "context"

type Ninjabot struct {
}

func NewBot(settings *Settings, exchange Exchange) Ninjabot {
	return Ninjabot{}
}

func (n *Ninjabot) Run(ctx context.Context) error {
	return nil
}
