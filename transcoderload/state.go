package main

import (
	"log"
	"time"
)

type State int

const (
	Growing State = iota
	Stalling
	Stable
)

type Estimator struct {
	state   State
	buckets []float64
	index   int
	weight  float64
}

func NewEstimator() *Estimator {
	return &Estimator{
		weight:  1,
		buckets: []float64{0},
	}
}

func (e *Estimator) Stall() {
	e.buckets[e.index] = e.buckets[e.index] - e.weight
	if e.index < 1 {
		e.index = 0
	} else {
		e.index = e.index - 1
	}
	e.weight = e.weight * 2
}

func (e *Estimator) Grow() {
	e.buckets[e.index] = e.buckets[e.index] + e.weight
	e.state = Growing
	e.index++
	if e.index == len(e.buckets) {
		e.buckets = append(e.buckets, 0)
	}
}

func (e *Estimator) PrintStats() {
	log.Println("Confidence per job count:", e.buckets)
}

func (e *Estimator) State() State {
	return e.state
}

func (e *Estimator) Cycle() (count int, holdTime time.Duration) {
	count = e.index + 1
	holdTime = time.Duration(e.weight*20) * time.Second
	return
}
