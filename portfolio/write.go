package portfolio

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"
)

// WriteJSON writes the result as indented JSON.
func (r *Result) WriteJSON(w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(r)
}

// ReadResult decodes a result written by WriteJSON.
func ReadResult(r io.Reader) (*Result, error) {
	var result Result
	if err := json.NewDecoder(r).Decode(&result); err != nil {
		return nil, err
	}
	if result.SchemaVersion != ResultVersion {
		return nil, fmt.Errorf("portfolio: unsupported result schema version %d", result.SchemaVersion)
	}
	return &result, nil
}

// WriteEquityCSV writes the combined equity and the equity of every
// strategy, one row per time.
func (r *Result) WriteEquityCSV(w io.Writer) error {
	writer := csv.NewWriter(w)
	header := []string{"time", "portfolio"}
	header = append(header, r.Spec.IDs()...)
	_ = writer.Write(header)
	for i, t := range r.Combined.Times {
		row := []string{time.Unix(t, 0).UTC().Format(time.RFC3339), format(r.Combined.Equity[i])}
		for _, id := range r.Spec.IDs() {
			row = append(row, format(r.Combined.ByStrategy[id][i]))
		}
		_ = writer.Write(row)
	}
	writer.Flush()
	return writer.Error()
}

func format(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
