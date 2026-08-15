package pool

import (
	"context"
	"runtime/debug"
	"sync"

	"github.com/pkg/errors"
)

// errTaskPanic is returned when a submitted task panics; the panic value and
// stack trace are appended to the error message for diagnostics.
var errTaskPanic = errors.New("panic in pool task")

// Pool runs tasks with a bounded concurrency level.
type Pool struct {
	ctx       context.Context
	cancel    context.CancelCauseFunc
	semaphore chan struct{}
}

// New creates a pool with max concurrent workers. max == 0 means unlimited.
// The pool's context is derived from parent; cancelling parent cancels the
// pool and all of its groups.
func New(maxConcurrency uint, parent context.Context) *Pool {
	var semaphore chan struct{}

	if maxConcurrency > 0 {
		semaphore = make(chan struct{}, int(maxConcurrency))
	}

	ctx, cancel := context.WithCancelCause(parent)

	return &Pool{ctx: ctx, cancel: cancel, semaphore: semaphore}
}

// NewGroup creates a task group sharing this pool's concurrency limit and context.
func (p *Pool) NewGroup() *Group {
	ctx, cancel := context.WithCancelCause(p.ctx)

	return &Group{pool: p, ctx: ctx, cancel: cancel}
}

// Stop stops the pool: no new tasks are accepted and the pool context is
// cancelled. Already-running tasks are allowed to finish.
func (p *Pool) Stop() {
	p.cancel(context.Canceled)
}

// Group collects tasks submitted together and reports the first error.
type Group struct {
	pool   *Pool
	ctx    context.Context
	cancel context.CancelCauseFunc

	wg         sync.WaitGroup
	mu         sync.Mutex
	firstError error
}

// Context returns the group's context, cancelled when the pool's parent
// context is cancelled or when any submitted task returns an error.
func (g *Group) Context() context.Context { return g.ctx }

// SubmitErr schedules task for execution. task runs concurrently with other
// submitted tasks, bounded by the pool's concurrency limit. If the group's
// context is already cancelled, task is not invoked (matching pond behaviour:
// cancelled groups skip queued tasks).
func (g *Group) SubmitErr(task func() error) {
	g.wg.Go(func() {
		err := g.ctx.Err()
		if err != nil {
			g.record(err)

			return
		}

		if g.pool.semaphore != nil {
			select {
			case g.pool.semaphore <- struct{}{}:
				defer func() { <-g.pool.semaphore }()
			case <-g.ctx.Done():
				g.record(g.ctx.Err())

				return
			}
		}

		err = g.runTask(task)
		if err != nil {
			g.record(err)
			g.cancel(err)
		}
	})
}

// Wait blocks until all submitted tasks complete and returns the first
// non-nil error, or the group's context error if it was cancelled.
func (g *Group) Wait() error {
	g.wg.Wait()

	g.mu.Lock()
	first := g.firstError
	g.mu.Unlock()

	if first != nil {
		return first
	}

	err := g.ctx.Err()
	if err != nil {
		return errors.Wrap(err, "pool group cancelled")
	}

	return nil
}

// runTask invokes task, converting any panic into an error so a panicking
// task fails the group instead of crashing the process (pond recovers panics
// by default as well).
func (g *Group) runTask(task func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.Wrapf(errTaskPanic, "%v\n%s", recovered, debug.Stack())
		}
	}()

	return task()
}

func (g *Group) record(err error) {
	g.mu.Lock()
	if g.firstError == nil {
		g.firstError = err
	}
	g.mu.Unlock()
}
