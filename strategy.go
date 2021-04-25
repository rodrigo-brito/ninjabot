package main

type Strategy interface {
	Init(settings *Settings)
	Indicators(dataframe *Dataframe)
	OnCandle(dataframe *Dataframe, broker Broker)
}
