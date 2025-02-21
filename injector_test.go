package goinject

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Parent struct {
}

type Child struct {
	parent *Parent
}

func TestShouldReturnFromProvider(t *testing.T) {
	assert.NotPanics(t, func() {
		injector, err := NewInjector(
			Provide(func() *Parent { return &Parent{} }),
			Provide(func(parent *Parent) *Child { return &Child{parent: parent} }),
		)
		assert.Nil(t, err)
		ctx := context.Background()
		var parent *Parent
		err = injector.Invoke(ctx, func(p *Parent) {
			parent = p
		})
		assert.Nil(t, err)
		var child *Child
		err = injector.Invoke(ctx, func(c *Child) {
			child = c
		})
		assert.Nil(t, err)
		assert.Same(t, parent, child.parent)
	})
}

func TestProvideShouldAcceptErrorReturnProviders(t *testing.T) {
	assert.NotPanics(t, func() {
		injector, err := NewInjector(
			Provide(func() (*Parent, error) { return &Parent{}, nil }, In(PerLookUp)),
			Provide(func(_ *Parent) (*Child, error) { return nil, fmt.Errorf("failed to create child") }, In(PerLookUp)),
		)
		assert.Nil(t, err)
		ctx := context.Background()
		t.Run("And return type if no error", func(t *testing.T) {
			err = injector.Invoke(ctx, func(parent *Parent) {
				assert.NotNil(t, parent)
			})
			assert.Nil(t, err)
		})
		t.Run("And return error otherwise", func(t *testing.T) {
			err = injector.Invoke(ctx, func(_ *Child) {
				assert.Fail(t, "should not be reached")
			})
			assert.ErrorContains(t, err, "failed to create child")
		})
	})
}

func TestUseUnknownScopeShouldReturnError(t *testing.T) {
	assert.NotPanics(t, func() {
		injector, err := NewInjector(
			Provide(func() *Parent { return &Parent{} }, In("unknown")),
		)
		assert.Nil(t, err)
		ctx := context.Background()
		err = injector.Invoke(ctx, func(_ *Parent) {
			assert.Fail(t, "should not be reached")
		})
		assert.ErrorContains(t, err, "unknown scope \"unknown\" for binding")
	})
}

type TestInvokeParamOptional struct {
	Params
	ParentA *Parent `inject:", optional"`
	ParentB *Parent `inject:"B"`
}

func TestInvokeWithOptional(t *testing.T) {
	assert.NotPanics(t, func() {
		t.Run("using param struct argument", func(t *testing.T) {
			injector, err := NewInjector(
				Provide(func() *Parent {
					return &Parent{}
				}, Named("B")),
			)
			assert.Nil(t, err)
			var parentA *Parent
			var parentB *Parent
			ctx := context.Background()
			err = injector.Invoke(ctx, func(param TestInvokeParamOptional) {
				parentA = param.ParentA
				parentB = param.ParentB
			})
			assert.Nil(t, err)
			assert.Nil(t, parentA)
			assert.NotNil(t, parentB)
		})

		t.Run("using param pointer argument", func(t *testing.T) {
			injector, err := NewInjector(
				Provide(func() *Parent {
					return &Parent{}
				}, Named("B")),
			)
			assert.Nil(t, err)
			var parentA *Parent
			var parentB *Parent
			ctx := context.Background()
			err = injector.Invoke(ctx, func(param *TestInvokeParamOptional) {
				parentA = param.ParentA
				parentB = param.ParentB
			})
			assert.Nil(t, err)
			assert.Nil(t, parentA)
			assert.NotNil(t, parentB)
		})
	})
}

type Color struct {
	name string
}

type TestInvokeParamAnnotated struct {
	Params
	Color *Color `inject:"red"`
}

func TestInvokeWithAnnotation(t *testing.T) {
	assert.NotPanics(t, func() {
		injector, err := NewInjector(
			Provide(func() *Color { return &Color{name: "red"} }, Named("red")),
			Provide(func() *Color { return &Color{name: "blue"} }, Named("blue")),
		)
		assert.Nil(t, err)
		var color *Color
		ctx := context.Background()
		err = injector.Invoke(ctx, func(param TestInvokeParamAnnotated) {
			color = param.Color
		})
		assert.NotNil(t, color)
		assert.Equal(t, "red", color.name)
		assert.Nil(t, err)
	})
}

