package data

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"

	"github.com/rodrigo-brito/ninjabot/pkg/exchange"
)

type Downloader struct {
	exchange exchange.Exchange
}

func NewDownloader(exchange exchange.Exchange) Downloader {
	return Downloader{
		exchange: exchange,
	}
}

func (d Downloader) Download(ctx context.Context, symbol, timeframe string, limit int, output string) error {
	recordFile, err := os.Create(output)
	if err != nil {
		return err
	}

	if limit > 1000 {
		return fmt.Errorf("invalid limit. max: 1000")
	}

	writer := csv.NewWriter(recordFile)
	candles, err := d.exchange.CandlesByLimit(ctx, symbol, timeframe, limit)
	if err != nil {
		return err
	}

	err = writer.Write([]string{"time", "open", "close", "low", "high", "volume", "trades"})
	if err != nil {
		return err
	}

	for _, candle := range candles {
		err := writer.Write(candle.ToSlice())
		if err != nil {
			return err
		}
	}

	writer.Flush()

	return writer.Error()
}
