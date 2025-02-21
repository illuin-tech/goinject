package goinject

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type sessionScopeKey int

const sessionScopeKeyVal sessionScopeKey = 0

type requestScopeKey int

const requestScopeKeyVal requestScopeKey = 0

type Request struct {
	ID int
}

type Session struct {
	ID int
}

type ContextualScopesParams struct {
	Params
	Request *Request `inject:""`
}

type ctxKey int

const requestKey ctxKey = iota

func TestContextualScopesUsingContextValue(t *testing.T) {
	notAwareContextError := errors.New("not running in value aware context")
	assert.NotPanics(t, func() {
		injector, err := NewInjector(
			Module("contextualScopeTest",
				RegisterScope("request", NewContextualScope(requestScopeKeyVal)),
				Provide(func(ctx InvocationContext) (*Request, error) {
					if r, ok := ctx.Value(requestKey).(*Request); ok {
						return r, nil
					} else {
						return nil, notAwareContextError
					}
				}, In("request")),
			),
		)
		assert.Nil(t, err)

		t.Run("Provider should be able to provide from InvocationContext", func(t *testing.T) {
			ctx := context.Background()
			requestCtx := WithContextualScopeEnabled(
				context.WithValue(ctx, requestKey, &Request{ID: 42}),
				requestScopeKeyVal,
			)
			defer ShutdownContextualScope(requestCtx, requestScopeKeyVal)

			invokeErr := injector.Invoke(requestCtx, func(r *Request) {
				assert.Equal(t, 42, r.ID)
			})
			assert.Nil(t, invokeErr)
		})

		t.Run("Provider should be able to provide from InvocationContext with error", func(t *testing.T) {
			ctx := context.Background()
			requestCtx := WithContextualScopeEnabled(
				ctx,
				requestScopeKeyVal,
			)
			defer ShutdownContextualScope(requestCtx, requestScopeKeyVal)

			invokeErr := injector.Invoke(requestCtx, func(*Request) {
				assert.Fail(t, "should not be called")
			})
			assert.ErrorIs(t, invokeErr, notAwareContextError)
		})
	})
}

func TestContextualScopes(t *testing.T) {
	assert.NotPanics(t, func() {
		count := 0
		injector, err := NewInjector(
			Module("contextualScopeTest",
				RegisterScope("request", NewContextualScope(requestScopeKeyVal)),
				RegisterScope("session", NewContextualScope(sessionScopeKeyVal)),
				Provide(func() *Request {
					res := &Request{ID: count}
					count++
					return res
				}, In("request")),
				Provide(func() *Session {
					res := &Session{ID: count}
					count++
					return res
				}, In("session")),
			),
		)
		assert.Nil(t, err)

		ctx := context.Background()

		t.Run("Contextual scope should return error if not active", func(t *testing.T) {
			err = injector.Invoke(ctx, func(_ *Request) {
				assert.Fail(t, "Should not be reached")
			})
			assert.True(t, errors.Is(err, &contextScopedNotActiveError{}))
		})

		t.Run("Contextual scope should return error if not active (using Params)", func(t *testing.T) {
			err = injector.Invoke(ctx, func(_ ContextualScopesParams) {
				assert.Fail(t, "Should not be reached")
			})
			assert.True(t, errors.Is(err, &contextScopedNotActiveError{}))
		})

		var sessionID int
		var sessionID2 int

		t.Run("Test session with multiple request should keep same session scope but different request scope",
			func(t *testing.T) {
				sessionCtx := WithContextualScopeEnabled(ctx, sessionScopeKeyVal)
				defer ShutdownContextualScope(sessionCtx, sessionScopeKeyVal)

				var request1ID int
				var request2ID int
				var sessionIDBis int

				t.Run("Test request 1", func(t *testing.T) {
					requestCtx := WithContextualScopeEnabled(sessionCtx, requestScopeKeyVal)
					defer ShutdownContextualScope(requestCtx, requestScopeKeyVal)

					err := injector.Invoke(requestCtx, func(session *Session, request *Request) {
						sessionID = session.ID
						request1ID = request.ID
					})
					assert.Nil(t, err)
				})

				t.Run("Test request 2", func(t *testing.T) {
					requestCtx := WithContextualScopeEnabled(sessionCtx, requestScopeKeyVal)
					defer ShutdownContextualScope(requestCtx, requestScopeKeyVal)

					err := injector.Invoke(requestCtx, func(session *Session, request *Request) {
						sessionIDBis = session.ID
						request2ID = request.ID
					})
					assert.Nil(t, err)
				})

				assert.NotZero(t, request1ID)
				assert.NotZero(t, request2ID)
				assert.NotEqual(t, request2ID, request1ID)

				assert.Equal(t, sessionID, sessionIDBis)
			})

		t.Run("Test session 2 (without request scope)", func(t *testing.T) {
			sessionCtx := WithContextualScopeEnabled(ctx, sessionScopeKeyVal)
			defer ShutdownContextualScope(sessionCtx, sessionScopeKeyVal)

			err := injector.Invoke(sessionCtx, func(session *Session) {
				sessionID2 = session.ID
			})
			assert.Nil(t, err)
		})

		assert.NotEqual(t, sessionID, sessionID2)
	})
}