func TestInvokeShouldReturnErrorIfExpectedSingleBindingButMultipleFound(t *testing.T) {
	assert.NotPanics(t, func() {
		injector, err := NewInjector(
			Provide(func() *Color { return &Color{name: "blue"} }),
			Provide(func() *Color { return &Color{name: "red"} }),
		)
		assert.Nil(t, err)
		ctx := context.Background()
		err = injector.Invoke(ctx, func(_ *Color) {
			assert.Fail(t, "should not be reached")
		})
		assert.NotNil(t, err)
		// verify error tree contains an injection error
		var expectedErrorType *injectionError
		assert.ErrorAs(t, err, &expectedErrorType)
		assert.Equal(t,
			"failed to call invokation function: failed to resolve"+
				" function argument #0: Got error while resolving type *goinject.Color"+
				" (with annotation \"\"):\nfound multiple bindings expected one",
			err.Error())
	})
}

type Red *Color
type Blue *Color

func TestInvokeUsingTypeDefinition(t *testing.T) {
	assert.NotPanics(t, func() {
		injector, err := NewInjector(
			Provide(func() *Color { return &Color{name: "blue"} }, As(Type[Blue]())),
			Provide(func() *Color { return &Color{name: "red"} }, As(Type[Red]())),
		)
		assert.Nil(t, err)
		var color Red
		ctx := context.Background()
		err = injector.Invoke(ctx, func(c Red) {
			color = c
		})
		assert.NotNil(t, color)
		assert.Equal(t, "red", color.name)
		assert.Nil(t, err)
	})
}

func TestInstallModuleShouldInstallBindingsOnce(t *testing.T) {
	assert.NotPanics(t, func() {
		subModule := Module("sub", Provide(func() *Parent {
			return &Parent{}
		}, Named("parent-in-sub")))
		parentModuleA := Module("parent-a", subModule)
		parentModuleB := Module("parent-b", subModule)
		injector, err := NewInjector(
			parentModuleA,
			parentModuleB,
		)
		assert.Nil(t, err)
		assert.NotNil(t, injector)
		assert.Equal(t, 1, len(injector.bindings[reflect.TypeFor[*Parent]()]))
		assert.Equal(t, 2, len(injector.bindings)) // we add a binding for *Injector
	})
}

type Shape interface {
	Name() string
}

type Rectangle struct {
}

func (r *Rectangle) Name() string {
	return "rectangle"
}

type Square struct {
}

func (s *Square) Name() string {
	return "square"
}

func TestBindToInterface(t *testing.T) {
	assert.NotPanics(t, func() {
		injector, err := NewInjector(
			Provide(func() *Rectangle {
				return &Rectangle{}
			}, As(Type[Shape]())),
		)
		assert.Nil(t, err)
		ctx := context.Background()
		err = injector.Invoke(ctx, func(s Shape) {
			assert.IsType(t, &Rectangle{}, s)
		})
		assert.Nil(t, err)
	})
}

func TestInjectorShouldBeProvided(t *testing.T) {
	assert.NotPanics(t, func() {
		injector, err := NewInjector()
		assert.Nil(t, err)
		ctx := context.Background()
		err = injector.Invoke(ctx, func(i *Injector) {
			assert.Same(t, i, injector)
		})
		assert.Nil(t, err)
	})
}

type WithRefCount struct {
	refCount int
}

func TestInjectorShutdownShouldShutdownSingletonScope(t *testing.T) {
	assert.NotPanics(t, func() {
		refCount := 0
		injector, err := NewInjector(
			Provide(func() *WithRefCount {
				res := &WithRefCount{refCount: refCount}
				refCount++
				return res
			}, WithDestroy(func(_ *WithRefCount) {
				refCount--
			}), In(Singleton)),
		)
		assert.Nil(t, err)
		ctx := context.Background()

		// singleton should be created eagerly
		assert.Equal(t, 1, refCount)

		err = injector.Invoke(ctx, func(c *WithRefCount) {
			assert.Equal(t, 1, refCount)
			assert.Equal(t, 0, c.refCount)
		})

		assert.Nil(t, err)
		assert.Equal(t, 1, refCount)
		injector.Shutdown()
		assert.Equal(t, 0, refCount)
		assert.Equal(t, 0, len(injector.bindings))
	})
}

func TestNewInjectorShouldReturnErrorIfEagerlyCreatedSingletonReturnError(t *testing.T) {
	returnedErr := fmt.Errorf("provider error")
	assert.NotPanics(t, func() {
		_, err := NewInjector(
			Provide(func() (*WithRefCount, error) {
				return nil, returnedErr
			}),
		)
		assert.ErrorIs(t, err, returnedErr)
		assert.Equal(t, "failed to get singleton instance: provider for type \"*goinject.WithRefCount\" "+
			"returned error: provider error", err.Error())
	})
}

