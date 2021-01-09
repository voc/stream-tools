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
	client *http.Client
}

func NewDownloader(ctx context.Context, timeout time.Duration, tasks <-chan *Task, limiter <-chan struct{}, results chan<- *Result) *Downloader {
	d := &Downloader{
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
	go func() {
		// fetch first task
		task := <-tasks
		for {
			// limit download
			select {
			case <-ctx.Done():
				return
			case <-limiter:
			}
			result := d.process(task)
			results <- result

			// fetch new task or reuse previous (playlist too short)
			select {
			case <-ctx.Done():
				return
			case task = <-tasks:
			default:
			}
		}
	}()
	return d
}

func (d *Downloader) process(task *Task) *Result {
	result := &Result{}
	req, err := http.NewRequest("GET", task.URL, nil)
	if err != nil {
		result.Err = err
		return result
	}
	resp, err := d.client.Do(req)
	if err == nil {
		result.Size = resp.ContentLength
		result.Code = resp.StatusCode
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}
	result.Err = err
	return result
}
