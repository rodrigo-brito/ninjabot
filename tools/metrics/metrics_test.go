package metrics

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMean(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		expected float64
	}{
		{
			name:     "empty slice",
			values:   []float64{},
			expected: math.NaN(),
		},
		{
			name:     "single value",
			values:   []float64{10},
			expected: 10,
		},
		{
			name:     "multiple values",
			values:   []float64{10, 20, 30},
			expected: 20,
		},
		{
			name:     "negative values",
			values:   []float64{-10, 0, 10},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Mean(tt.values)
			if math.IsNaN(tt.expected) {
				assert.True(t, math.IsNaN(result))
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestPayoff(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		expected float64
	}{
		{
			name:     "empty slice",
			values:   []float64{},
			expected: 0,
		},
		{
			name:     "only wins",
			values:   []float64{10, 20},
			expected: 0,
		},
		{
			name:     "only loses",
			values:   []float64{-10, -20},
			expected: 0,
		},
		{
			name:     "wins and loses",
			values:   []float64{10, -5},
			expected: 2, // avgWin=10, avgLose=-5, abs(10/-5) = 2
		},
		{
			name:     "zero values",
			values:   []float64{0, 0},
			expected: 0, // avgWin=0, avgLose=0, 0/0 -> NaN -> 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Payoff(tt.values))
		})
	}
}

func TestProfitFactor(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		expected float64
	}{
		{
			name:     "empty slice",
			values:   []float64{},
			expected: 10, // loses == 0 returns 10
		},
		{
			name:     "only wins",
			values:   []float64{10, 20},
			expected: 10, // loses == 0 returns 10
		},
		{
			name:     "only loses",
			values:   []float64{-10, -20},
			expected: 0, // wins=0, loses=-30, 0/-30 = 0
		},
		{
			name:     "wins and loses",
			values:   []float64{10, -5},
			expected: 2, // wins=10, loses=-5, abs(10/-5) = 2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ProfitFactor(tt.values))
		})
	}
}