type MultiBindOptionalInvokeParams struct {
	Params
	Shapes []Shape `inject:",optional"`
}

func TestMultiBind(t *testing.T) {
	t.Run("Using multiple interface implementation", func(t *testing.T) {
		assert.NotPanics(t, func() {
			injector, err := NewInjector(
				Provide(func() *Rectangle {
					return &Rectangle{}
				}, As(Type[Shape]())),
				Provide(func() *Square {
					return &Square{}
				}, As(Type[Shape]())),
			)
			assert.Nil(t, err)
			ctx := context.Background()
			err = injector.Invoke(ctx, func(shapes []Shape) {
				var names []string
				for _, shape := range shapes {
					names = append(names, shape.Name())
				}
				assert.Contains(t, names, "square")
				assert.Contains(t, names, "rectangle")
			})
			assert.Nil(t, err)
		})
	})

	t.Run("Should not throw error if not found and optional", func(t *testing.T) {
		assert.NotPanics(t, func() {
			injector, err := NewInjector()
			assert.Nil(t, err)
			ctx := context.Background()
			err = injector.Invoke(ctx, func(params MultiBindOptionalInvokeParams) {
				assert.Empty(t, params.Shapes)
			})
			assert.Nil(t, err)
		})
	})

	t.Run("Should throw error if not found and not optional", func(t *testing.T) {
		assert.NotPanics(t, func() {
			injector, err := NewInjector()
			assert.Nil(t, err)
			ctx := context.Background()
			err = injector.Invoke(ctx, func(_ []Shape) {
				assert.Fail(t, "should not be reached")
			})
			assert.NotNil(t, err)
			var expectedErrorType *injectionError
			assert.ErrorAs(t, err, &expectedErrorType)
			assert.Equal(t, "failed to call invokation function: failed to resolve function argument #0: "+
				"Got error while resolving type goinject.Shape (with annotation \"\"):\n"+
				"did not found binding, expected at least one", err.Error())
		})
	})
}

type WithProvider struct {
	provider Provider[*WithRefCount]
}

type WithProviderParam struct {
	Params
	Provider Provider[*WithRefCount] `inject:",optional"`
}

func TestProvider(t *testing.T) {
	assert.NotPanics(t, func() {
		t.Run("Get from provider should re-ask scope (with per-lookup)", func(t *testing.T) {
			refCount := 0
			injector, rootError := NewInjector(
				Provide(func() *WithRefCount {
					res := &WithRefCount{refCount: refCount}
					refCount++
					return res
				}, In(PerLookUp)),
				Provide(func(p Provider[*WithRefCount]) *WithProvider {
					return &WithProvider{
						provider: p,
					}
				}),
			)
			assert.Nil(t, rootError)
			ctx := context.Background()

			rootError = injector.Invoke(ctx, func(w *WithProvider) {
				ref1, err := w.provider(ctx)
				assert.Nil(t, err)
				ref2, err := w.provider(ctx)
				assert.Nil(t, err)
				assert.NotEqual(t, ref2, ref1)
				assert.Equal(t, 0, ref1.refCount)
				assert.Equal(t, 1, ref2.refCount)
			})
			assert.Nil(t, rootError)
		})

		t.Run("Get from provider should re-ask scope (with singleton)", func(t *testing.T) {
			refCount := 0
			injector, rootError := NewInjector(
				Provide(func() *WithRefCount {
					res := &WithRefCount{refCount: refCount}
					refCount++
					return res
				}, In(Singleton)),
				Provide(func(p Provider[*WithRefCount]) *WithProvider {
					return &WithProvider{
						provider: p,
					}
				}),
			)
			assert.Nil(t, rootError)
			ctx := context.Background()

			rootError = injector.Invoke(ctx, func(w *WithProvider) {
				ref1, err := w.provider(ctx)
				assert.Nil(t, err)
				ref2, err := w.provider(ctx)
				assert.Nil(t, err)
				assert.Same(t, ref2, ref1)
			})
			assert.Nil(t, rootError)
		})

		t.Run("Provider with optional should return zero value if not present", func(t *testing.T) {
			injector, rootError := NewInjector()
			assert.Nil(t, rootError)
			ctx := context.Background()

			rootError = injector.Invoke(ctx, func(w WithProviderParam) {
				ref, err := w.Provider(ctx)
				assert.Nil(t, err)
				assert.Nil(t, ref)
			})
			assert.Nil(t, rootError)
		})

		t.Run("Provider should return error", func(t *testing.T) {
			injector, rootError := NewInjector(
				Provide(func() (*WithRefCount, error) {
					return nil, fmt.Errorf("test error")
				}, In(PerLookUp)),
				Provide(func(p Provider[*WithRefCount]) *WithProvider {
					return &WithProvider{
						provider: p,
					}
				}, In(PerLookUp)),
			)
			assert.Nil(t, rootError)
			ctx := context.Background()

			rootError = injector.Invoke(ctx, func(w *WithProvider) {
				ref, err := w.provider(ctx)
				assert.Nil(t, ref)
				assert.NotNil(t, err)
				assert.Equal(t, "provider for type \"*goinject.WithRefCount\" returned error: test error", err.Error())
			})
			assert.Nil(t, rootError)
		})
	})
}

