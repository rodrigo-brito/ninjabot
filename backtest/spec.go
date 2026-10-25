package backtest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	str2duration "github.com/xhit/go-str2duration/v2"
	"gopkg.in/yaml.v3"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

// SpecVersion is the version of the run spec layout.
const SpecVersion = 1

// Errors that classify why a spec cannot run. The CLI maps them to exit
// codes.
var (
	ErrInvalidSpec = errors.New("backtest: invalid spec")
	ErrData        = errors.New("backtest: data error")
)

// DataSource is a CSV file of candles of one pair.
type DataSource struct {
	Pair       string `json:"pair"`
	File       string `json:"file"`
	Timeframe  string `json:"timeframe"`
	HeikinAshi bool   `json:"heikin_ashi,omitempty"`
}

// Fees are charged on every fill, as a fraction of the traded volume.
type Fees struct {
	Maker float64 `json:"maker"`
	Taker float64 `json:"taker"`
}

// Date is a time that reads either RFC 3339 or a plain date (2006-01-02, UTC).
type Date struct{ time.Time }

func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.UTC().Format(time.RFC3339))
}

func (d *Date) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw == "" {
		d.Time = time.Time{}
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			d.Time = t.UTC()
			return nil
		}
	}
	return fmt.Errorf("invalid date %q: use 2006-01-02 or RFC 3339", raw)
}

// Window restricts a run to the candles within [start, end]. Either side
// may be omitted.
type Window struct {
	Start Date `json:"start"`
	End   Date `json:"end"`
}

// Spec describes a backtest run in a file: data, wallet, strategy and its
// parameters. Specs are JSON (canonical) or YAML.
type Spec struct {
	SchemaVersion int            `json:"schema_version"`
	Name          string         `json:"name,omitempty"`
	Data          []DataSource   `json:"data"`
	Quote         string         `json:"quote"`
	Capital       float64        `json:"capital"`
	Fees          Fees           `json:"fees"`
	Window        *Window        `json:"window,omitempty"`
	Strategy      string         `json:"strategy"`
	Params        map[string]any `json:"params,omitempty"`
	// Pairs to trade; every pair of Data when empty.
	Pairs []string `json:"pairs,omitempty"`
	// Timeframe overrides the timeframe of the strategy.
	Timeframe string `json:"timeframe,omitempty"`
	// Seed of the randomized statistics of the result (bootstrap, Monte
	// Carlo, permutation tests); 1 when omitted.
	Seed int64 `json:"seed,omitempty"`
}

// ReadSpec decodes a spec from JSON or YAML. Unknown fields are an error, so
// a typo never silently falls back to a default.
func ReadSpec(r io.Reader) (Spec, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Spec{}, err
	}
	return ParseSpec(data)
}

// ParseSpec is ReadSpec over bytes.
func ParseSpec(data []byte) (Spec, error) {
	var spec Spec
	if err := DecodeStrict(data, &spec); err != nil {
		return Spec{}, err
	}
	return spec, nil
}

// DecodeStrict decodes a JSON or YAML document into v, rejecting unknown
// fields. Errors wrap ErrInvalidSpec.
func DecodeStrict(data []byte, v any) error {
	data, err := toJSON(data)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidSpec, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidSpec, err)
	}
	return nil
}

// toJSON converts a YAML document to JSON; JSON documents pass through.
func toJSON(data []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		return trimmed, nil
	}
	var document any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	return json.Marshal(document)
}

// Validate reports every problem of the spec at once, with what to change.
func (s Spec) Validate() error {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	if s.SchemaVersion != SpecVersion {
		add("schema_version must be %d, got %d", SpecVersion, s.SchemaVersion)
	}
	if s.Strategy == "" {
		add("strategy is required: run `ninjabot strategies` to list them")
	}
	if s.Quote == "" {
		add(`quote is required, e.g. "USDT"`)
	}
	if s.Capital <= 0 {
		add("capital must be positive, got %v", s.Capital)
	}
	if s.Fees.Maker < 0 || s.Fees.Maker >= 1 || s.Fees.Taker < 0 || s.Fees.Taker >= 1 {
		add("fees are fractions in [0, 1), e.g. 0.001 for 0.1%%")
	}
	if len(s.Data) == 0 {
		add("data needs at least one source: {pair, file, timeframe}")
	}

	loaded := make(map[string]bool, len(s.Data))
	for i, source := range s.Data {
		switch {
		case source.Pair == "":
			add("data[%d].pair is required", i)
		case loaded[source.Pair]:
			add("data[%d]: pair %s is loaded twice", i, source.Pair)
		}
		loaded[source.Pair] = true
		if source.File == "" {
			add("data[%d].file is required", i)
		}
		if source.Timeframe == "" {
			add("data[%d].timeframe is required, e.g. \"1h\"", i)
		}
		if asset, quote := exchange.SplitAssetQuote(source.Pair); source.Pair != "" && (asset == "" || quote == "") {
			add("data[%d]: unknown pair %s", i, source.Pair)
		} else if source.Pair != "" && s.Quote != "" && quote != s.Quote {
			add("data[%d]: pair %s is not quoted in %s", i, source.Pair, s.Quote)
		}
	}
	for _, pair := range s.Pairs {
		if !loaded[pair] {
			add("pair %s has no data source", pair)
		}
	}
	if s.Window != nil && !s.Window.Start.IsZero() && !s.Window.End.IsZero() && !s.Window.Start.Before(s.Window.End.Time) {
		add("window.start must be before window.end")
	}

	if len(problems) > 0 {
		return fmt.Errorf("%w:\n  - %s", ErrInvalidSpec, strings.Join(problems, "\n  - "))
	}
	return nil
}

