package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
)

// BenchmarkChart_OnCandle measures the work a closed candle triggers with a
// dashboard connected (the candle event with every indicator), for a fixed
// history size.
func BenchmarkChart_OnCandle(b *testing.B) {
	for _, history := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("history=%d", history), func(b *testing.B) {
			c, err := New(WithStrategyIndicators(fakeStrategy{}), WithCustomIndicators(&counterIndicator{}))
			if err != nil {
				b.Fatal(err)
			}
			c.events.subscribe() // a connected dashboard; full buffers drop events

			base := time.Unix(1_600_000_000, 0).UTC()
			for i := 0; i < history; i++ {
				candle := candleAt("BTCUSDT", base.Add(time.Duration(i)*time.Hour), 100+float64(i%10))
				// the strategy controller reports its indicators before the candle reaches the chart
				c.OnIndicators(candle, fakeStrategy{}.Indicators(&model.Dataframe{
					Time: []time.Time{candle.Time}, Close: model.Series[float64]{candle.Close},
				}))
				c.OnCandle(candle)
			}

			b.ReportAllocs()
			b.ResetTimer()
			last := c.candles["BTCUSDT"][history-1]
			for i := 0; i < b.N; i++ {
				c.mu.Lock()
				c.candleEvent("BTCUSDT", last)
				c.mu.Unlock()
			}
		})
	}
}
