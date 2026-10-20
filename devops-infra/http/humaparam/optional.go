// Package humaparam provides huma request parameter helper types.
//
// huma's parameter pipeline does not support pointer types (form/header/path/query
// panic, huma.go:215), so optional semantics are implemented via the
// ParamWrapper/ParamReactor interfaces: parsing writes to Value, presence is recorded in IsSet.
package humaparam

import (
	"reflect"

	"github.com/danielgtaylor/huma/v2"
)

// Optional wraps an optional parameter, distinguishing "not provided" from "explicit zero value".
//
//	Declared in Input: Status humaparam.Optional[int8] `query:"status" minimum:"0" maximum:"1"`
//	Handler-side conversion: svc.List(ctx, in.Status.Ptr()) — nil when not provided, aligned with the service layer's *int8 semantics.
type Optional[T any] struct {
	Value T
	IsSet bool
}

// Receiver implements huma.ParamWrapper, exposing the field that receives the parsed value.
func (o *Optional[T]) Receiver() reflect.Value {
	return reflect.ValueOf(o).Elem().Field(0)
}

// Schema implements huma.SchemaProvider: docs and validation use the inner T's schema
// rather than the wrapper struct's object schema (without this step validation would
// report "expected object"; see the official OptionalParam example at huma_test.go:115).
func (o Optional[T]) Schema(r huma.Registry) *huma.Schema {
	return huma.SchemaFromType(r, reflect.TypeOf(o.Value))
}

// OnParamSet implements huma.ParamReactor, recording whether the parameter appeared in the request.
func (o *Optional[T]) OnParamSet(isSet bool, _ any) {
	o.IsSet = isSet
}

// Ptr adapts to the service layer's pointer semantics: nil when not provided,
// a pointer to the value otherwise.
func (o *Optional[T]) Ptr() *T {
	if !o.IsSet {
		return nil
	}
	v := o.Value
	return &v
}