// Normalize returns the spec with its defaults made explicit: the pairs to
// trade, sorted.
func (s Spec) Normalize() Spec {
	if len(s.Pairs) == 0 {
		for _, source := range s.Data {
			s.Pairs = append(s.Pairs, source.Pair)
		}
	}
	s.Pairs = sortedPairs(s.Pairs)
	return s
}

// Resolve returns the canonical form of the spec for a registry: the
// normalized spec with every strategy parameter explicit (defaults included).
// When the strategy has a "timeframe" parameter, the timeframe lives there:
// a top-level timeframe overrides it and is cleared. Two specs that describe
// the same run resolve, and hash, identically.
func (s Spec) Resolve(registry *strategy.Registry) (Spec, error) {
	s = s.Normalize()
	def, err := registry.Lookup(s.Strategy)
	if err != nil {
		return Spec{}, err
	}
	params := make(map[string]any, len(s.Params)+1)
	for key, value := range s.Params {
		params[key] = value
	}
	if s.Timeframe != "" && hasParam(def, "timeframe") {
		params["timeframe"] = s.Timeframe
		s.Timeframe = ""
	}
	resolved, err := def.Resolve(params)
	if err != nil {
		return Spec{}, err
	}
	// round-trip through JSON so numbers have one representation
	data, err := json.Marshal(resolved)
	if err != nil {
		return Spec{}, err
	}
	s.Params = nil
	if err := json.Unmarshal(data, &s.Params); err != nil {
		return Spec{}, err
	}
	return s, nil
}

func hasParam(def strategy.Definition, name string) bool {
	for _, param := range def.Params {
		if param.Name == name {
			return true
		}
	}
	return false
}

// Hash identifies the run a spec describes: the sha256 of its normalized
// JSON. Key order, whitespace and the file format do not change it.
func (s Spec) Hash() string {
	data, err := json.Marshal(s.Normalize())
	if err != nil {
		panic(err) // a spec decoded from JSON always encodes
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// LoadDataset loads the data sources of the spec. Relative files are
// resolved against dir, usually the directory of the spec file.
func (s Spec) LoadDataset(dir string) (*Dataset, error) {
	feeds := make([]exchange.PairFeed, len(s.Data))
	for i, source := range s.Data {
		file := source.File
		if dir != "" && !filepath.IsAbs(file) {
			file = filepath.Join(dir, file)
		}
		feeds[i] = exchange.PairFeed{Pair: source.Pair, File: file, Timeframe: source.Timeframe, HeikinAshi: source.HeikinAshi}
	}
	ds, err := LoadDataset(feeds...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrData, err)
	}
	return ds, nil
}

// Config builds the engine configuration of the spec, creating the strategy
// from the registry.
func (s Spec) Config(registry *strategy.Registry) (Config, error) {
	if err := s.Validate(); err != nil {
		return Config{}, err
	}
	s, err := s.Resolve(registry)
	if err != nil {
		return Config{}, err
	}
	instance, err := registry.New(s.Strategy, s.Params)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Pairs:     s.Pairs,
		Strategy:  instance,
		Timeframe: s.Timeframe,
		Quote:     s.Quote,
		Balances:  map[string]float64{s.Quote: s.Capital},
		MakerFee:  s.Fees.Maker,
		TakerFee:  s.Fees.Taker,
		Seed:      s.Seed,
	}, nil
}

// FromSpec prepares the run of a spec over a dataset loaded with
// Spec.LoadDataset. Options adjust the configuration before the engine is
// built, e.g. to skip the statistics.
func FromSpec(ds *Dataset, s Spec, registry *strategy.Registry, options ...func(*Config)) (*Engine, error) {
	cfg, err := s.Config(registry)
	if err != nil {
		return nil, err
	}
	for _, option := range options {
		option(&cfg)
	}
	if s.Window != nil {
		start := s.Window.Start.Time
		if !start.IsZero() {
			// load the warmup of the strategy before the window, so its
			// indicators are ready when the window starts
			cfg.TradeFrom = start
			timeframe := cfg.Timeframe
			if timeframe == "" {
				timeframe = cfg.Strategy.Timeframe()
			}
			if step, err := str2duration.ParseDuration(timeframe); err == nil {
				start = start.Add(-time.Duration(cfg.Strategy.WarmupPeriod()+1) * step)
			}
		}
		ds = ds.Window(start, s.Window.End.Time)
	}
	engine, err := New(ds, cfg)
	if err != nil {
		if errors.Is(err, ErrUnknownPair) || errors.Is(err, exchange.ErrInsufficientData) {
			return nil, fmt.Errorf("%w: %w", ErrData, err)
		}
		return nil, err
	}
	engine.c.Config.Strategy = s.Strategy
	if resolved, err := s.Resolve(registry); err == nil {
		engine.c.Config.Params = resolved.Params
	}
	return engine, nil
}
