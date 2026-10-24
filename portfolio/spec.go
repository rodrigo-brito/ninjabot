// Package portfolio backtests several strategies together and reports each
// one and the combination: a basket of weakly correlated strategies often has
// a smoother equity curve than any of them alone.
//
// In the isolated mode each strategy trades its own share of the capital in
// its own paper wallet, in parallel; the portfolio equity is the time-aligned
// sum of the wallets plus the unallocated cash.
package portfolio

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/rodrigo-brito/ninjabot/backtest"
)

// SpecVersion is the version of the portfolio spec layout.
const SpecVersion = 1

// Modes of a portfolio.
const (
	// ModeIsolated gives every strategy its own wallet with Weight x Capital.
	ModeIsolated = "isolated"
	// ModeShared runs every strategy on one wallet with the whole capital;
	// each one is attributed Weight x Capital, sees that budget as its cash
	// and its own holdings as its positions.
	ModeShared = "shared"
)

// StrategySpec is one strategy of a portfolio.
type StrategySpec struct {
	// ID names the strategy in the results; unique within the portfolio.
	ID       string         `json:"id"`
	Strategy string         `json:"strategy"`
	Params   map[string]any `json:"params,omitempty"`
	// Pairs to trade; every pair of the data when empty.
	Pairs     []string `json:"pairs,omitempty"`
	Timeframe string   `json:"timeframe,omitempty"`
	// Weight is the fraction of the capital allocated to the strategy.
	Weight float64 `json:"weight"`
}

// Spec describes a portfolio backtest.
type Spec struct {
	SchemaVersion int                   `json:"schema_version"`
	Name          string                `json:"name,omitempty"`
	Data          []backtest.DataSource `json:"data"`
	Quote         string                `json:"quote"`
	Capital       float64               `json:"capital"`
	Fees          backtest.Fees         `json:"fees"`
	Mode          string                `json:"mode,omitempty"` // isolated (default) or shared
	Window        *backtest.Window      `json:"window,omitempty"`
	Strategies    []StrategySpec        `json:"strategies"`
}

// ReadSpec decodes a portfolio spec from JSON or YAML, rejecting unknown
// fields.
func ReadSpec(r io.Reader) (Spec, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Spec{}, err
	}
	var spec Spec
	if err := backtest.DecodeStrict(data, &spec); err != nil {
		return Spec{}, err
	}
	return spec, nil
}

// Validate reports every problem of the spec at once.
func (s Spec) Validate() error {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	if s.SchemaVersion != SpecVersion {
		add("schema_version must be %d, got %d", SpecVersion, s.SchemaVersion)
	}
	if s.Mode != "" && s.Mode != ModeIsolated && s.Mode != ModeShared {
		add("mode %q is not supported, use %q or %q", s.Mode, ModeIsolated, ModeShared)
	}
	if len(s.Strategies) == 0 {
		add("strategies needs at least one entry")
	}

	ids := make(map[string]bool, len(s.Strategies))
	total := 0.0
	for i, strategy := range s.Strategies {
		switch {
		case strategy.ID == "":
			add("strategies[%d].id is required", i)
		case ids[strategy.ID]:
			add("strategies[%d]: id %q is used twice", i, strategy.ID)
		}
		ids[strategy.ID] = true
		if strategy.Weight <= 0 {
			add("strategies[%d].weight must be positive", i)
		}
		total += strategy.Weight
		if err := s.run(i).Validate(); err != nil {
			for _, line := range strings.Split(err.Error(), "\n  - ")[1:] {
				add("strategies[%d] (%s): %s", i, strategy.ID, line)
			}
		}
	}
	if total > 1+1e-9 {
		add("the weights add up to %.4g, more than the whole capital", total)
	}

	if len(problems) > 0 {
		return fmt.Errorf("%w:\n  - %s", backtest.ErrInvalidSpec, strings.Join(dedupe(problems), "\n  - "))
	}
	return nil
}

func dedupe(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := values[:0]
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

// run is the backtest spec of the i-th strategy, with its share of the
// capital.
func (s Spec) run(i int) backtest.Spec {
	strategy := s.Strategies[i]
	return backtest.Spec{
		SchemaVersion: backtest.SpecVersion,
		Name:          strategy.ID,
		Data:          s.Data,
		Quote:         s.Quote,
		Capital:       s.Capital * strategy.Weight,
		Fees:          s.Fees,
		Window:        s.Window,
		Strategy:      strategy.Strategy,
		Params:        strategy.Params,
		Pairs:         strategy.Pairs,
		Timeframe:     strategy.Timeframe,
	}
}

// Runs returns the backtest spec of every strategy, in order.
func (s Spec) Runs() []backtest.Spec {
	runs := make([]backtest.Spec, len(s.Strategies))
	for i := range s.Strategies {
		runs[i] = s.run(i).Normalize()
	}
	return runs
}

// Unallocated is the capital no strategy receives.
func (s Spec) Unallocated() float64 {
	total := 0.0
	for _, strategy := range s.Strategies {
		total += strategy.Weight
	}
	return math.Max(0, s.Capital*(1-total))
}

// Normalize makes the defaults explicit.
func (s Spec) Normalize() Spec {
	if s.Mode == "" {
		s.Mode = ModeIsolated
	}
	strategies := make([]StrategySpec, len(s.Strategies))
	for i, strategy := range s.Strategies {
		strategy.Pairs = s.run(i).Normalize().Pairs
		strategies[i] = strategy
	}
	s.Strategies = strategies
	return s
}

// Hash identifies the portfolio: the sha256 of its normalized JSON.
func (s Spec) Hash() string {
	data, err := json.Marshal(s.Normalize())
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// IDs returns the strategy IDs, in spec order.
func (s Spec) IDs() []string {
	ids := make([]string, len(s.Strategies))
	for i, strategy := range s.Strategies {
		ids[i] = strategy.ID
	}
	return ids
}
