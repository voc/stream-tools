package main

import (
	"net/url"
)

// Task encapsulates a work item that should go in a work pool.
type Task struct {
	URL *url.URL
}

// Result contains info communicated back to the statistics collector
type Result struct {
	Err  error
	Code int
	Size int64
}
