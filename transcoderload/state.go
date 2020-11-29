package main

import "log"

type State int

const (
	Growing State = iota
	Stalling
	Stable
)

type Estimator struct {
	state   State
	buckets []int
}

func (e *Estimator) inc(slot int) {
	diff := (slot - len(e.buckets))
	for i := 0; i < diff; i++ {
		e.buckets = append(e.buckets, 0)
	}
	e.buckets[slot-1]++
}

func (e *Estimator) Stall(slot int) {
	if e.state == Growing {
		e.inc(slot)
	}
	e.state = Stalling
}

func (e *Estimator) Grow() {
	e.state = Growing
}

func (e *Estimator) PrintStats() {
	log.Println("buckets:", e.buckets)
}
