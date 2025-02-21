# go-inject

## Description

`go-inject` is a dependency injection library that support contextual scope.

## Example

```go
package myapp

import (
	"context"

	"github.com/illuin-tech/goinject"
)

type key int
const myScopeKey key = 0

const MyScope = "MyScope"

// define function to declare your own scope in context
func WithMyScopeEnabled(ctx context.Context) context.Context {
	return goinject.WithContextualScopeEnabled(ctx, myScopeKey)
}

func ShutdownMyContextScoped(ctx context.Context) {
	goinject.ShutdownContextualScope(ctx, myScopeKey)
}

// define injection modules
var Module = goinject.Module("myModule",
	goinject.RegisterScope(MyScope, goinject.NewContextualScope(myScopeKey)),
	goinject.Provide(func() string {
		return "Hello world from scope"
    }, goinject.In(MyScope)),
)

func main() {
    ctx := context.Background()
	
	// enable scope
	ctx = WithMyScopeEnabled(ctx)
	defer ShutdownMyContextScoped(ctx)
	
	injector, _ := goinject.NewInjector(Module)
	_ = injector.Invoke(ctx, func(hello string) {
	    println(hello)	
	})
}
```