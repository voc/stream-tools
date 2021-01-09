package main

import (
	"time"
)

// Task encapsulates a work item that should go in a work
// pool.
type Task struct {
	URL      string
	Deadline time.Time
}

type Result struct {
	Err  error
	Code int
	Size int64
}