func TestConditional(t *testing.T) {
	t.Run("Test conditional env var should not register binding if no match", func(t *testing.T) {
		t.Setenv("TEST", "CASE-KO")
		injector, err := NewInjector(
			When(OnEnvironmentVariable("TEST", "CASE-OK", false),
				Provide(func() (*Parent, error) { return &Parent{}, nil }),
			),
		)
		assert.Nil(t, err)
		ctx := context.Background()
		err = injector.Invoke(ctx, func(_ *Parent) {
			assert.Fail(t, "inaccessible")
		})
		assert.NotNil(t, err)
		var expectedErrorType *injectionError
		assert.ErrorAs(t, err, &expectedErrorType)
		assert.Equal(t,
			"failed to call invokation function: failed to resolve function argument #0: "+
				"Got error while resolving type *goinject.Parent (with annotation \"\"):\ndid not found binding, "+
				"expected one",
			err.Error(),
		)
	})

	t.Run("Test conditional env var should register binding if match", func(t *testing.T) {
		t.Setenv("TEST", "CASE-OK")
		injector, err := NewInjector(
			When(OnEnvironmentVariable("TEST", "CASE-OK", false),
				Provide(func() (*Parent, error) { return &Parent{}, nil }),
			),
		)
		assert.Nil(t, err)
		ctx := context.Background()
		err = injector.Invoke(ctx, func(parent *Parent) {
			assert.NotNil(t, parent)
		})
		assert.Nil(t, err)
	})

	t.Run("Test conditional env var should register binding if no match but match missing", func(t *testing.T) {
		injector, err := NewInjector(
			When(OnEnvironmentVariable("TEST", "CASE-OO", true),
				Provide(func() (*Parent, error) { return &Parent{}, nil }),
			),
		)
		assert.Nil(t, err)
		ctx := context.Background()
		err = injector.Invoke(ctx, func(parent *Parent) {
			assert.NotNil(t, parent)
		})
		assert.Nil(t, err)
	})

	t.Run("Test When should return binding configuration errors", func(t *testing.T) {
		_, err := NewInjector(
			When(OnEnvironmentVariable("TEST", "CASE-OK", true),
				Provide(nil),
			),
		)
		assert.NotNil(t, err)
		assert.IsType(t, err, &injectorConfigurationError{})
		assert.Equal(t, "cannot accept nil provider", err.Error())
	})
}

func TestInvokeError(t *testing.T) {
	t.Run("Invoke should not accept nil", func(t *testing.T) {
		injector, err := NewInjector()
		assert.Nil(t, err)
		ctx := context.Background()
		err = injector.Invoke(ctx, nil)
		assert.NotNil(t, err)
		assert.IsType(t, err, &invalidInputError{})
		assert.Equal(t, "can't invoke on nil", err.Error())
	})

	t.Run("Invoke should only accept function", func(t *testing.T) {
		injector, err := NewInjector()
		assert.Nil(t, err)
		ctx := context.Background()
		err = injector.Invoke(ctx, true)
		assert.NotNil(t, err)
		assert.IsType(t, err, &invalidInputError{})
		assert.Equal(t, "can't invoke non-function true (type bool)", err.Error())
	})

	t.Run("Invoke should only accept function returning error", func(t *testing.T) {
		injector, err := NewInjector()
		assert.Nil(t, err)
		ctx := context.Background()
		err = injector.Invoke(ctx, func() *Parent { return nil })
		assert.NotNil(t, err)
		assert.IsType(t, err, &invalidInputError{})
		assert.Equal(t, "can't invoke on function whose return type is not error or no return type", err.Error())
	})

	t.Run("Invoke should return error if function return error", func(t *testing.T) {
		injector, err := NewInjector()
		assert.Nil(t, err)
		ctx := context.Background()
		invokationFnReturnedError := fmt.Errorf("returned error")
		err = injector.Invoke(ctx, func() error { return invokationFnReturnedError })
		assert.NotNil(t, err)
		assert.ErrorIs(t, err, invokationFnReturnedError)
	})
}

