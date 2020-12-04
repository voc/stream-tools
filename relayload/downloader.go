package main

import (
	"context"
	"fmt"
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
		for task := range tasks {
			// Skip old tasks
			select {
			case <-ctx.Done():
				return
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
	result := &Result{}
	req, err := http.NewRequestWithContext(task.Context, "GET", task.URL, nil)
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
	if resp.StatusCode != 200 {
		result.Err = fmt.Errorf("Got status %v", resp.Status)
	}
	bytes, err := io.Copy(ioutil.Discard, resp.Body)
	result.Loaded = bytes
	result.Err = err
	return result
}
