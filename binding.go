package goinject

import (
	"context"
	"fmt"
	"reflect"
)

// binding defines a type mapped to a more concrete type
type binding struct {
	typeof        reflect.Type
	provider      reflect.Value
	providedType  reflect.Type
	annotatedWith string
	scope         string
	destroyMethod func(value reflect.Value)
}

func (b *binding) create(ctx context.Context, injector *Injector) (reflect.Value, error) {
	res, err := injector.callFunctionWithArgumentInstance(ctx, b.provider)
	if err != nil {
		return reflect.Value{},
			fmt.Errorf("failed to call provider function for type %q: %w", b.providedType.String(), err)
	}
	if b.provider.Type().NumOut() == 2 {
		errValue := res[1].Interface()
		if errValue != nil {
			err, _ = errValue.(error)
		}
	}
	if err != nil {
		return res[0], fmt.Errorf("provider for type %q returned error: %w", b.providedType.String(), err)
	} else {
		return res[0], nil
	}
}
