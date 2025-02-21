package goinject

import (
	"context"
	"reflect"
	"sync"
)

// Instance is the return type for Scope ResolveBinding method.
// It is used to hidde the usage of reflect.Value in the public API
type Instance reflect.Value

type instanceRegistry struct {
	mu                 sync.Mutex                 // lock guarding instanceLock
	instanceLock       map[*binding]*sync.RWMutex // lock guarding instances
	instances          sync.Map
	destroyMethodsLock sync.Mutex
	destroyMethods     []func()
}

func (r *instanceRegistry) resolveBinding(
	binding *binding,
	instanceCreator func() (Instance, error),
) (Instance, error) {
	r.mu.Lock()

	if l, ok := r.instanceLock[binding]; ok {
		r.mu.Unlock()
		l.RLock()
		defer l.RUnlock()

		instance, _ := r.instances.Load(binding)
		return instance.(Instance), nil
	}

	r.instanceLock[binding] = new(sync.RWMutex)
	l := r.instanceLock[binding]
	l.Lock()
	r.mu.Unlock()

	instance, err := instanceCreator()
	r.instances.Store(binding, instance)

	defer l.Unlock()

	return instance, err
}

func (r *instanceRegistry) registerDestructionCallback(
	destroyCallback func(),
) {
	r.destroyMethodsLock.Lock()
	defer r.destroyMethodsLock.Unlock()
	r.destroyMethods = append(r.destroyMethods, destroyCallback)
}

func (r *instanceRegistry) shutdown() {
	r.destroyMethodsLock.Lock()
	defer r.destroyMethodsLock.Unlock()

	for i := len(r.destroyMethods) - 1; i >= 0; i-- {
		r.destroyMethods[i]()
	}

	r.destroyMethods = []func(){}
}

func newInstanceRegistry() *instanceRegistry {
	return &instanceRegistry{
		instanceLock:   make(map[*binding]*sync.RWMutex),
		destroyMethods: []func(){},
	}
}

// Scope defines a scope's behaviour
type Scope interface {
	// ResolveBinding resolve a dependency injection context for current scope
	ResolveBinding(
		ctx context.Context,
		binding *binding,
		instanceCreator func() (Instance, error),
	) (Instance, error)

	// RegisterDestructionCallback register a destruction callback. It is the responsibility of the Scope to call
	// this callback when destroying the Scope
	RegisterDestructionCallback(
		ctx context.Context,
		destroyCallback func(),
	)
}

const PerLookUp = "inject.PerLookUp"

// perLookUpScope is a Scope that return a new instance when requested
type perLookUpScope struct {
}

var _ Scope = new(perLookUpScope)

func newPerLookUpScope() Scope {
	return &perLookUpScope{}
}

func (s *perLookUpScope) ResolveBinding(
	_ context.Context,
	_ *binding,
	instanceCreator func() (Instance, error),
) (Instance, error) {
	return instanceCreator()
}

func (s *perLookUpScope) RegisterDestructionCallback(
	_ context.Context,
	_ func(),
) {
	// nothing to do, per lookup provided need to close destroy method themselves
}

const Singleton = "inject.Singleton"

// singletonScope is our Scope to handle Singletons
type singletonScope struct {
	instanceRegistry *instanceRegistry
}

var _ Scope = new(singletonScope)

func newSingletonScope() *singletonScope {
	return &singletonScope{
		instanceRegistry: newInstanceRegistry(),
	}
}

func (s *singletonScope) ResolveBinding(
	_ context.Context,
	binding *binding,
	instanceCreator func() (Instance, error),
) (Instance, error) {
	return s.instanceRegistry.resolveBinding(binding, instanceCreator)
}

func (s *singletonScope) RegisterDestructionCallback(
	_ context.Context,
	destroyCallback func(),
) {
	s.instanceRegistry.registerDestructionCallback(destroyCallback)
}

func (s *singletonScope) Shutdown() {
	s.instanceRegistry.shutdown()
}

// contextualScope is an abstract scope to handle context attached scoped (request, session, ...)
type contextualScope struct {
	key any
}

var _ Scope = new(contextualScope)

func (s *contextualScope) ResolveBinding(
	ctx context.Context,
	binding *binding,
	instanceCreator func() (Instance, error),
) (Instance, error) {
	if ctx == nil {
		return Instance{}, newContextScopedNotActiveError()
	}
	scopeHolder, ok := ctx.Value(s.key).(*instanceRegistry)
	if !ok {
		return Instance{}, newContextScopedNotActiveError()
	}
	return scopeHolder.resolveBinding(binding, instanceCreator)
}

func (s *contextualScope) RegisterDestructionCallback(
	ctx context.Context,
	destroyCallback func(),
) {
	if scopeHolder, ok := ctx.Value(s.key).(*instanceRegistry); ok {
		scopeHolder.registerDestructionCallback(destroyCallback)
	}
}

func NewContextualScope(key any) Scope {
	return &contextualScope{
		key: key,
	}
}

func WithContextualScopeEnabled(ctx context.Context, key any) context.Context {
	return context.WithValue(ctx, key, newInstanceRegistry())
}

func ShutdownContextualScope(ctx context.Context, key any) {
	holder, ok := ctx.Value(key).(*instanceRegistry)
	if ok {
		holder.shutdown()
	}
}
