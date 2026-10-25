package strategy

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
)

type crossParams struct {
	Timeframe string  `json:"timeframe" default:"4h" enum:"1h,4h,1d" description:"candle timeframe"`
	Fast      int     `json:"fast" default:"8" min:"2" description:"fast period"`
	Slow      int     `json:"slow" default:"21" min:"2" max:"500"`
	Size      float64 `json:"size" default:"0.5" min:"0" max:"1"`
	Short     bool    `json:"short"`
}

type paramStrategy struct{ p crossParams }

func (s paramStrategy) Timeframe() string                            { return s.p.Timeframe }
func (s paramStrategy) WarmupPeriod() int                            { return s.p.Slow }
func (s paramStrategy) Indicators(*model.Dataframe) []ChartIndicator { return nil }
func (s paramStrategy) OnCandle(*model.Dataframe, service.Broker)    {}

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	registry := NewRegistry()
	require.NoError(t, registry.Add(DefinitionOf("cross", "trades crosses", func(p crossParams) (Strategy, error) {
		if p.Fast >= p.Slow {
			return nil, fmt.Errorf("fast (%d) must be below slow (%d)", p.Fast, p.Slow)
		}
		return paramStrategy{p}, nil
	})))
	return registry
}

func TestParamsOf(t *testing.T) {
	specs := ParamsOf(crossParams{})
	require.Len(t, specs, 5)
	require.Equal(t, ParamSpec{Name: "timeframe", Type: TypeString, Description: "candle timeframe",
		Default: "4h", Enum: []string{"1h", "4h", "1d"}}, specs[0])
	require.Equal(t, TypeInt, specs[1].Type)
	require.Equal(t, 8, specs[1].Default)
	require.Equal(t, 2.0, *specs[1].Min)
	require.Equal(t, 500.0, *specs[2].Max)
	require.Equal(t, TypeFloat, specs[3].Type)
	require.Equal(t, 0.5, specs[3].Default)
	require.Equal(t, ParamSpec{Name: "short", Type: TypeBool}, specs[4])

	require.Panics(t, func() { ParamsOf(1) })
	require.Panics(t, func() {
		ParamsOf(struct {
			A []int `json:"a"`
		}{})
	})
	require.Panics(t, func() {
		ParamsOf(struct {
			A int `json:"a" default:"x"`
		}{})
	})
}

func TestRegistry_New(t *testing.T) {
	registry := newTestRegistry(t)

	s, err := registry.New("cross", nil)
	require.NoError(t, err)
	require.Equal(t, crossParams{Timeframe: "4h", Fast: 8, Slow: 21, Size: 0.5}, s.(paramStrategy).p)

	// JSON numbers arrive as float64
	var params map[string]any
	require.NoError(t, json.Unmarshal([]byte(`{"fast": 5, "slow": 34, "timeframe": "1h", "short": true}`), &params))
	s, err = registry.New("cross", params)
	require.NoError(t, err)
	require.Equal(t, crossParams{Timeframe: "1h", Fast: 5, Slow: 34, Size: 0.5, Short: true}, s.(paramStrategy).p)

	other, err := registry.New("cross", params)
	require.NoError(t, err)
	require.NotSame(t, &s, &other)

	for name, tc := range map[string]struct {
		strategy string
		params   map[string]any
		target   error
		message  string
	}{
		"unknown strategy": {"nope", nil, ErrUnknownStrategy, `"nope" (known: cross)`},
		"unknown param":    {"cross", map[string]any{"fats": 3}, ErrInvalidParams, `no param "fats"`},
		"not an integer":   {"cross", map[string]any{"fast": 3.5}, ErrInvalidParams, "must be an integer"},
		"below min":        {"cross", map[string]any{"fast": 1}, ErrInvalidParams, "must be >= 2"},
		"above max":        {"cross", map[string]any{"slow": 900}, ErrInvalidParams, "must be <= 500"},
		"enum":             {"cross", map[string]any{"timeframe": "2h"}, ErrInvalidParams, "must be one of"},
		"wrong type":       {"cross", map[string]any{"timeframe": 4}, ErrInvalidParams, "must be a string"},
		"bool":             {"cross", map[string]any{"short": "yes"}, ErrInvalidParams, "must be a boolean"},
		"number":           {"cross", map[string]any{"size": "big"}, ErrInvalidParams, "must be a number"},
		"factory error":    {"cross", map[string]any{"fast": 30}, ErrInvalidParams, "fast (30) must be below slow (21)"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := registry.New(tc.strategy, tc.params)
			require.True(t, errors.Is(err, tc.target), err)
			require.Contains(t, err.Error(), tc.message)
		})
	}
}

