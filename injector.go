package goinject

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

var errorReflectType = reflect.TypeFor[error]()
var invocationContextReflectType = reflect.TypeFor[InvocationContext]()

// Injector defines bindings & scopes
type Injector struct {
	bindings       map[reflect.Type]map[string][]*binding // list of available bindings by type and annotations
	scopes         map[string]Scope                       // Scope by names
	singletonScope *singletonScope
}

// NewInjector builds up a new Injector out of a list of Modules with singleton scope
func NewInjector(options ...Option) (*Injector, error) {
	mod := &configuration{
		bindings: make(map[*binding]bool),
		scopes:   make(map[string]Scope),
	}

	for _, o := range options {
		err := o.apply(mod)
		if err != nil {
			return nil, err
		}
	}

	singletonScope := newSingletonScope()
	mod.scopes[Singleton] = singletonScope
	mod.scopes[PerLookUp] = newPerLookUpScope()

	injector := &Injector{
		bindings:       make(map[reflect.Type]map[string][]*binding),
		scopes:         make(map[string]Scope),
		singletonScope: singletonScope,
	}

	injectorType := reflect.TypeFor[*Injector]()
	injectorBinding := &binding{
		typeof:       injectorType,
		provider:     reflect.ValueOf(func() *Injector { return injector }),
		providedType: injectorType,
		scope:        Singleton,
	}

	injector.scopes = mod.scopes
	for b := range mod.bindings {
		_, ok := injector.bindings[b.typeof]
		if !ok {
			injector.bindings[b.typeof] = make(map[string][]*binding)
		}
		injector.bindings[b.typeof][b.annotatedWith] = append(injector.bindings[b.typeof][b.annotatedWith], b)
	}

	injector.bindings[injectorType] = make(map[string][]*binding)
	injector.bindings[injectorType][""] = []*binding{injectorBinding}

	err := injector.eagerlyCreateSingletons()
	if err != nil {
		return nil, err
	}
	return injector, nil
}

// Shutdown clear underlying singleton scope
func (injector *Injector) Shutdown() {
	injector.singletonScope.Shutdown()
	injector.bindings = make(map[reflect.Type]map[string][]*binding)
	injector.scopes = make(map[string]Scope)
}

// Invoke will execute the parameter function (which must be a function that optionally can return an error).
// argument of function will be resolved by the injector using configured providers & scope.
func (injector *Injector) Invoke(ctx context.Context, function any) error {
	if function == nil {
		return newInvalidInputError("can't invoke on nil")
	}
	fvalue := reflect.ValueOf(function)
	ftype := fvalue.Type()
	if ftype.Kind() != reflect.Func {
		return newInvalidInputError(
			fmt.Sprintf("can't invoke non-function %v (type %v)", function, ftype))
	}

	if ftype.NumOut() > 1 || (ftype.NumOut() == 1 && !ftype.Out(0).AssignableTo(errorReflectType)) {
		return newInvalidInputError("can't invoke on function whose return type is not error or no return type")
	}

	res, err := injector.callFunctionWithArgumentInstance(ctx, fvalue)
	if err != nil {
		return fmt.Errorf("failed to call invokation function: %w", err)
	}
	if ftype.NumOut() == 1 {
		invokationError := res[0].Interface().(error)
		if invokationError != nil {
			return fmt.Errorf("invokation returned error: %w", invokationError)
		}
	}
	return nil
}

func (injector *Injector) eagerlyCreateSingletons() error {
	for _, bindingsByAnnotation := range injector.bindings {
		for _, bindingList := range bindingsByAnnotation {
			for _, b := range bindingList {
				if b.scope == Singleton {
					_, err := injector.getScopedInstanceFromBinding(nil, b) //nolint:staticcheck
					if err != nil {
						return fmt.Errorf("failed to get singleton instance: %w", err)
					}
				}
			}
		}
	}
	return nil
}

func (injector *Injector) callFunctionWithArgumentInstance(
	ctx context.Context,
	fValue reflect.Value,
) ([]reflect.Value, error) {
	fType := fValue.Type()
	in := make([]reflect.Value, fType.NumIn())
	var err error
	for i := 0; i < fType.NumIn(); i++ {
		if in[i], err = injector.getFunctionArgumentInstance(ctx, fType.In(i)); err != nil {
			return []reflect.Value{}, fmt.Errorf("failed to resolve function argument #%d: %w", i, err)
		}
	}

	res := fValue.Call(in)
	return res, nil
}

func (injector *Injector) getFunctionArgumentInstance(ctx context.Context, argType reflect.Type) (reflect.Value, error) {
	if EmbedsParams(argType) {
		return injector.createEmbeddedParams(ctx, argType)
	} else {
		return injector.getInstanceOfAnnotatedType(ctx, argType, "", false)
	}
}

func (injector *Injector) createEmbeddedParams(ctx context.Context, embeddedType reflect.Type) (reflect.Value, error) {
	if embeddedType.Kind() == reflect.Ptr {
		n := reflect.New(embeddedType.Elem())
		return n, injector.setParamFields(ctx, n.Elem())
	} else { // struct
		n := reflect.New(embeddedType).Elem()
		return n, injector.setParamFields(ctx, n)
	}
}

