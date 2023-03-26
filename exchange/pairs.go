package exchange

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
)

type AssetQuote struct {
	Quote string
	Asset string
}

var (
	//go:embed pairs.json
	pairs             []byte
	pairAssetQuoteMap = make(map[string]AssetQuote)
)

func init() {
	err := json.Unmarshal(pairs, &pairAssetQuoteMap)
	if err != nil {
		panic(err)
	}
}

func SplitAssetQuote(pair string) (asset string, quote string) {
	data := pairAssetQuoteMap[pair]
	return data.Asset, data.Quote
}

func updateParisFile() error {
	client := binance.NewClient("", "")
	sportInfo, err := client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get exchange info: %v", err)
	}

	futureClient := futures.NewClient("", "")
	futureInfo, err := futureClient.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get exchange info: %v", err)
	}

	for _, info := range sportInfo.Symbols {
		pairAssetQuoteMap[info.Symbol] = AssetQuote{
			Quote: info.QuoteAsset,
			Asset: info.BaseAsset,
		}
	}

	for _, info := range futureInfo.Symbols {
		pairAssetQuoteMap[info.Symbol] = AssetQuote{
			Quote: info.QuoteAsset,
			Asset: info.BaseAsset,
		}
	}

	fmt.Printf("Total pairs: %d\n", len(pairAssetQuoteMap))

	content, err := json.Marshal(pairAssetQuoteMap)
	if err != nil {
		return fmt.Errorf("failed to marshal pairs: %v", err)
	}

	err = os.WriteFile("pairs.json", content, 0644)
	if err != nil {
		return fmt.Errorf("failed to write to file: %v", err)
	}

	return nil
}
