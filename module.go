package goinject

import (
	"fmt"
	"reflect"
)

type configuration struct {
	bindings map[*binding]bool
	scopes   map[string]Scope
}

// Option enable to configure the given injector
type Option interface {
	apply(*configuration) error
}

type moduleOption struct {
	name    string
	options []Option
}

func (o *moduleOption) apply(mod *configuration) error {
	for _, opt := range o.options {
		err := opt.apply(mod)
		if err != nil {
			return newInjectorConfigurationError(
				fmt.Sprintf("error while installing module %s", o.name), err)
		}
	}
	return nil
}

// Module group a list of Option in order to easily reuse them.
// the Module name is used in error when applying Option to easily find misconfigured options.
func Module(name string, opts ...Option) Option {
	mo := &moduleOption{
		name:    name,
		options: opts,
	}
	return mo
}

type provideOption struct {
	constructor any
	annotations []Annotation
}

func (o *provideOption) apply(mod *configuration) error {
	if o.constructor == nil {
		return newInjectorConfigurationError("cannot accept nil provider", nil)
	}
	providerFncValue := reflect.ValueOf(o.constructor)
	fncType := providerFncValue.Type()
	if fncType.Kind() != reflect.Func {
		return newInjectorConfigurationError("provider argument should be a function", nil)
	}
	if fncType.NumOut() > 2 || fncType.NumOut() == 0 {
		return newInjectorConfigurationError("expected a function that return an instance and optionally an error", nil)
	}
	if fncType.NumOut() == 2 && !fncType.Out(1).AssignableTo(reflect.TypeOf(new(error)).Elem()) {
		return newInjectorConfigurationError("second return type of provider should be an error", nil)
	}
	b := &binding{}
	b.provider = providerFncValue
	b.providedType = fncType.Out(0)
	b.typeof = b.providedType
	b.scope = Singleton

	for _, a := range o.annotations {
		err := a.apply(b)
		if err != nil {
			return newInjectorConfigurationError(
				fmt.Sprintf("got error while configuring provider for provided type %s", b.providedType),
				err,
			)
		}
	}

	mod.bindings[b] = true
	return nil
}

// Provide define a binding from a function constructor that must return the provided instance (and optionally an error)
// arguments of the constructor parameter will be resolved by the injector itself.
// Provide enable to annotate the created binding using Annotation
func Provide(constructor any, annotations ...Annotation) Option {
	return &provideOption{
		constructor: constructor,
		annotations: annotations,
	}
}

type registerScopeOption struct {
	name  string
	scope Scope
}

func (o *registerScopeOption) apply(mod *configuration) error {
	mod.scopes[o.name] = o.scope
	return nil
}

// RegisterScope register a new Scope with a name
func RegisterScope(name string, scope Scope) Option {
	return &registerScopeOption{
		name:  name,
		scope: scope,
	}
}

type whenOption struct {
	condition Conditional
	options   []Option
}

func (o *whenOption) apply(mod *configuration) error {
	if o.condition.evaluate() {
		for _, opt := range o.options {
			if err := opt.apply(mod); err != nil {
				return err
			}
		}
	}

	return nil
}

// When enable to group a list of Option that will be applied only if the given Conditional evaluate to true
func When(condition Conditional, options ...Option) Option {
	return &whenOption{
		condition: condition,
		options:   options,
	}
}

// Annotation are used to configured bindings created by the Provide function
type Annotation interface {
	apply(b *binding) error
}

type asAnnotation struct {
	target AsType
}

func (a *asAnnotation) apply(b *binding) error {
	targetType := a.target.getType()
	if !b.providedType.AssignableTo(targetType) {
		return newInjectorConfigurationError(
			fmt.Sprintf("cannot assign %s to %s as specified in As argument", b.providedType, targetType),
			nil,
		)
	}
	b.typeof = targetType
	return nil
}

// AsType is used in As function as an argument to register a provided type to another given assignable type
type AsType interface {
	getType() reflect.Type
}

type typeFor[T any] struct {
}

func (t *typeFor[T]) getType() reflect.Type {
	return reflect.TypeFor[T]()
}

// Type return an AsType for a given type T
func Type[T any]() AsType {
	return &typeFor[T]{}
}

// As return an annotation that is used to override the binding registration type.
// Use it to bind a concrete type to an interface.
func As(target AsType) Annotation {
	return &asAnnotation{target: target}
}

type nameAnnotation struct {
	name string
}

func (a *nameAnnotation) apply(b *binding) error {
	b.annotatedWith = a.name
	return nil
}

// Named return an annotation that is used to define the binding annotation name.
func Named(name string) Annotation {
	return &nameAnnotation{name: name}
}

type inAnnotation struct {
	scope string
}

// In return an annotation that is used to define the binding scope
func In(scope string) Annotation {
	return &inAnnotation{scope: scope}
}

func (a *inAnnotation) apply(b *binding) error {
	b.scope = a.scope
	return nil
}

type withDestroyAnnotation struct {
	destroyMethod any
}

func (a *withDestroyAnnotation) apply(b *binding) error {
	destroyMethodFnVal := reflect.ValueOf(a.destroyMethod)
	if destroyMethodFnVal.Kind() != reflect.Func ||
		destroyMethodFnVal.Type().NumIn() != 1 ||
		destroyMethodFnVal.Type().In(0) != b.providedType ||
		destroyMethodFnVal.Type().NumOut() != 0 {
		return newInjectorConfigurationError(
			"argument of WithDestroy must be a function with one argument returning void",
			nil,
		)
	}
	b.destroyMethod = func(val reflect.Value) {
		destroyMethodFnVal.Call([]reflect.Value{val})
	}
	return nil
}

// WithDestroy return an annotation that declare a destroyMethod that will be used when closing a scope
func WithDestroy(destroyMethod any) Annotation {
	return &withDestroyAnnotation{
		destroyMethod: destroyMethod,
	}
}