func TestRegistry_AddErrors(t *testing.T) {
	registry := newTestRegistry(t)
	factory := func(map[string]any) (Strategy, error) { return paramStrategy{}, nil }
	require.Error(t, registry.Add(Definition{Name: "cross", Factory: factory}), "duplicated")
	require.Error(t, registry.Add(Definition{Name: "x"}), "no factory")
	require.Error(t, registry.Add(Definition{Name: "y", Factory: factory,
		Params: []ParamSpec{{Name: "a", Type: TypeInt}, {Name: "a", Type: TypeInt}}}))
	require.Error(t, registry.Add(Definition{Name: "z", Factory: factory, Params: []ParamSpec{{Name: "a", Type: "date"}}}))
}

func TestRegistry_DescribeAndSchema(t *testing.T) {
	registry := newTestRegistry(t)
	require.NoError(t, registry.Add(Definition{Name: "alpha", Factory: func(map[string]any) (Strategy, error) {
		return paramStrategy{}, nil
	}}))

	definitions := registry.Describe()
	require.Len(t, definitions, 2)
	require.Equal(t, "alpha", definitions[0].Name)

	data, err := json.Marshal(definitions[1])
	require.NoError(t, err)
	require.Contains(t, string(data), `"name":"cross"`)

	schema := definitions[1].JSONSchema()
	require.Equal(t, "https://json-schema.org/draft/2020-12/schema", schema["$schema"])
	require.Equal(t, "object", schema["type"])
	require.Equal(t, false, schema["additionalProperties"])
	properties := schema["properties"].(map[string]any)
	require.Equal(t, map[string]any{"type": "integer", "default": 8, "minimum": 2.0, "description": "fast period"},
		properties["fast"])
	require.Equal(t, []string{"1h", "4h", "1d"}, properties["timeframe"].(map[string]any)["enum"])
	require.Equal(t, "boolean", properties["short"].(map[string]any)["type"])
	require.Equal(t, "number", properties["size"].(map[string]any)["type"])
	_, err = json.Marshal(schema)
	require.NoError(t, err)
}

func TestDecodeParams(t *testing.T) {
	var p crossParams
	require.NoError(t, DecodeParams(map[string]any{"fast": 3}, &p))
	require.Equal(t, crossParams{Timeframe: "4h", Fast: 3, Slow: 21, Size: 0.5}, p)

	require.ErrorIs(t, DecodeParams(map[string]any{"unknown": 1}, &p), ErrInvalidParams)
	require.Error(t, DecodeParams(nil, p), "not a pointer")
}

func TestRegistry_Concurrent(t *testing.T) {
	registry := NewRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("s%d", i)
			require.NoError(t, registry.Add(DefinitionOf(name, "", func(p crossParams) (Strategy, error) {
				return paramStrategy{p}, nil
			})))
			_, err := registry.New(name, map[string]any{"fast": 3})
			require.NoError(t, err)
			_ = registry.Describe()
		}()
	}
	wg.Wait()
	require.Len(t, registry.Describe(), 8)
}

func TestRegister_Default(t *testing.T) {
	name := "test-default-registry"
	RegisterParams(name, "", func(p crossParams) (Strategy, error) { return paramStrategy{p}, nil })
	require.Panics(t, func() { Register(name, func(map[string]any) (Strategy, error) { return nil, nil }) })
	_, err := Default.New(name, nil)
	require.NoError(t, err)
}

func TestWithDefaults(t *testing.T) {
	require.Equal(t, crossParams{Timeframe: "4h", Fast: 3, Slow: 21, Size: 0.5}, WithDefaults(crossParams{Fast: 3}))
	require.Equal(t, crossParams{Timeframe: "1d", Fast: 8, Slow: 21, Size: 0.5}, WithDefaults(crossParams{Timeframe: "1d"}))
}
