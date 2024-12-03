package gobase

import (
	"context"
	"fmt"
	"time"

	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

// Context passed to the operation func will tell it is cancelled if queue is stopping
type Op func(context.Context)

// Queue is synchronous operations pool used to ensure that at a given time moment only one database read/write operation is exec.
// There is no need in async operations in this project.
// RPC request handlers and telegram message handlers both end up in a shared queue of operations.
// A minimum level of consistency is then guaranteed.
type Queue struct {
	name   string
	logger Logger
	ctx    context.Context
	cancel context.CancelFunc
	op     chan Op
}

// Makes new Queue (unintialized)
// Without Initialize, Enqueue takes up to to [backlog] operations before blocked.
// [backlog] defines number of operations pre-scheduled (pending) in queue, a non-zero value will lead to losing some if queue is Stopped
func NewQueue(name string, logger Logger, backlog int) *Queue {
	return &Queue{
		name:   name,
		logger: logger,
		op:     make(chan Op, backlog),
	}
}

// Create queue context (cancellable) for Run() goroutine
// Initializing queue must be followed by spawning Run() goroutine.
func (q *Queue) Initialize(ctx context.Context) {
	q.logger.Message(gelf.LOG_DEBUG, "queue", fmt.Sprintf("%s Queue::Initialize", q.name))

	q.ctx, q.cancel = context.WithCancel(ctx)
}

// IsReady tests if queue is intiailized and was not stopped
func (q *Queue) IsReady() bool {
	return q.ctx != nil && q.cancel != nil && q.op != nil
}

// Stop iteration inside Run() loop, preventing executing further queued operations.
// Pending operations on queue are lost (if non-zero backlog used)
// Some operations including running one will not be interrupted and will proceed even after call.
// Context passed to the operation func will tell it is cancelled if queue is stopping
// TODO: block before Run() is exited?
func (q *Queue) Stop() {
	q.logger.Message(gelf.LOG_DEBUG, "queue", fmt.Sprintf("%s Queue::Stop", q.name))
	q.cancel()
	close(q.op)
	q.ctx = nil
	q.cancel = nil
}

// Goroutine that performs all future operations in order.
func (q *Queue) Run() {
	q.logger.Message(gelf.LOG_DEBUG, "queue", fmt.Sprintf("%s Queue::Run", q.name))

	defer q.logger.Message(gelf.LOG_WARNING, "queue", fmt.Sprintf("%s Queue::Run end", q.name))

	for {
		select {
		case op := <-q.op:
			if op == nil { // normally channel termination
				return
			}

			// Install panic handler with logging on this thread/goroutine
			defer LogPanic(q.logger, "queue")

			op(q.ctx)
		case <-q.ctx.Done():
			return
		}
	}
}

// Push operation to be executed after others queued before.
// May block if queue blocking (is full)
func (q *Queue) Enqueue(op Op) {
	q.op <- op
}

// Push operation to be executed after others queued before.
// This method will block until the operation finishes.
// Operation won't run if given context is cancelled
// Return value is true when the operation was finished and returned.
func (q *Queue) Join(ctx context.Context, op Op) bool {
	c := make(chan bool)
	defer close(c)

	// FIXME: add select for context cancellation

	q.op <- func(ctx context.Context) {
		op(ctx)
		c <- true
	}

	return <-c
}

// Push operation to be executed after others queued before.
// This method will block until the operation finishes.
// Operation won't run if given context is cancelled
// Operation won't run if waiting for queue is longer than the startTimeout
// Return value is true when the operation was finished and returned.
func (q *Queue) JoinTimeout(ctx context.Context, startTimeout time.Duration, op Op) bool {
	c := make(chan bool)
	defer close(c)

	started := time.Now()

	// FIXME: add select for context cancellation, and Ticker for timeout

	q.op <- func(ctx context.Context) {
		if time.Since(started) >= startTimeout {
			c <- false
		} else {
			op(ctx)
			c <- true
		}
	}

	return <-c
}
