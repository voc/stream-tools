package main

import (
	"context"
	"time"
)

// Task encapsulates a work item that should go in a work
// pool.
type Task struct {
	URL      string
	Deadline time.Time
	Context  context.Context
}

type Result struct {
	Err    error
	Loaded int64
}
