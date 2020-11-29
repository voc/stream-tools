package main

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func run(ctx context.Context, dirname string) {
	estimator := Estimator{}
	defer estimator.PrintStats()

	n := 1
	count := 0
	ticker := time.NewTicker(20 * time.Second)
	overload := make(chan Overload)
	var jobs []*Job
	for {
		if count == 0 {
			count++
			estimator.Grow()
		}
		diff := count - len(jobs)
		log.Println("count", count, "diff", diff)

		if diff > 0 {
			for i := 0; i < diff; i++ {
				// increase jobs
				name := "ffmpeg" + strconv.Itoa(n)
				n++
				job, err := launch(ctx, name, dirname, overload)
				if err != nil {
					log.Fatal(err)
				}
				jobs = append(jobs, job)
				log.Println("launched")
			}

		} else {
			// decrease jobs
			for i := 0; i < -diff; i++ {
				jobs[len(jobs)-1-i].Stop()
			}
		}

		// wait or exit
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		var remain []*Job
		for _, job := range jobs {
			if job.Running() {
				remain = append(remain, job)
			}
		}
		if len(remain) == count {
			count++
			estimator.Grow()
		} else {
			estimator.Stall(count)
			count = len(remain)
		}
		jobs = remain
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	dirname, err := ioutil.TempDir(os.TempDir(), "*")
	if err != nil {
		log.Fatal(err)
	}

	// run ffmpegs
	go run(ctx, dirname)

	// signal handling
	c := make(chan os.Signal, 1)
	signal.Notify(c,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM)

	for {
		sig := <-c
		log.Println("Caught signal", sig)
		if sig != syscall.SIGHUP {
			// Wait for shutdown
			cancel()
			time.Sleep(200 * time.Millisecond)
			os.RemoveAll(dirname)
			return
		}
	}
}
