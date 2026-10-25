package metrics

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBlocks(t *testing.T) {
	blocks := Blocks([]float64{1, 2, 3, 4, 5}, 2)
	require.Len(t, blocks, 2)
	require.Equal(t, Block{N: 3, Sum: 6, SumSq: 14}, blocks[0])
	require.Equal(t, Block{N: 2, Sum: 9, SumSq: 41}, blocks[1])
	require.Nil(t, Blocks([]float64{1}, 2))
	require.Nil(t, Blocks([]float64{1, 2}, 0))
}

func TestSharpeOfMatchesSharpe(t *testing.T) {
	returns := []float64{0.01, -0.02, 0.03, 0.005, -0.01, 0.02, 0.0, 0.015}
	blocks := Blocks(returns, 4)
	require.InDelta(t, Sharpe(returns), sharpeOf(blocks, []int{0, 1, 2, 3}), 1e-12)
}

func noise(random *rand.Rand, n int, drift float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = drift + random.NormFloat64()*0.01
	}
	return out
}

func TestPBO(t *testing.T) {
	random := rand.New(rand.NewSource(3)) //nolint:gosec

	t.Run("noise is overfit", func(t *testing.T) {
		configs := make([][]Block, 50)
		for i := range configs {
			configs[i] = Blocks(noise(random, 1000, 0), 10)
		}
		result := PBO(configs)
		require.Equal(t, 252, result.Splits, "C(10, 5)")
		require.InDelta(t, 0.5, result.PBO, 0.2)
	})

	t.Run("a real edge is not", func(t *testing.T) {
		configs := make([][]Block, 50)
		for i := range configs {
			configs[i] = Blocks(noise(random, 1000, 0), 10)
		}
		configs[7] = Blocks(noise(random, 1000, 0.004), 10)
		result := PBO(configs)
		require.Less(t, result.PBO, 0.05)
	})

	t.Run("degenerate inputs", func(t *testing.T) {
		require.Zero(t, PBO(nil).Splits)
		require.Zero(t, PBO([][]Block{Blocks(noise(random, 10, 0), 2)}).Splits)
		odd := [][]Block{Blocks(noise(random, 30, 0), 3), Blocks(noise(random, 30, 0), 3)}
		require.Zero(t, PBO(odd).Splits)
		uneven := [][]Block{Blocks(noise(random, 30, 0), 2), Blocks(noise(random, 30, 0), 4)}
		require.Zero(t, PBO(uneven).Splits)
	})
}
