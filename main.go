package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	var (
		apiKey    = os.Getenv("TEST_KEY")
		secretKey = os.Getenv("TEST_SECRET")
		ctx       = context.Background()
	)

	binance := NewBinance(apiKey, secretKey)
	candles, err := binance.LoadCandles(ctx, "BTCUSDT", "1h", time.Now().AddDate(0, 0, -1), time.Now())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(candles)

	fmt.Println("-- live --")
	ccandle, cerr := binance.SubscribeCandles("BTCUSDT", "1m")
	for {
		select {
		case candle := <-ccandle:
			fmt.Println(candle)
		case err := <-cerr:
			log.Fatal(err)
		}
	}
}
