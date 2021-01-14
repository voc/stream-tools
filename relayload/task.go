package main

import (
	"net/url"
	"time"
)

// Task encapsulates a work item that should go in a work
// pool.
type Task struct {
	URL      *url.URL
	Deadline time.Time
}

type Result struct {
	Err  error
	Code int
	Size int64
}
