// Package stream provides coordination
// for readers and writers of live job output.
package stream

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/wepogo/testbot"
)

type key struct {
	id  string
	job testbot.Job
}

var (
	mu   sync.Mutex
	cond = sync.Cond{L: &mu}
	req  = map[string][]testbot.Job{}
	ret  = map[key][]chan io.ReadCloser{}
)

// Get gets the output of job.
// The only errors it returns are from ctx.
//
// The caller is responsible for closing the returned io.ReadCloser.
func Get(ctx context.Context, id string, job testbot.Job) (io.ReadCloser, error) {
	k := key{id, job}
	c := make(chan io.ReadCloser)

	mu.Lock()
	ret[k] = append(ret[k], c)
	req[id] = append(req[id], job)
	mu.Unlock()
	cond.Broadcast()

	go func() {
		<-ctx.Done()
		mu.Lock()
		for i, c1 := range ret[k] {
			if c1 == c {
				ret[k] = append(ret[k][:i], ret[k][i+1:]...)
				rmjob(id, job)
				close(c)
				break
			}
		}
		mu.Unlock()
	}()

	r, ok := <-c
	if !ok {
		return nil, ctx.Err()
	}
	return r, nil
}

// Poll waits until a request for id is available.
// If ctx becomes done before a request is available,
// Poll returns the zero Job.
func Poll(ctx context.Context, id string) testbot.Job {
	// When ctx is done, wake ourselves
	// (plus, unfortunately, everyone else too).
	go func() { <-ctx.Done(); cond.Broadcast() }()
	mu.Lock()
	defer mu.Unlock()
	for len(req[id]) == 0 {
		cond.Wait()
		if ctx.Err() != nil {
			return testbot.Job{}
		}
	}
	return req[id][0]
}

// Send sends r to a matching call to Get, if one is available.
// If successful, it reads from r, closes r, and then returns.
func Send(id string, job testbot.Job, r io.ReadCloser) {
	k := key{id, job}

	mu.Lock()
	a := ret[k]
	if len(a) == 0 {
		mu.Unlock()
		return
	}
	c := a[0]
	ret[k] = a[1:]
	rmjob(id, job)
	mu.Unlock()

	done := make(chan struct{})
	c <- &doneReadCloser{r, done}
	<-done
}

type doneReadCloser struct {
	io.ReadCloser
	c chan struct{}
}

func (dc *doneReadCloser) Close() error {
	if dc.c == nil {
		return errors.New("already closed")
	}
	err := dc.ReadCloser.Close()
	close(dc.c)
	dc.c = nil
	return err
}

// caller must hold mu
func rmjob(id string, job testbot.Job) {
	for i, j := range req[id] {
		if j == job {
			req[id] = append(req[id][:i], req[id][i+1:]...)
			break
		}
	}
}
