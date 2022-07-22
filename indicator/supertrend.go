package indicator

import "github.com/markcheno/go-talib"

func SuperTrend(high, low, close []float64, atrPeriod int, factor float64) []float64 {
	atr := talib.Atr(high, low, close, atrPeriod)
	basicUpperBand := make([]float64, len(atr))
	basicLowerBand := make([]float64, len(atr))
	finalUpperBand := make([]float64, len(atr))
	finalLowerBand := make([]float64, len(atr))
	superTrend := make([]float64, len(atr))

	for i := 1; i < len(basicLowerBand); i++ {
		basicUpperBand[i] = (high[i]+low[i])/2.0 + atr[i]*factor
		basicLowerBand[i] = (high[i]+low[i])/2.0 - atr[i]*factor

		if basicUpperBand[i] < finalUpperBand[i-1] ||
			close[i-1] > finalUpperBand[i-1] {
			finalUpperBand[i] = basicUpperBand[i]
		} else {
			finalUpperBand[i] = finalUpperBand[i-1]
		}

		if basicLowerBand[i] > finalLowerBand[i-1] ||
			close[i-1] < finalLowerBand[i-1] {
			finalLowerBand[i] = basicLowerBand[i]
		} else {
			finalLowerBand[i] = finalLowerBand[i-1]
		}

		if finalUpperBand[i-1] == superTrend[i-1] {
			if close[i] > finalUpperBand[i] {
				superTrend[i] = finalLowerBand[i]
			} else {
				superTrend[i] = finalUpperBand[i]
			}
		} else {
			if close[i] < finalLowerBand[i] {
				superTrend[i] = finalUpperBand[i]
			} else {
				superTrend[i] = finalLowerBand[i]
			}
		}
	}

	return superTrend
}
