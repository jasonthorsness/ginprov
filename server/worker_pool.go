package server

import (
	"context"
	"sync"
)

// WorkerPool is a fixed-size pool of workers for arbitrary work. Incoming work is enqueued in a FIFO channel which
// the individual workers pull from.
type WorkerPool struct {
	workCh chan workWrapper
	wg     sync.WaitGroup
}

// NewWorkerPool starts a new worker pool with the specified number of workers and work channel capacity.
// Arguments must both be positive numbers.
func NewWorkerPool(numWorkers, workChannelCapacity int) *WorkerPool {
	w := &WorkerPool{make(chan workWrapper, workChannelCapacity), sync.WaitGroup{}}

	for range numWorkers {
		w.wg.Add(1)

		go w.workerLoop()
	}

	return w
}

// DoWork queues work to the pool for asynchronous execution.
// 1. DoWork returns immediately often but not necessarily before do() is called.
// 2. If the work queue is full, do() is not called and the function returns false.
// 3. Otherwise, do() will be called exactly once.
// 4. do() must not panic, if it does the panic will escape and the program will terminate.
func DoWork[TWork any](
	ctx context.Context,
	w *WorkerPool,
	work TWork,
	do func(context.Context, TWork),
) bool {
	return trySend(w.workCh, workWrapper{ctx, wrapDo(do), work})
}

// Close stops the pool from accepting work and blocks until do returns for all pending work.
// It always returns nil but has error signature to conform to io.Closer.
func (w *WorkerPool) Close() error {
	close(w.workCh)
	w.wg.Wait()

	return nil
}

func wrapDo[TWork any](do func(context.Context, TWork)) func(context.Context, any) {
	return func(ctx context.Context, work any) {
		do(ctx, work.(TWork)) //nolint:forcetypeassert // constrained by generic
	}
}

type workWrapper struct {
	ctx  context.Context
	do   func(context.Context, any)
	work any
}

func (w *WorkerPool) workerLoop() {
	defer w.wg.Done()

	for {
		r, ok := <-w.workCh
		if !ok {
			break
		}

		r.do(r.ctx, r.work)
	}
}

func trySend[T any](ch chan<- T, v T) bool {
	select {
	case ch <- v:
		return true
	default:
		return false
	}
}
