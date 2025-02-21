package goinject

import (
	"fmt"
	"reflect"
)

type invalidInputError struct {
	message string
}

var _ error = &invalidInputError{}

func newInvalidInputError(msg string) *invalidInputError {
	return &invalidInputError{msg}
}

func (e *invalidInputError) Error() string { return e.message }

type injectionError struct {
	rType      reflect.Type
	annotation string
	cause      error
}

var _ error = &injectionError{}

func newInjectionError(typ reflect.Type, annotation string, cause error) *injectionError {
	return &injectionError{typ, annotation, cause}
}

func (e *injectionError) Error() string {
	return fmt.Sprintf("Got error while resolving type %s (with annotation %q):\n%s", e.rType.String(), e.annotation, e.cause)
}

func (e *injectionError) Unwrap() error { return e.cause }

type contextScopedNotActiveError struct {
}

var _ error = &contextScopedNotActiveError{}

func newContextScopedNotActiveError() *contextScopedNotActiveError {
	return &contextScopedNotActiveError{}
}

func (e *contextScopedNotActiveError) Error() string { return "Scope is not active" }

type injectorConfigurationError struct {
	message string
	cause   error
}

var _ error = &injectorConfigurationError{}

func newInjectorConfigurationError(message string, cause error) *injectorConfigurationError {
	return &injectorConfigurationError{message, cause}
}

func (e *injectorConfigurationError) Error() string {
	if e.cause == nil {
		return e.message
	} else {
		return fmt.Sprintf("%s:\n%s", e.message, e.cause)
	}
}

func (e *injectorConfigurationError) Unwrap() error { return e.cause }