func TestContextualScopeDestroy(t *testing.T) {
	assert.NotPanics(t, func() {
		count := 0
		injector, err := NewInjector(
			Module("contextualScopeTest",
				RegisterScope("session", NewContextualScope(sessionScopeKeyVal)),
				Provide(func() *Session {
					res := &Session{ID: count}
					count++
					return res
				}, In("session"), WithDestroy(func(_ *Session) {
					count--
				})),
			),
		)
		assert.Nil(t, err)
		ctx := context.Background()

		t.Run("Run session", func(t *testing.T) {
			sessionCtx := WithContextualScopeEnabled(ctx, sessionScopeKeyVal)
			defer ShutdownContextualScope(sessionCtx, sessionScopeKeyVal)

			err := injector.Invoke(sessionCtx, func(_ *Session) {
				assert.Equal(t, 1, count)
			})
			assert.Nil(t, err)
		})

		assert.Equal(t, 0, count)
	})
}

type SingletonInjectee struct {
	ID int
}

func TestSingletonScope(t *testing.T) {
	count := 0
	assert.NotPanics(t, func() {
		injector, err := NewInjector(
			Provide(func() *SingletonInjectee {
				res := &SingletonInjectee{ID: count}
				count++
				return res
			}, In(Singleton)),
		)
		assert.Nil(t, err)

		ctx := context.Background()
		var fetch1 *SingletonInjectee
		var fetch2 *SingletonInjectee
		err = injector.Invoke(ctx, func(s *SingletonInjectee) {
			fetch1 = s
		})
		assert.Nil(t, err)
		err = injector.Invoke(ctx, func(s *SingletonInjectee) {
			fetch2 = s
		})
		assert.Nil(t, err)
		assert.NotNil(t, fetch1)
		assert.NotNil(t, fetch2)
		assert.Same(t, fetch1, fetch2)
	})
}

type PerLookUpInjectee struct {
	ID int
}

func TestPerLookUpScope(t *testing.T) {
	assert.NotPanics(t, func() {
		t.Run("Should return new instance on each request", func(t *testing.T) {
			count := 0
			injector, err := NewInjector(
				Provide(func() *PerLookUpInjectee {
					res := &PerLookUpInjectee{ID: count}
					count++
					return res
				}, In(PerLookUp)),
			)
			assert.Nil(t, err)

			ctx := context.Background()
			var fetch1 *PerLookUpInjectee
			var fetch2 *PerLookUpInjectee
			err = injector.Invoke(ctx, func(s *PerLookUpInjectee) {
				fetch1 = s
			})
			assert.Nil(t, err)
			err = injector.Invoke(ctx, func(s *PerLookUpInjectee) {
				fetch2 = s
			})
			assert.Nil(t, err)
			assert.NotNil(t, fetch1)
			assert.NotNil(t, fetch2)
			assert.NotEqual(t, fetch1, fetch2)
		})

		t.Run("Should ignore destroy instance methods", func(t *testing.T) {
			count := 0
			injector, err := NewInjector(
				Provide(func() *PerLookUpInjectee {
					res := &PerLookUpInjectee{ID: count}
					count++
					return res
				}, In(PerLookUp), WithDestroy(func(_ *PerLookUpInjectee) { count-- })),
			)
			assert.Nil(t, err)

			ctx := context.Background()
			err = injector.Invoke(ctx, func(s *PerLookUpInjectee) {
				assert.Equal(t, 0, s.ID)
				assert.Equal(t, 1, count)
			})
			assert.Nil(t, err)
			assert.Equal(t, 1, count)
			injector.Shutdown()
			assert.Equal(t, 1, count)
		})
	})
}
