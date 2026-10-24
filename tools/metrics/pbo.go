package metrics

import (
	"math"
	"sort"
)

// Block summarizes the returns of one time block of a strategy: enough to
// combine blocks and compute their mean and standard deviation exactly.
type Block struct {
	N     int     `json:"n"`
	Sum   float64 `json:"sum"`
	SumSq float64 `json:"sum_sq"`
}

// Blocks splits returns into count consecutive blocks of (almost) equal size.
func Blocks(returns []float64, count int) []Block {
	if count <= 0 || len(returns) < count {
		return nil
	}
	blocks := make([]Block, count)
	for i, r := range returns {
		b := &blocks[i*count/len(returns)]
		b.N++
		b.Sum += r
		b.SumSq += r * r
	}
	return blocks
}

// sharpeOf is the mean over the standard deviation of the combined blocks.
func sharpeOf(blocks []Block, indices []int) float64 {
	var n, sum, sumSq float64
	for _, i := range indices {
		n += float64(blocks[i].N)
		sum += blocks[i].Sum
		sumSq += blocks[i].SumSq
	}
	if n < 2 {
		return 0
	}
	mean := sum / n
	variance := (sumSq - n*mean*mean) / (n - 1)
	if variance <= 0 {
		return 0
	}
	return mean / math.Sqrt(variance)
}

// PBOResult is the probability of backtest overfitting of a family of
// strategies, estimated by combinatorially symmetric cross-validation.
type PBOResult struct {
	// PBO is the share of splits where the best in-sample configuration
	// ranks below the median out of sample.
	PBO float64 `json:"pbo"`
	// Splits is the number of in-sample / out-of-sample splits evaluated.
	Splits int `json:"splits"`
	// Logits are the logits of the out-of-sample relative rank of the best
	// in-sample configuration, one per split: negative means below median.
	Logits []float64 `json:"logits,omitempty"`
}

// PBO estimates the probability of backtest overfitting (Bailey, Borwein,
// López de Prado and Zhu, 2015) of N configurations whose returns were each
// split in the same S time blocks (S even). For every way of choosing S/2
// blocks as in-sample, the configuration with the best in-sample Sharpe is
// ranked out of sample; PBO is the share of splits where it falls at or
// below the median. Close to 0 means the selection generalizes; close to 0.5
// or above means it is noise.
func PBO(configs [][]Block) PBOResult {
	if len(configs) < 2 {
		return PBOResult{}
	}
	s := len(configs[0])
	if s < 2 || s%2 != 0 {
		return PBOResult{}
	}
	for _, blocks := range configs {
		if len(blocks) != s {
			return PBOResult{}
		}
	}

	result := PBOResult{}
	below := 0
	combinations(s, s/2, func(inSample []int) {
		outSample := complement(s, inSample)

		best, bestSharpe := 0, math.Inf(-1)
		for i, blocks := range configs {
			if sharpe := sharpeOf(blocks, inSample); sharpe > bestSharpe {
				best, bestSharpe = i, sharpe
			}
		}

		out := make([]float64, len(configs))
		for i, blocks := range configs {
			out[i] = sharpeOf(blocks, outSample)
		}
		// relative rank of the selected configuration, in (0, 1)
		sorted := append([]float64(nil), out...)
		sort.Float64s(sorted)
		rank := float64(sort.SearchFloat64s(sorted, out[best])+1) / float64(len(out)+1)
		logit := math.Log(rank / (1 - rank))
		result.Logits = append(result.Logits, logit)
		if logit <= 0 {
			below++
		}
	})

	result.Splits = len(result.Logits)
	if result.Splits > 0 {
		result.PBO = float64(below) / float64(result.Splits)
	}
	return result
}

// combinations calls fn with every sorted subset of size k of 0..n-1.
func combinations(n, k int, fn func([]int)) {
	subset := make([]int, k)
	var walk func(start, depth int)
	walk = func(start, depth int) {
		if depth == k {
			fn(subset)
			return
		}
		for i := start; i <= n-(k-depth); i++ {
			subset[depth] = i
			walk(i+1, depth+1)
		}
	}
	walk(0, 0)
}

func complement(n int, subset []int) []int {
	in := make(map[int]bool, len(subset))
	for _, i := range subset {
		in[i] = true
	}
	out := make([]int, 0, n-len(subset))
	for i := 0; i < n; i++ {
		if !in[i] {
			out = append(out, i)
		}
	}
	return out
}
