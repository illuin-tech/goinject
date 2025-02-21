package goinject

import (
	"context"
	"reflect"
)

// Params may be embedded in struct to request the injector to create it
// as special struct. When a constructor accepts such a struct, instead of the
// struct becoming a dependency for that constructor, all its fields become
// dependencies instead.
//
// Fields of the struct may optionally be tagged.
// The following tags are supported,
//
//	annotation    Requests a value with the same name and type from the
//	              container. See Named Values for more information.
//	optional      If set to true, indicates that the dependency is optional and
//	              the constructor gracefully handles its absence.
type Params struct{}

var _paramType = reflect.TypeOf(Params{})

// EmbedsParams checks whether the given struct is an inject.Params struct. A struct qualifies
// as an inject.Params struct if it embeds inject.Params type.
//
// A struct MUST qualify as an inject.Params struct for its fields to be treated
// specially by the injector.
func EmbedsParams(o reflect.Type) bool {
	return embedsType(o, _paramType)
}

// Returns true if t embeds e
func embedsType(t, e reflect.Type) bool {
	if t.Kind() == reflect.Ptr {
		return embedsType(t.Elem(), e)
	}

	if t.Kind() != reflect.Struct {
		// for now, only struct are supported, it might be a good idea to support pointer too
		return false
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous && f.Type == e {
			return true
		}
	}

	return false
}

type Provider[T any] func(ctx InvocationContext) (T, error)

// InvocationContext wrap context.Context.
// Use this interface to retrieve the context pass to the Invoke method of the injector in providers
type InvocationContext interface {
	context.Context
}
