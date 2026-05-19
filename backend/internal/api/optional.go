package api

import (
	"encoding/json"
	"reflect"

	"github.com/danielgtaylor/huma/v2"
)

// Optional[T] models a JSON field that carries three states:
//   - absent from the body         → Present == false, Null == false
//   - present with a value         → Present == true,  Null == false, Value populated
//   - present and explicitly null  → Present == true,  Null == true
//
// PATCH and upsert handlers use this to tell "leave this field alone" from
// "clear this field" — a distinction Go's json package can't otherwise make
// with a plain pointer.
type Optional[T any] struct {
	Present bool
	Null    bool
	Value   T
}

// UnmarshalJSON is only called when the field is *present* in the input —
// that's how we know Present should be true. If the value is JSON null,
// Null is true and Value stays at the zero value.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Present = true
	if string(data) == "null" {
		o.Null = true
		return nil
	}
	return json.Unmarshal(data, &o.Value)
}

// MarshalJSON serializes round-trips correctly: absent fields disappear via
// `omitempty`-on-pointer patterns (we never marshal an Optional ourselves
// in this codebase, but huma may marshal example values during docs gen).
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if !o.Present || o.Null {
		return []byte("null"), nil
	}
	return json.Marshal(o.Value)
}

// Schema makes the field show up in OpenAPI as a nullable version of the
// inner type. We delegate to huma's reflection-based generator for T.
func (o Optional[T]) Schema(r huma.Registry) *huma.Schema {
	s := huma.SchemaFromField(r, reflect.StructField{
		Name: "Value",
		Type: reflect.TypeOf(*new(T)),
	}, "")
	if s != nil {
		s.Nullable = true
	}
	return s
}