func TestInjectorConfigurationError(t *testing.T) {
	t.Run("Provide cannot accept nil", func(t *testing.T) {
		_, err := NewInjector(
			Provide(nil))
		assert.NotNil(t, err)
		assert.IsType(t, err, &injectorConfigurationError{})
		assert.Equal(t, "cannot accept nil provider", err.Error())
	})

	t.Run("Provider should use function as argument", func(t *testing.T) {
		_, err := NewInjector(
			Provide(true))
		assert.NotNil(t, err)
		assert.IsType(t, err, &injectorConfigurationError{})
		assert.Equal(t, "provider argument should be a function", err.Error())
	})

	t.Run("Provider function should return an instance", func(t *testing.T) {
		_, err := NewInjector(
			Provide(func() {}))
		assert.NotNil(t, err)
		assert.IsType(t, err, &injectorConfigurationError{})
		assert.Equal(t, "expected a function that return an instance and optionally an error", err.Error())
	})

	t.Run("Provider function cannot return multiple types (except error)", func(t *testing.T) {
		_, err := NewInjector(
			Provide(func() (*Parent, *Child) {
				return &Parent{}, &Child{}
			}))
		assert.NotNil(t, err)
		assert.IsType(t, err, &injectorConfigurationError{})
		assert.Equal(t, "second return type of provider should be an error", err.Error())
	})

	t.Run("Module should return nested errors", func(t *testing.T) {
		_, err := NewInjector(
			Module("test.Module",
				Provide(nil)),
		)
		assert.NotNil(t, err)
		assert.IsType(t, err, &injectorConfigurationError{})
		assert.Equal(t, "error while installing module test.Module:\ncannot accept nil provider", err.Error())
	})

	t.Run("As provider annotation should raise error if not assignable", func(t *testing.T) {
		_, err := NewInjector(
			Provide(func() *Parent {
				return &Parent{}
			}, As(Type[*Child]())),
		)
		assert.NotNil(t, err)
		assert.IsType(t, err, &injectorConfigurationError{})
		assert.Equal(t,
			"got error while configuring provider for provided type *goinject.Parent:\ncannot assign "+
				"*goinject.Parent to *goinject.Child as specified in As argument",
			err.Error(),
		)
	})

	t.Run("WithDestroy should raise an error if not a function", func(t *testing.T) {
		_, err := NewInjector(
			Provide(func() *Parent {
				return &Parent{}
			}, WithDestroy(true)),
		)
		assert.NotNil(t, err)
		assert.IsType(t, err, &injectorConfigurationError{})
		assert.Equal(t,
			"got error while configuring provider for provided type *goinject.Parent:\nargument of WithDestroy"+
				" must be a function with one argument returning void",
			err.Error(),
		)
	})

	t.Run("WithDestroy should raise an error if not a function of provided type", func(t *testing.T) {
		_, err := NewInjector(
			Provide(func() *Parent {
				return &Parent{}
			}, WithDestroy(func(_ *Child) {})),
		)
		assert.NotNil(t, err)
		assert.IsType(t, err, &injectorConfigurationError{})
		assert.Equal(t,
			"got error while configuring provider for provided type *goinject.Parent:\nargument of WithDestroy"+
				" must be a function with one argument returning void",
			err.Error(),
		)
	})

	t.Run("WithDestroy should raise an error if not a void function of provided type", func(t *testing.T) {
		_, err := NewInjector(
			Provide(func() *Parent {
				return &Parent{}
			}, WithDestroy(func(_ *Parent) error {
				return nil
			})),
		)
		assert.NotNil(t, err)
		assert.IsType(t, err, &injectorConfigurationError{})
		assert.Equal(t,
			"got error while configuring provider for provided type *goinject.Parent:\nargument of WithDestroy "+
				"must be a function with one argument returning void", err.Error(),
		)
	})
}
