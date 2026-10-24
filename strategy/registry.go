package strategy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Errors returned by a Registry.
var (
	ErrUnknownStrategy = errors.New("strategy: unknown strategy")
	ErrInvalidParams   = errors.New("strategy: invalid params")
)

// Factory builds a strategy from its parameters, already validated against
// the ParamSpecs of its definition and completed with their defaults.
type Factory func(params map[string]any) (Strategy, error)

// Parameter types of a ParamSpec.
const (
	TypeInt    = "int"
	TypeFloat  = "float"
	TypeString = "string"
	TypeBool   = "bool"
)

// ParamSpec describes one parameter of a strategy.
type ParamSpec struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // int, float, string or bool
	Description string   `json:"description,omitempty"`
	Default     any      `json:"default,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// Definition is a strategy known to a registry.
type Definition struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Params      []ParamSpec `json:"params"`
	Factory     Factory     `json:"-"`
}

// Registry maps strategy names to factories, so strategies can be created
// from configuration files and command lines. It is safe for concurrent use.
type Registry struct {
	mu          sync.RWMutex
	definitions map[string]Definition
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{definitions: make(map[string]Definition)}
}

// Default is the registry used by Register and by the ninjabot CLI.
var Default = NewRegistry()

// Register adds a strategy to the Default registry. It panics on a
// duplicated name or an invalid spec, which are programming errors.
func Register(name string, factory Factory, params ...ParamSpec) {
	if err := Default.Add(Definition{Name: name, Params: params, Factory: factory}); err != nil {
		panic(err)
	}
}

// RegisterParams adds to the Default registry a strategy whose parameters
// are the fields of P (see ParamsOf). The factory receives P decoded and
// completed with the defaults of its tags.
func RegisterParams[P any](name, description string, build func(P) (Strategy, error)) {
	if err := Default.Add(DefinitionOf(name, description, build)); err != nil {
		panic(err)
	}
}

// DefinitionOf builds the definition of a strategy whose parameters are the
// fields of P.
func DefinitionOf[P any](name, description string, build func(P) (Strategy, error)) Definition {
	var zero P
	return Definition{
		Name:        name,
		Description: description,
		Params:      ParamsOf(zero),
		Factory: func(params map[string]any) (Strategy, error) {
			var p P
			if err := DecodeParams(params, &p); err != nil {
				return nil, err
			}
			return build(p)
		},
	}
}

// Add registers a definition.
func (r *Registry) Add(def Definition) error {
	if def.Name == "" || def.Factory == nil {
		return errors.New("strategy: a definition needs a name and a factory")
	}
	seen := make(map[string]bool, len(def.Params))
	for _, param := range def.Params {
		if seen[param.Name] {
			return fmt.Errorf("strategy: %s declares param %q twice", def.Name, param.Name)
		}
		seen[param.Name] = true
		switch param.Type {
		case TypeInt, TypeFloat, TypeString, TypeBool:
		default:
			return fmt.Errorf("strategy: %s param %q has unknown type %q", def.Name, param.Name, param.Type)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.definitions[def.Name]; ok {
		return fmt.Errorf("strategy: %s registered twice", def.Name)
	}
	r.definitions[def.Name] = def
	return nil
}

// Lookup returns the definition of a strategy.
func (r *Registry) Lookup(name string) (Definition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.definitions[name]
	if !ok {
		return Definition{}, fmt.Errorf("%w: %q (known: %s)", ErrUnknownStrategy, name, strings.Join(r.names(), ", "))
	}
	return def, nil
}

// New validates the parameters, fills the defaults and builds the strategy.
// Every call returns a new instance, so parallel runs never share state.
func (r *Registry) New(name string, params map[string]any) (Strategy, error) {
	def, err := r.Lookup(name)
	if err != nil {
		return nil, err
	}
	resolved, err := def.Resolve(params)
	if err != nil {
		return nil, err
	}
	strategy, err := def.Factory(resolved)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrInvalidParams, name, err)
	}
	return strategy, nil
}

// Describe returns every definition, sorted by name.
func (r *Registry) Describe() []Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	definitions := make([]Definition, 0, len(r.definitions))
	for _, name := range r.names() {
		definitions = append(definitions, r.definitions[name])
	}
	return definitions
}

func (r *Registry) names() []string {
	names := make([]string, 0, len(r.definitions))
	for name := range r.definitions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Resolve checks params against the specs and returns a copy completed with
// the defaults. Integers given as whole floats (as JSON decodes them) are
// accepted.
func (d Definition) Resolve(params map[string]any) (map[string]any, error) {
	specs := make(map[string]ParamSpec, len(d.Params))
	for _, spec := range d.Params {
		specs[spec.Name] = spec
	}

	resolved := make(map[string]any, len(d.Params))
	for name, value := range params {
		spec, ok := specs[name]
		if !ok {
			return nil, fmt.Errorf("%w: %s has no param %q (known: %s)", ErrInvalidParams, d.Name, name,
				strings.Join(d.paramNames(), ", "))
		}
		normalized, err := spec.check(value)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrInvalidParams, d.Name, err)
		}
		resolved[name] = normalized
	}
	for _, spec := range d.Params {
		if _, ok := resolved[spec.Name]; !ok && spec.Default != nil {
			resolved[spec.Name] = spec.Default
		}
	}
	return resolved, nil
}

func (d Definition) paramNames() []string {
	names := make([]string, len(d.Params))
	for i, spec := range d.Params {
		names[i] = spec.Name
	}
	return names
}

func (p ParamSpec) check(value any) (any, error) {
	var number float64
	switch p.Type {
	case TypeInt:
		f, ok := toFloat(value)
		if !ok || f != math.Trunc(f) {
			return nil, fmt.Errorf("param %q must be an integer, got %v", p.Name, value)
		}
		number, value = f, int(f)
	case TypeFloat:
		f, ok := toFloat(value)
		if !ok {
			return nil, fmt.Errorf("param %q must be a number, got %v", p.Name, value)
		}
		number, value = f, f
	case TypeString:
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("param %q must be a string, got %v", p.Name, value)
		}
		if len(p.Enum) > 0 && !contains(p.Enum, s) {
			return nil, fmt.Errorf("param %q must be one of %s, got %q", p.Name, strings.Join(p.Enum, ", "), s)
		}
		return s, nil
	case TypeBool:
		if _, ok := value.(bool); !ok {
			return nil, fmt.Errorf("param %q must be a boolean, got %v", p.Name, value)
		}
		return value, nil
	}

	if p.Min != nil && number < *p.Min {
		return nil, fmt.Errorf("param %q must be >= %v, got %v", p.Name, *p.Min, number)
	}
	if p.Max != nil && number > *p.Max {
		return nil, fmt.Errorf("param %q must be <= %v, got %v", p.Name, *p.Max, number)
	}
	return value, nil
}

func toFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	}
	return 0, false
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

// JSONSchema returns the JSON Schema (draft 2020-12) of the parameters.
func (d Definition) JSONSchema() map[string]any {
	properties := make(map[string]any, len(d.Params))
	for _, spec := range d.Params {
		property := map[string]any{}
		switch spec.Type {
		case TypeInt:
			property["type"] = "integer"
		case TypeFloat:
			property["type"] = "number"
		case TypeString:
			property["type"] = "string"
		case TypeBool:
			property["type"] = "boolean"
		}
		if spec.Description != "" {
			property["description"] = spec.Description
		}
		if spec.Default != nil {
			property["default"] = spec.Default
		}
		if spec.Min != nil {
			property["minimum"] = *spec.Min
		}
		if spec.Max != nil {
			property["maximum"] = *spec.Max
		}
		if len(spec.Enum) > 0 {
			property["enum"] = spec.Enum
		}
		properties[spec.Name] = property
	}
	schema := map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"title":                d.Name,
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if d.Description != "" {
		schema["description"] = d.Description
	}
	return schema
}

// ParamsOf derives the parameter specs from the exported fields of a struct.
// The name comes from the json tag; the other tags are default, description,
// min, max and enum (comma separated). Fields of type int, float64, string
// and bool are supported.
//
//	type Params struct {
//		Fast int `json:"fast" default:"8" min:"2" description:"fast EMA period"`
//	}
func ParamsOf(v any) []ParamSpec {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("strategy: params of %s are not a struct", t))
	}

	specs := make([]ParamSpec, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name, ok := paramName(field)
		if !ok {
			continue
		}
		spec := ParamSpec{Name: name, Description: field.Tag.Get("description")}
		switch field.Type.Kind() {
		case reflect.Int, reflect.Int64, reflect.Int32:
			spec.Type = TypeInt
		case reflect.Float64, reflect.Float32:
			spec.Type = TypeFloat
		case reflect.String:
			spec.Type = TypeString
		case reflect.Bool:
			spec.Type = TypeBool
		default:
			panic(fmt.Sprintf("strategy: param %s has unsupported type %s", name, field.Type))
		}
		if raw, ok := field.Tag.Lookup("default"); ok {
			spec.Default = parseDefault(spec.Type, name, raw)
		}
		spec.Min = parseBound(field.Tag, "min")
		spec.Max = parseBound(field.Tag, "max")
		if enum := field.Tag.Get("enum"); enum != "" {
			spec.Enum = strings.Split(enum, ",")
		}
		specs = append(specs, spec)
	}
	return specs
}

func paramName(field reflect.StructField) (string, bool) {
	if !field.IsExported() {
		return "", false
	}
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false
	}
	if name, _, _ := strings.Cut(tag, ","); name != "" {
		return name, true
	}
	return field.Name, true
}

func parseDefault(kind, name, raw string) any {
	var (
		value any
		err   error
	)
	switch kind {
	case TypeInt:
		value, err = strconv.Atoi(raw)
	case TypeFloat:
		value, err = strconv.ParseFloat(raw, 64)
	case TypeBool:
		value, err = strconv.ParseBool(raw)
	default:
		value = raw
	}
	if err != nil {
		panic(fmt.Sprintf("strategy: invalid default %q of param %s: %v", raw, name, err))
	}
	return value
}

func parseBound(tag reflect.StructTag, key string) *float64 {
	raw, ok := tag.Lookup(key)
	if !ok {
		return nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		panic(fmt.Sprintf("strategy: invalid %s %q: %v", key, raw, err))
	}
	return &value
}

// DecodeParams decodes params into the struct pointed by into: fields absent
// from params take the value of their default tag, and keys that match no
// field are an error.
func DecodeParams(params map[string]any, into any) error {
	target := reflect.ValueOf(into)
	if target.Kind() != reflect.Pointer || target.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("strategy: DecodeParams needs a pointer to a struct, got %T", into)
	}

	merged := make(map[string]any, len(params))
	for _, spec := range ParamsOf(into) {
		if spec.Default != nil {
			merged[spec.Name] = spec.Default
		}
	}
	for key, value := range params {
		merged[key] = value
	}

	data, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidParams, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidParams, err)
	}
	return nil
}

// WithDefaults returns p with every zero field replaced by the default tag
// of the field. It lets a strategy built as a zero value, or with only some
// parameters set, behave like one built from the registry.
func WithDefaults[P any](p P) P {
	var defaults P
	if err := DecodeParams(nil, &defaults); err != nil {
		panic(err)
	}
	value := reflect.ValueOf(&p).Elem()
	fallback := reflect.ValueOf(defaults)
	for i := 0; i < value.NumField(); i++ {
		if field := value.Field(i); field.CanSet() && field.IsZero() {
			field.Set(fallback.Field(i))
		}
	}
	return p
}
