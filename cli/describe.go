package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/urfave/cli/v2"

	"github.com/rodrigo-brito/ninjabot/backtest"
	"github.com/rodrigo-brito/ninjabot/portfolio"
	"github.com/rodrigo-brito/ninjabot/strategy"
	"github.com/rodrigo-brito/ninjabot/sweep"
	"github.com/rodrigo-brito/ninjabot/tools/jsonschema"
)

func writeJSON(c *cli.Context, value any) error {
	encoder := json.NewEncoder(c.App.Writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

// strategyDescription is the JSON form of a registered strategy.
type strategyDescription struct {
	strategy.Definition
	Schema map[string]any `json:"schema"`
}

func strategiesCommand(registry *strategy.Registry) *cli.Command {
	return &cli.Command{
		Name:  "strategies",
		Usage: "List the registered strategies and their parameters",
		Flags: []cli.Flag{&cli.BoolFlag{Name: "json", Usage: "print JSON, with the schema of the parameters"}},
		Action: func(c *cli.Context) error {
			definitions := registry.Describe()
			if c.Bool("json") {
				descriptions := make([]strategyDescription, len(definitions))
				for i, def := range definitions {
					descriptions[i] = strategyDescription{Definition: def, Schema: def.JSONSchema()}
				}
				return writeJSON(c, descriptions)
			}

			table := tabwriter.NewWriter(c.App.Writer, 0, 0, 2, ' ', 0)
			for _, def := range definitions {
				fmt.Fprintf(table, "%s\t%s\n", def.Name, def.Description)
				for _, param := range def.Params {
					fmt.Fprintf(table, "  %s\t%s\n", param.Name, describeParam(param))
				}
			}
			return table.Flush()
		},
	}
}

func describeParam(param strategy.ParamSpec) string {
	parts := []string{param.Type}
	if param.Default != nil {
		parts = append(parts, fmt.Sprintf("default %v", param.Default))
	}
	if param.Min != nil {
		parts = append(parts, fmt.Sprintf("min %v", *param.Min))
	}
	if param.Max != nil {
		parts = append(parts, fmt.Sprintf("max %v", *param.Max))
	}
	if len(param.Enum) > 0 {
		parts = append(parts, "one of "+strings.Join(param.Enum, "|"))
	}
	text := strings.Join(parts, ", ")
	if param.Description != "" {
		text += " — " + param.Description
	}
	return text
}

func validateCommand(registry *strategy.Registry) *cli.Command {
	return &cli.Command{
		Name: "validate",
		Usage: "Check a spec file without running it: a run, sweep or portfolio spec, its strategy " +
			"parameters and its data files",
		Description: "Prints ok and the hash of the resolved run (every parameter explicit), or the number\n" +
			"of runs of a sweep, or the hash of a portfolio.",
		Flags: []cli.Flag{specFlag},
		Action: func(c *cli.Context) error {
			path := c.String("spec")
			if path == "" {
				return usageError("--spec is required")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return usageError("read spec: %v", err)
			}
			dir := filepath.Dir(path)

			var probe map[string]json.RawMessage
			if err := yamlOrJSON(data, &probe); err != nil {
				return err
			}

			switch {
			case probe["strategies"] != nil:
				spec, err := portfolio.ReadSpec(bytes.NewReader(data))
				if err != nil {
					return err
				}
				if err := spec.Validate(); err != nil {
					return err
				}
				for _, run := range spec.Runs() {
					if _, err := run.Resolve(registry); err != nil {
						return fmt.Errorf("strategy %s: %w", run.Name, err)
					}
				}
				if err := checkFiles(spec.Runs()[0], dir); err != nil {
					return err
				}
				fmt.Fprintf(c.App.Writer, "ok portfolio of %d strategies %s\n", len(spec.Strategies), spec.Hash())
			case probe["grid"] != nil || probe["random"] != nil:
				spec, err := sweep.ReadSpec(bytes.NewReader(data))
				if err != nil {
					return err
				}
				runs, err := spec.Expand()
				if err != nil {
					return err
				}
				for _, run := range runs {
					if _, err := run.Resolve(registry); err != nil {
						return err
					}
				}
				if err := checkFiles(spec.Spec, dir); err != nil {
					return err
				}
				fmt.Fprintf(c.App.Writer, "ok sweep of %d runs\n", len(runs))
			default:
				spec, err := backtest.ParseSpec(data)
				if err != nil {
					return err
				}
				if err := spec.Validate(); err != nil {
					return err
				}
				resolved, err := spec.Resolve(registry)
				if err != nil {
					return err
				}
				if err := checkFiles(spec, dir); err != nil {
					return err
				}
				fmt.Fprintf(c.App.Writer, "ok %s\n", resolved.Hash())
			}
			return nil
		},
	}
}

// yamlOrJSON decodes the top-level keys of a JSON or YAML document.
func yamlOrJSON(data []byte, into *map[string]json.RawMessage) error {
	var document map[string]any
	if err := backtest.DecodeStrict(data, &document); err != nil {
		return err
	}
	*into = make(map[string]json.RawMessage, len(document))
	for key, value := range document {
		raw, _ := json.Marshal(value)
		(*into)[key] = raw
	}
	return nil
}

func checkFiles(spec backtest.Spec, dir string) error {
	for _, source := range spec.Data {
		path := source.File
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("%w: data file of %s: %w", backtest.ErrData, source.Pair, err)
		}
	}
	return nil
}

func schemaCommand(registry *strategy.Registry) *cli.Command {
	return &cli.Command{
		Name:      "schema",
		Usage:     "Print the JSON Schema of a file format: spec, result, or strategy <name>",
		ArgsUsage: "spec | result | strategy <name>",
		Action: func(c *cli.Context) error {
			switch c.Args().First() {
			case "spec":
				return writeJSON(c, jsonschema.For(backtest.Spec{}, "ninjabot backtest spec"))
			case "result":
				return writeJSON(c, jsonschema.For(backtest.Result{}, "ninjabot backtest result"))
			case "strategy":
				def, err := registry.Lookup(c.Args().Get(1))
				if err != nil {
					return err
				}
				return writeJSON(c, def.JSONSchema())
			}
			return usageError("schema needs one of: spec, result, strategy <name>")
		},
	}
}
