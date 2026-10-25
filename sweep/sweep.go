// Package sweep runs a family of backtests that differ in their parameters
// (a grid or a seeded random sample), ranks them by a metric and caches
// every run by the hash of its spec, so an interrupted or extended sweep only
// runs what is missing.
package sweep

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/rodrigo-brito/ninjabot/backtest"
	"github.com/rodrigo-brito/ninjabot/strategy"
	"github.com/rodrigo-brito/ninjabot/tools/metrics"
)

// Files written to the output directory.
const (
	RunsFile    = "runs.jsonl"
	SummaryFile = "summary.json"
	RunsDir     = "runs"
)

// Range is the domain of a parameter in a random sweep: either a list of
// values, or a numeric interval (integers when Integer is set).
type Range struct {
	Min     float64 `json:"min,omitempty"`
	Max     float64 `json:"max,omitempty"`
	Integer bool    `json:"integer,omitempty"`
	Values  []any   `json:"values,omitempty"`
}

// Random draws Samples parameter sets from the ranges, with a fixed seed.
type Random struct {
	Samples int              `json:"samples"`
	Seed    int64            `json:"seed"`
	Ranges  map[string]Range `json:"ranges"`
}

// Spec is a backtest spec plus the parameters to vary. Keys name either a
// strategy parameter (params.fast) or a field of the spec (timeframe).
type Spec struct {
	backtest.Spec
	// Grid runs the cartesian product of the values.
	Grid map[string][]any `json:"grid,omitempty"`
	// Random runs a seeded sample of the ranges.
	Random *Random `json:"random,omitempty"`
}

// ReadSpec decodes a sweep spec from JSON or YAML, rejecting unknown fields.
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

// Validate checks the base spec and the variations.
func (s Spec) Validate() error {
	if (s.Grid == nil) == (s.Random == nil) {
		return fmt.Errorf("%w: a sweep needs either grid or random", backtest.ErrInvalidSpec)
	}
	keys := make([]string, 0)
	for key, values := range s.Grid {
		if len(values) == 0 {
			return fmt.Errorf("%w: grid %s has no values", backtest.ErrInvalidSpec, key)
		}
		keys = append(keys, key)
	}
	if s.Random != nil {
		if s.Random.Samples <= 0 {
			return fmt.Errorf("%w: random.samples must be positive", backtest.ErrInvalidSpec)
		}
		for key, r := range s.Random.Ranges {
			if len(r.Values) == 0 && r.Max < r.Min {
				return fmt.Errorf("%w: random range %s has max below min", backtest.ErrInvalidSpec, key)
			}
			keys = append(keys, key)
		}
	}
	for _, key := range keys {
		if key != "timeframe" && !strings.HasPrefix(key, "params.") {
			return fmt.Errorf("%w: cannot vary %q: use params.<name> or timeframe", backtest.ErrInvalidSpec, key)
		}
	}
	return s.Spec.Validate()
}

// Expand returns the backtest spec of every run, in a deterministic order.
func (s Spec) Expand() ([]backtest.Spec, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	var variations []map[string]any
	if s.Grid != nil {
		keys := sortedKeys(s.Grid)
		variations = []map[string]any{{}}
		for _, key := range keys {
			next := make([]map[string]any, 0, len(variations)*len(s.Grid[key]))
			for _, base := range variations {
				for _, value := range s.Grid[key] {
					variation := clone(base)
					variation[key] = value
					next = append(next, variation)
				}
			}
			variations = next
		}
	} else {
		random := rand.New(rand.NewSource(s.Random.Seed)) //nolint:gosec // reproducible sampling
		keys := sortedKeys(s.Random.Ranges)
		for i := 0; i < s.Random.Samples; i++ {
			variation := make(map[string]any, len(keys))
			for _, key := range keys {
				r := s.Random.Ranges[key]
				switch {
				case len(r.Values) > 0:
					variation[key] = r.Values[random.Intn(len(r.Values))]
				case r.Integer:
					variation[key] = math.Floor(r.Min) + float64(random.Intn(int(math.Floor(r.Max)-math.Floor(r.Min))+1))
				default:
					variation[key] = r.Min + random.Float64()*(r.Max-r.Min)
				}
			}
			variations = append(variations, variation)
		}
	}

	specs := make([]backtest.Spec, 0, len(variations))
	seen := make(map[string]bool, len(variations))
	for _, variation := range variations {
		run := s.Spec
		run.Params = clone(s.Params)
		for key, value := range variation {
			if key == "timeframe" {
				timeframe, ok := value.(string)
				if !ok {
					return nil, fmt.Errorf("%w: timeframe must be a string, got %v", backtest.ErrInvalidSpec, value)
				}
				run.Timeframe = timeframe
				continue
			}
			run.Params[strings.TrimPrefix(key, "params.")] = value
		}
		run = run.Normalize()
		if hash := run.Hash(); !seen[hash] { // a random sample may draw a set twice
			seen[hash] = true
			specs = append(specs, run)
		}
	}
	return specs, nil
}

