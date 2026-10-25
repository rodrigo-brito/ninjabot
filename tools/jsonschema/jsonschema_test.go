package jsonschema

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type custom struct{}

func (custom) MarshalJSON() ([]byte, error) { return []byte("1"), nil }

type inner struct {
	Value float64 `json:"value"`
}

type Embedded struct {
	Flag bool `json:"flag"`
}

type sample struct {
	Embedded
	Name     string           `json:"name"`
	Count    int              `json:"count,omitempty"`
	When     time.Time        `json:"when"`
	Took     time.Duration    `json:"took"`
	Items    []inner          `json:"items"`
	ByName   map[string]inner `json:"by_name"`
	Optional *inner           `json:"optional"`
	Any      any              `json:"any"`
	Custom   custom           `json:"custom"`
	Self     []sample         `json:"self"`
	Skipped  string           `json:"-"`
	NoTag    uint8
	hidden   int // nolint:unused // exercises unexported fields
}

func TestFor(t *testing.T) {
	schema := For(sample{}, "sample")
	require.Equal(t, "https://json-schema.org/draft/2020-12/schema", schema["$schema"])
	require.Equal(t, "sample", schema["title"])

	properties := schema["properties"].(map[string]any)
	require.Equal(t, map[string]any{"type": "boolean"}, properties["flag"])
	require.Equal(t, map[string]any{"type": "string"}, properties["name"])
	require.Equal(t, map[string]any{"type": "string", "format": "date-time"}, properties["when"])
	require.Equal(t, "integer", properties["took"].(map[string]any)["type"])
	require.Equal(t, "array", properties["items"].(map[string]any)["type"])
	require.Equal(t, "object", properties["by_name"].(map[string]any)["type"])
	require.Equal(t, map[string]any{}, properties["any"])
	require.Equal(t, map[string]any{}, properties["custom"])
	require.Equal(t, map[string]any{"type": "integer"}, properties["NoTag"])
	require.NotContains(t, properties, "Skipped")
	require.NotContains(t, properties, "hidden")

	required := schema["required"].([]string)
	require.Contains(t, required, "name")
	require.NotContains(t, required, "count")
	require.NotContains(t, required, "optional")

	_, err := json.Marshal(schema)
	require.NoError(t, err)
}
