package main

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"
)

type Runner struct {
	jobs      []*Job
	estimator *Estimator
	done      sync.WaitGroup
}

func NewRunner(ctx context.Context, dirname string, cmd string) *Runner {
	r := &Runner{
		estimator: NewEstimator(),
	}
	r.done.Add(1)
	go r.run(ctx, dirname, cmd)
	return r
}

func (r *Runner) Wait() {
	r.done.Wait()
}

func (r *Runner) stop() {
	// exit
	for i := 0; i < len(r.jobs); i++ {
		r.jobs[i].Stop()
	}
}

func (r *Runner) run(ctx context.Context, dirname string, cmd string) {
	defer r.done.Done()
	defer r.estimator.PrintStats()

	n := 1
	timer := time.NewTimer(time.Second)
	timer.Stop()
	defer timer.Stop()
	notify := make(chan struct{}, 1)
	var jobs []*Job
	for {
		count, holdTime := r.estimator.Cycle()
		diff := count - len(jobs)
		log.Printf("count: %d, diff: %d, hold: %v", count, diff, holdTime)

		for i := 0; i < diff; i++ {
			// increase jobs
			name := "ffmpeg" + strconv.Itoa(n)
			n++
			job, err := launch(ctx, name, dirname, cmd, notify)
			if err != nil {
				log.Fatal(err)
			}
			jobs = append(jobs, job)
			log.Println("job launched")
			time.Sleep(time.Second)
		}

		timer.Reset(holdTime)

		// Make sure we stop immediately when requested
		select {
		case <-ctx.Done():
			r.stop()
			return
		default:
		}

		select {
		case <-ctx.Done():
			r.stop()
			return
		case <-notify:
			// job did stall
			if !timer.Stop() {
				<-timer.C
			}
			// stop all running jobs
			for i := 0; i < len(jobs); i++ {
				jobs[i].Stop()
			}
			jobs = []*Job{}
			// drain notify
			select {
			case <-notify:
			default:
			}
			r.estimator.Stall()
			r.estimator.PrintStats()
			continue
		case <-timer.C:
			// cycle ended without stall
		}
		r.estimator.Grow()
		r.estimator.PrintStats()
	}
}