// hashOf is the hash of the resolved run, so a run is cached under the same
// hash whether its parameters were given or defaulted.
func hashOf(run backtest.Spec, registry *strategy.Registry) string {
	if resolved, err := run.Resolve(registry); err == nil {
		return resolved.Hash()
	}
	return run.Hash()
}

// normalizeParams gives params the types they have once read back from
// runs.jsonl, so cached and fresh records compare equal.
func normalizeParams(params map[string]any) map[string]any {
	data, err := json.Marshal(params)
	if err != nil {
		return params
	}
	var normalized map[string]any
	if err := json.Unmarshal(data, &normalized); err != nil {
		return params
	}
	return normalized
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func clone(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for key, value := range m {
		out[key] = value
	}
	return out
}

// Record is one line of runs.jsonl.
type Record struct {
	Hash      string           `json:"hash"`
	Params    map[string]any   `json:"params"`
	Timeframe string           `json:"timeframe,omitempty"`
	Metrics   backtest.Metrics `json:"metrics"`
	// Blocks summarize the bar returns in equal time blocks, for the
	// overfitting estimate of the sweep.
	Blocks []metrics.Block `json:"blocks,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// blockCount is the number of time blocks of the overfitting estimate:
// C(16, 8) = 12 870 splits.
const blockCount = 16

// Summary is the ranking of a sweep.
type Summary struct {
	Metric string `json:"metric"`
	Runs   int    `json:"runs"`
	Failed int    `json:"failed"`
	Cached int    `json:"cached"`
	// Overfitting is the probability of backtest overfitting of the family
	// (CSCV over 16 time blocks): how often the best run in one half of the
	// time ranks below the median in the other half.
	Overfitting *metrics.PBOResult `json:"overfitting,omitempty"`
	Ranking     []Record           `json:"ranking"`
}

// Options tune a sweep.
type Options struct {
	// Out is the output directory; required.
	Out string
	// Parallel bounds the runs at once; runtime.NumCPU() when zero.
	Parallel int
	// Metric ranks the runs; sharpe_annual by default.
	Metric string
	// Top bounds the ranking in summary.json; every run when zero.
	Top int
	// Full writes the artifacts of every run to runs/<hash>/, with the
	// robustness reports.
	Full bool
}

// Run executes the sweep: runs whose hash is already in runs.jsonl are
// reused, the others run in parallel over one dataset. It returns the
// summary also written to summary.json.
func Run(ctx context.Context, spec Spec, dir string, registry *strategy.Registry, options Options) (*Summary, error) {
	if options.Out == "" {
		return nil, errors.New("sweep: an output directory is required")
	}
	if options.Metric == "" {
		options.Metric = "sharpe_annual"
	}
	higherIsBetter, ok := backtest.MetricNames[options.Metric]
	if !ok {
		return nil, fmt.Errorf("%w: unknown metric %q", backtest.ErrInvalidSpec, options.Metric)
	}
	if options.Parallel <= 0 {
		options.Parallel = runtime.NumCPU()
	}

	specs, err := spec.Expand()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(options.Out, 0o755); err != nil {
		return nil, err
	}

	cache, err := readRecords(filepath.Join(options.Out, RunsFile))
	if err != nil {
		return nil, err
	}

	ds, err := spec.LoadDataset(dir)
	if err != nil {
		return nil, err
	}

	records := make([]Record, len(specs))
	cached := 0
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(options.Parallel)
	for i, run := range specs {
		hash := hashOf(run, registry)
		if record, ok := cache[hash]; ok && record.Error == "" {
			records[i] = record
			cached++
			continue
		}
		group.Go(func() error {
			records[i] = execute(groupCtx, ds, run, hash, registry, options)
			return groupCtx.Err()
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	if err := writeRecords(filepath.Join(options.Out, RunsFile), records); err != nil {
		return nil, err
	}

	summary := rank(records, options.Metric, higherIsBetter, options.Top)
	summary.Cached = cached
	file, err := os.Create(filepath.Join(options.Out, SummaryFile))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		return nil, err
	}
	return summary, nil
}

// execute runs one spec; a failure is recorded, not returned, so one bad
// parameter set does not stop the sweep.
func execute(ctx context.Context, ds *backtest.Dataset, run backtest.Spec, hash string,
	registry *strategy.Registry, options Options) Record {
	record := Record{Hash: hash, Params: run.Params, Timeframe: run.Timeframe}
	engine, err := backtest.FromSpec(ds, run, registry, func(cfg *backtest.Config) {
		cfg.NoStatistics = !options.Full
	})
	if err != nil {
		record.Error = err.Error()
		return record
	}
	result, err := engine.Run(ctx)
	if err != nil {
		record.Error = err.Error()
		return record
	}
	record.Params = normalizeParams(result.Config.Params)
	record.Metrics = result.Metrics
	record.Blocks = metrics.Blocks(barReturns(result.Equity), blockCount)

	if options.Full {
		if err := writeRun(filepath.Join(options.Out, RunsDir, hash), run, result); err != nil {
			record.Error = err.Error()
		}
	}
	return record
}

func writeRun(dir string, run backtest.Spec, result *backtest.Result) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, write := range map[string]func(io.Writer) error{
		"result.json": result.WriteJSON,
		"trades.csv":  result.WriteTradesCSV,
		"equity.csv":  result.WriteEquityCSV,
		"spec.json": func(w io.Writer) error {
			encoder := json.NewEncoder(w)
			encoder.SetIndent("", "  ")
			return encoder.Encode(run)
		},
	} {
		file, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if err := write(file); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

func barReturns(equity []backtest.Point) []float64 {
	out := make([]float64, 0, len(equity))
	for i := 1; i < len(equity); i++ {
		if previous := equity[i-1].Value; previous > 0 {
			out = append(out, (equity[i].Value-previous)/previous)
		}
	}
	return out
}

func overfitting(records []Record) *metrics.PBOResult {
	configs := make([][]metrics.Block, 0, len(records))
	for _, record := range records {
		if record.Error == "" && len(record.Blocks) == blockCount {
			configs = append(configs, record.Blocks)
		}
	}
	if len(configs) < 2 {
		return nil
	}
	result := metrics.PBO(configs)
	result.Logits = nil
	return &result
}

func rank(records []Record, metric string, higherIsBetter bool, top int) *Summary {
	summary := &Summary{Metric: metric, Runs: len(records), Overfitting: overfitting(records)}
	ranked := make([]Record, 0, len(records))
	for _, record := range records {
		if record.Error != "" {
			summary.Failed++
			continue
		}
		ranked = append(ranked, record)
	}
	value := func(r Record) float64 {
		v, _ := r.Metrics.Value(metric)
		return v
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := value(ranked[i]), value(ranked[j])
		if a != b {
			if higherIsBetter {
				return a > b
			}
			return a < b
		}
		return ranked[i].Hash < ranked[j].Hash
	})
	if top > 0 && len(ranked) > top {
		ranked = ranked[:top]
	}
	summary.Ranking = ranked
	return summary
}

func readRecords(path string) (map[string]Record, error) {
	records := make(map[string]Record)
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return records, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		var record Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, fmt.Errorf("sweep: corrupt %s: %w", path, err)
		}
		records[record.Hash] = record
	}
	return records, scanner.Err()
}

// writeRecords keeps the records of previous sweeps that are not part of
// this one, then writes this sweep in expansion order.
func writeRecords(path string, records []Record) error {
	previous, err := readRecords(path)
	if err != nil {
		return err
	}
	current := make(map[string]bool, len(records))
	for _, record := range records {
		current[record.Hash] = true
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	encoder := json.NewEncoder(writer)
	for _, hash := range sortedKeys(previous) {
		if !current[hash] {
			if err := encoder.Encode(previous[hash]); err != nil {
				return err
			}
		}
	}
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			return err
		}
	}
	return writer.Flush()
}
