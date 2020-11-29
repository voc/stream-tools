package main

import (
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

// Pool is a worker group that runs a number of tasks at a
// configured concurrency.
type Downloader struct {
	client *http.Client
}

var transport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   3 * time.Second,
		KeepAlive: 3 * time.Second,
		DualStack: true,
	}).DialContext,
	ForceAttemptHTTP2:     false,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

func NewDownloader(timeout time.Duration, tasks <-chan *Task, limiter <-chan struct{}, results chan<- *Result) *Downloader {
	d := &Downloader{
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
	go func() {
		for task := range tasks {
			// Skip old tasks
			select {
			case <-task.Context.Done():
				continue
			case <-limiter:
			}
			result := d.process(task)
			results <- result
		}
	}()
	return d
}

func (d *Downloader) process(task *Task) *Result {
	var body io.ReadCloser
	result := &Result{}
	req, err := http.NewRequestWithContext(task.Context, "GET", task.URL, EOFReader{})
	req.GetBody = func() (io.ReadCloser, error) {
		if body == nil {
			return ioutil.NopCloser(EOFReader{}), nil
		}
		return body, nil
	}
	if err != nil {
		result.Err = err
		return result
	}
	resp, err := d.client.Do(req)
	if err != nil {
		result.Err = err
		return result
	}
	defer resp.Body.Close()
	body = resp.Body
	bytes, err := io.Copy(ioutil.Discard, resp.Body)
	result.Loaded = bytes
	result.Err = err
	return result
}

type EOFReader struct{}

func (e EOFReader) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}
