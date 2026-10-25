package backtest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

func readSpecFile(t *testing.T, path string) Spec {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()
	spec, err := ReadSpec(file)
	require.NoError(t, err)
	return spec
}

func TestReadSpec_JSONAndYAMLHashEqually(t *testing.T) {
	jsonSpec := readSpecFile(t, "../examples/specs/emacross.json")
	yamlSpec := readSpecFile(t, "../examples/specs/emacross.yaml")
	require.NoError(t, jsonSpec.Validate())
	require.Equal(t, jsonSpec.Hash(), yamlSpec.Hash())
	require.Len(t, jsonSpec.Hash(), 64)

	reordered, err := ParseSpec([]byte(`{"strategy":"emacross","capital":10000,"quote":"USDT",
		"schema_version":1, "name":"emacross on BTC and ETH",
		"fees":{"taker":0.001,"maker":0.001},
		"params":{"slow":21,"timeframe":"4h","fast":8},
		"data":[{"timeframe":"1h","pair":"BTCUSDT","file":"../../testdata/btc-1h.csv"},
		        {"pair":"ETHUSDT","file":"../../testdata/eth-1h.csv","timeframe":"1h"}],
		"pairs":["ETHUSDT","BTCUSDT"]}`))
	require.NoError(t, err)
	require.Equal(t, jsonSpec.Hash(), reordered.Hash(), "explicit pairs equal to the default ones")

	changed := jsonSpec
	changed.Params = map[string]any{"fast": 9}
	require.NotEqual(t, jsonSpec.Hash(), changed.Hash())
}

func TestReadSpec_Errors(t *testing.T) {
	_, err := ParseSpec([]byte(`{"schema_version": 1, "strategi": "emacross"}`))
	require.ErrorIs(t, err, ErrInvalidSpec)
	require.Contains(t, err.Error(), "strategi")

	_, err = ParseSpec([]byte("strategy: [unclosed"))
	require.ErrorIs(t, err, ErrInvalidSpec)
}

func TestSpec_Validate(t *testing.T) {
	err := Spec{
		SchemaVersion: 2,
		Capital:       -1,
		Fees:          Fees{Maker: 1.5},
		Data: []DataSource{
			{Pair: "BTCUSDT", File: "a.csv", Timeframe: "1h"},
			{Pair: "BTCUSDT"},
			{Pair: "BTCBRL", File: "b.csv", Timeframe: "1h"},
		},
		Quote:  "USDT",
		Pairs:  []string{"XRPUSDT"},
		Window: &Window{Start: Date{time.Date(2021, 2, 1, 0, 0, 0, 0, time.UTC)}, End: Date{time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)}},
	}.Validate()
	require.ErrorIs(t, err, ErrInvalidSpec)
	for _, problem := range []string{
		"schema_version must be 1", "strategy is required", "capital must be positive", "fees are fractions",
		"data[1]: pair BTCUSDT is loaded twice", "data[1].file is required", "data[1].timeframe is required",
		"data[2]: pair BTCBRL is not quoted in USDT", "pair XRPUSDT has no data source",
		"window.start must be before window.end",
	} {
		require.Contains(t, err.Error(), problem)
	}
	require.Equal(t, 10, strings.Count(err.Error(), "\n  - "))
}

func TestDate(t *testing.T) {
	var d Date
	require.NoError(t, d.UnmarshalJSON([]byte(`"2021-03-04"`)))
	require.Equal(t, time.Date(2021, 3, 4, 0, 0, 0, 0, time.UTC), d.Time)
	require.NoError(t, d.UnmarshalJSON([]byte(`"2021-03-04T10:00:00-03:00"`)))
	require.Equal(t, time.Date(2021, 3, 4, 13, 0, 0, 0, time.UTC), d.Time)
	require.Error(t, d.UnmarshalJSON([]byte(`"04/03/2021"`)))
	data, err := d.MarshalJSON()
	require.NoError(t, err)
	require.Equal(t, `"2021-03-04T13:00:00Z"`, string(data))
	data, _ = Date{}.MarshalJSON()
	require.Equal(t, "null", string(data))
}

type specStrategy struct{ emaCross }

