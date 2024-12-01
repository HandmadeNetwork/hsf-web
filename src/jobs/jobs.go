package jobs

import (
	"context"
	"time"
)

type Job struct {
	Name string
	c    chan struct{}
}

func New(name string) *Job {
	return &Job{
		Name: name,
		c:    make(chan struct{}),
	}
}

func Finished(name string) *Job {
	job := New(name)
	return job.Finish()
}

func (j *Job) Finish() *Job {
	close(j.c)
	return j
}

type Tracker struct {
	ctx         context.Context
	cancelFunc  context.CancelFunc
	doneChan    chan struct{}
	pendingJobs []*Job
}

func NewTracker(ctx context.Context) *Tracker {
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	return &Tracker{
		ctx:        cancelCtx,
		cancelFunc: cancelFunc,
		doneChan:   make(chan struct{}),
	}
}

func (tracker *Tracker) Add(job ...*Job) {
	tracker.pendingJobs = append(tracker.pendingJobs, job...)
}

func (tracker *Tracker) Finish(timeoutDur time.Duration) []string {
	allDoneChan := make(chan struct{})
	tracker.cancelFunc()
	timer := time.NewTimer(timeoutDur)

	go func() {
		for _, p := range tracker.pendingJobs {
			<-p.c
		}
		close(allDoneChan)
	}()
	select {
	case <-timer.C:
		return tracker.ListUnfinished()
	case <-allDoneChan:
		return nil
	}
}

func (tracker *Tracker) ListUnfinished() []string {
	unfinished := []string{}
	for _, p := range tracker.pendingJobs {
		select {
		case <-p.c:
			continue
		default:
			unfinished = append(unfinished, p.Name)
		}
	}
	return unfinished
}

func (tracker *Tracker) Deadline() (deadline time.Time, ok bool) {
	return tracker.ctx.Deadline()
}

func (tracker *Tracker) Done() <-chan struct{} {
	return tracker.ctx.Done()
}

func (tracker *Tracker) Err() error {
	return tracker.ctx.Err()
}

func (tracker *Tracker) Value(key any) any {
	return tracker.ctx.Value(key)
}
