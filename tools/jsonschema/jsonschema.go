// Package jsonschema derives a JSON Schema (draft 2020-12) from a Go type,
// following its encoding/json tags. It covers the types of the ninjabot file
// contracts: structs, slices, maps with string keys, pointers, numbers,
// strings, booleans, time.Time and types with a custom JSON encoding
// (described as any value).
package jsonschema

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"
)

var (
	timeType      = reflect.TypeOf(time.Time{})
	durationType  = reflect.TypeOf(time.Duration(0))
	marshalerType = reflect.TypeOf((*json.Marshaler)(nil)).Elem()
)

// For returns the schema of the type of v, titled title.
func For(v any, title string) map[string]any {
	schema := typeSchema(reflect.TypeOf(v), map[reflect.Type]bool{})
	schema["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	schema["title"] = title
	return schema
}

func typeSchema(t reflect.Type, visiting map[reflect.Type]bool) map[string]any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch {
	case t == timeType:
		return map[string]any{"type": "string", "format": "date-time"}
	case t == durationType:
		return map[string]any{"type": "integer", "description": "nanoseconds"}
	case t.Implements(marshalerType) || reflect.PointerTo(t).Implements(marshalerType):
		return map[string]any{}
	}

	switch t.Kind() {
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": typeSchema(t.Elem(), visiting)}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": typeSchema(t.Elem(), visiting)}
	case reflect.Interface:
		return map[string]any{}
	case reflect.Struct:
		if visiting[t] {
			return map[string]any{"type": "object"}
		}
		visiting[t] = true
		defer delete(visiting, t)
		return structSchema(t, visiting)
	}
	return map[string]any{}
}

func structSchema(t reflect.Type, visiting map[reflect.Type]bool) map[string]any {
	properties := map[string]any{}
	var required []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, options, _ := strings.Cut(tag, ",")
		if field.Anonymous && name == "" {
			embedded := typeSchema(field.Type, visiting)
			if props, ok := embedded["properties"].(map[string]any); ok {
				for key, value := range props {
					properties[key] = value
				}
			}
			continue
		}
		if name == "" {
			name = field.Name
		}
		properties[name] = typeSchema(field.Type, visiting)
		if !strings.Contains(options, "omitempty") && field.Type.Kind() != reflect.Pointer {
			required = append(required, name)
		}
	}

	schema := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