func (injector *Injector) setParamFields(
	ctx context.Context,
	paramValue reflect.Value,
) error {
	embeddedType := paramValue.Type()
	for fieldIndex := 0; fieldIndex < embeddedType.NumField(); fieldIndex++ {
		field := paramValue.Field(fieldIndex)
		if field.Type() == _paramType {
			continue
		}
		if tag, ok := embeddedType.Field(fieldIndex).Tag.Lookup("inject"); ok {
			if !field.CanSet() {
				return newInjectionError(field.Type(), tag, fmt.Errorf("use inject tag on unsettable field"))
			}

			var optional bool
			for _, option := range strings.Split(tag, ",") {
				if strings.TrimSpace(option) == "optional" {
					optional = true
				}
			}
			tag = strings.Split(tag, ",")[0]

			instance, err := injector.getInstanceOfAnnotatedType(ctx, field.Type(), tag, optional)
			if err != nil {
				return newInjectionError(field.Type(), tag, err)
			}
			if instance.IsValid() {
				field.Set(instance)
			} else if optional {
				continue
			} else {
				return newInjectionError(field.Type(), tag, fmt.Errorf("cannot get valid instance from scope"))
			}
		}
	}
	return nil
}

// getInstanceOfAnnotatedType resolves a type request within the injector
func (injector *Injector) getInstanceOfAnnotatedType(
	ctx context.Context,
	t reflect.Type,
	annotation string,
	optional bool,
) (reflect.Value, error) {
	// if is slice, return as multi bindings
	if t.Kind() == reflect.Slice {
		bindings := injector.findBindingsForAnnotatedType(t.Elem(), annotation)
		if len(bindings) > 0 {
			n := reflect.MakeSlice(t, 0, len(bindings))
			for _, binding := range bindings {
				r, err := injector.getScopedInstanceFromBinding(ctx, binding)
				if err != nil {
					return reflect.Value{}, err
				}
				n = reflect.Append(n, r)
			}
			return n, nil
		} else if optional {
			return reflect.MakeSlice(t, 0, 0), nil
		} else {
			return reflect.MakeSlice(t, 0, 0), newInjectionError(t.Elem(), annotation,
				fmt.Errorf("did not found binding, expected at least one"))
		}
	}

	// check if there is a binding for this type & annotation
	bindings := injector.findBindingsForAnnotatedType(t, annotation)
	if len(bindings) > 1 {
		return reflect.Value{},
			newInjectionError(t, annotation, fmt.Errorf("found multiple bindings expected one"))
	} else if len(bindings) == 1 {
		return injector.getScopedInstanceFromBinding(ctx, bindings[0])
	} else if injector.isProviderType(t) {
		return injector.createProviderValue(t, annotation, optional), nil
	} else if t == invocationContextReflectType {
		return reflect.ValueOf(ctx), nil
	} else if optional {
		return reflect.Value{}, nil
	} else {
		return reflect.Value{},
			newInjectionError(t, annotation, fmt.Errorf("did not found binding, expected one"))
	}
}

func (injector *Injector) isProviderType(t reflect.Type) bool {
	return t.Kind() == reflect.Func &&
		t.NumIn() == 1 && t.In(0) == invocationContextReflectType &&
		t.NumOut() == 2 && t.Out(1) == errorReflectType
}

func (injector *Injector) createProviderValue(
	t reflect.Type,
	annotation string,
	optional bool,
) reflect.Value {
	bindingType := t.Out(0)
	return reflect.MakeFunc(t, func(args []reflect.Value) (results []reflect.Value) {
		ctx := args[0].Interface().(context.Context)
		instance, err := injector.getInstanceOfAnnotatedType(ctx, bindingType, annotation, optional)
		var instanceVal reflect.Value
		if instance.IsValid() {
			instanceVal = instance
		} else {
			instanceVal = reflect.Zero(bindingType)
		}
		var errVal reflect.Value
		if err != nil {
			errVal = reflect.ValueOf(err)
		} else {
			errVal = reflect.Zero(errorReflectType)
		}
		return []reflect.Value{
			instanceVal,
			errVal,
		}
	})
}

func (injector *Injector) findBindingsForAnnotatedType(
	t reflect.Type,
	annotation string,
) []*binding {
	if _, ok := injector.bindings[t]; ok && len(injector.bindings[t][annotation]) > 0 {
		bindings := injector.bindings[t][annotation]
		res := make([]*binding, len(bindings))
		copy(res, bindings)
		return res
	}

	return []*binding{}
}

func (injector *Injector) getScopedInstanceFromBinding(
	ctx context.Context,
	binding *binding,
) (reflect.Value, error) {
	scope, err := injector.getScopeFromBinding(binding)
	if err != nil {
		return reflect.Value{}, err
	}
	val, err := scope.ResolveBinding(ctx, binding, func() (Instance, error) {
		val, creationError := binding.create(ctx, injector)
		destroyMethod := binding.destroyMethod
		if creationError == nil && destroyMethod != nil && !val.IsZero() {
			scope.RegisterDestructionCallback(
				ctx,
				func() { destroyMethod(val) },
			)
		}
		return Instance(val), creationError
	})
	return reflect.Value(val), err
}

func (injector *Injector) getScopeFromBinding(
	binding *binding,
) (Scope, error) {
	if scope, ok := injector.scopes[binding.scope]; ok {
		return scope, nil
	}
	return nil, newInjectionError(
		binding.typeof, binding.annotatedWith, fmt.Errorf("unknown scope %q for binding", binding.scope))
}