func TestFromSpec(t *testing.T) {
	registry := strategy.NewRegistry()
	registry.Add(strategy.Definition{Name: "ema", Params: []strategy.ParamSpec{{Name: "timeframe", Type: strategy.TypeString, Default: "1d"}},
		Factory: func(params map[string]any) (strategy.Strategy, error) {
			return &specStrategy{emaCross{timeframe: params["timeframe"].(string)}}, nil
		}})

	spec := readSpecFile(t, "../examples/specs/emacross.json")
	spec.Strategy = "ema"
	spec.Params = nil
	spec.Fees = Fees{}

	ds, err := spec.LoadDataset(filepath.Dir("../examples/specs/emacross.json"))
	require.NoError(t, err)
	engine, err := FromSpec(ds, spec, registry)
	require.NoError(t, err)
	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, "ema", result.Config.Strategy)
	require.Equal(t, map[string]any{"timeframe": "1d"}, result.Config.Params)

	// the golden numbers of the legacy bot
	summary := summarize(result)
	require.InDelta(t, 5340.224, summary["BTCUSDT"][0], 0.001)
	require.InDelta(t, 7590.7381, summary["ETHUSDT"][0], 0.001)

	t.Run("window", func(t *testing.T) {
		windowed := spec
		windowed.Window = &Window{Start: Date{time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)}}
		engine, err := FromSpec(ds, windowed, registry)
		require.NoError(t, err)
		result, err := engine.Run(context.Background())
		require.NoError(t, err)
		require.False(t, result.Period.Start.Before(windowed.Window.Start.Time))
	})

	t.Run("errors", func(t *testing.T) {
		_, err := spec.LoadDataset("/nonexistent")
		require.ErrorIs(t, err, ErrData)

		unknown := spec
		unknown.Strategy = "nope"
		_, err = FromSpec(ds, unknown, registry)
		require.ErrorIs(t, err, strategy.ErrUnknownStrategy)

		invalid := spec
		invalid.Params = map[string]any{"x": 1}
		_, err = FromSpec(ds, invalid, registry)
		require.ErrorIs(t, err, strategy.ErrInvalidParams)

		broken := spec
		broken.Capital = 0
		_, err = FromSpec(ds, broken, registry)
		require.ErrorIs(t, err, ErrInvalidSpec)

		missing := spec
		missing.Data = append(missing.Data, DataSource{Pair: "XRPUSDT", File: "x.csv", Timeframe: "1h"})
		_, err = FromSpec(ds, missing, registry)
		require.True(t, errors.Is(err, ErrData), err)
	})
}

// firstCall records the first candle the strategy trades on.
type firstCall struct {
	emaCross
	first time.Time
}

func (s *firstCall) OnCandle(df *model.Dataframe, _ service.Broker) {
	if s.first.IsZero() {
		s.first = df.Time[len(df.Time)-1]
	}
}

func TestFromSpec_WindowWarmup(t *testing.T) {
	registry := strategy.NewRegistry()
	instance := &firstCall{emaCross: emaCross{timeframe: "1d"}}
	require.NoError(t, registry.Add(strategy.Definition{Name: "first",
		Factory: func(map[string]any) (strategy.Strategy, error) { return instance, nil }}))

	spec := readSpecFile(t, "../examples/specs/emacross.json")
	spec.Strategy, spec.Params = "first", nil
	start := time.Date(2021, 3, 1, 0, 0, 0, 0, time.UTC)
	spec.Window = &Window{Start: Date{start}, End: Date{time.Date(2021, 3, 20, 0, 0, 0, 0, time.UTC)}}

	ds, err := spec.LoadDataset("../examples/specs")
	require.NoError(t, err)
	engine, err := FromSpec(ds, spec, registry)
	require.NoError(t, err)
	result, err := engine.Run(context.Background())
	require.NoError(t, err)

	// the warmup (10 daily candles) ran before the window: the strategy
	// trades from its first day
	require.Equal(t, start, instance.first)
	require.Equal(t, start, result.Period.Start)
	require.False(t, time.Unix(result.Equity[0].Time, 0).Before(start), "the warmup is not measured")
	require.Equal(t, 10000.0, result.Wallet.Initial)
}
