package main

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

// Downloader struct
type Downloader struct {
	timeout time.Duration
	request *http.Request
}

func NewDownloader(timeout time.Duration, authFunc SetAuthFunc) *Downloader {
	req, _ := http.NewRequest("GET", "", nil)
	authFunc(req)
	d := &Downloader{
		timeout: timeout,
		request: req,
	}
	return d
}

func (d *Downloader) RunWorkers(ctx context.Context, numWorkers uint, tasks <-chan *Task, limiter <-chan struct{}, results chan<- *Result) {
	for i := uint(0); i < numWorkers; i++ {
		go d.run(ctx, tasks, limiter, results)
	}
}

func (d *Downloader) run(ctx context.Context, tasks <-chan *Task, limiter <-chan struct{}, results chan<- *Result) {
	client := &http.Client{
		Timeout: d.timeout,
		// Transport: transport,
	}

	// fetch first task
	task := <-tasks
	for {
		// limit download
		select {
		case <-ctx.Done():
			return
		case <-limiter:
		}
		result := d.process(client, task)
		results <- result

		// fetch new task or reuse previous (playlist too short)
		select {
		case <-ctx.Done():
			return
		case task = <-tasks:
		default:
		}
	}
}

func (d *Downloader) process(client *http.Client, task *Task) *Result {
	result := &Result{}
	// req, err := http.NewRequest("GET", task.URL, nil)
	req := cloneRequest(d.request)
	req.URL = task.URL
	resp, err := client.Do(req)
	if err == nil {
		result.Size = resp.ContentLength
		result.Code = resp.StatusCode
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}
	result.Err = err
	return result
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}
