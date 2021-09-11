package data

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloader_candlesCount(t *testing.T) {
	tt := []struct {
		start     time.Time
		end       time.Time
		timeframe string
		interval  time.Duration
		total     int
	}{
		{time.Now(), time.Now().AddDate(0, 0, 10), "1d", time.Hour * 24, 10},
		{time.Now(), time.Now().Add(60 * time.Minute), "1m", time.Minute, 60},
		{time.Now(), time.Now().Add(60 * time.Minute), "15m", 15 * time.Minute, 4},
	}

	for _, tc := range tt {
		total, interval, err := candlesCount(tc.start, tc.end, tc.timeframe)
		require.NoError(t, err)
		assert.Equal(t, tc.total, total)
		assert.Equal(t, tc.interval, interval)
	}
}
