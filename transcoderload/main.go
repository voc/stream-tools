package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func run(ctx context.Context, dirname string, cmd string) {
	estimator := NewEstimator()
	defer estimator.PrintStats()

	n := 1
	timer := time.NewTimer(time.Second)
	timer.Stop()
	notify := make(chan struct{}, 1)
	var jobs []*Job
	for {
		count, holdTime := estimator.Cycle()
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
			log.Println("launched")
			time.Sleep(time.Second)
		}

		timer.Reset(holdTime)

		select {
		case <-ctx.Done():
			// exit
			return
		case <-notify:
			// job did stall
			for i := 0; i < len(jobs); i++ {
				jobs[i].Stop()
			}
			jobs = []*Job{}
			// drain notify
			select {
			case <-notify:
			default:
			}
			estimator.Stall()
			estimator.PrintStats()
			continue
		case <-timer.C:
			// cycle ended without stall
		}
		estimator.Grow()
		estimator.PrintStats()
	}
}

func main() {
	var cmd = flag.String("cmd", "ffmpeg", "command")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	dirname, err := ioutil.TempDir(os.TempDir(), "*")
	if err != nil {
		log.Fatal(err)
	}

	// run ffmpegs
	go run(ctx, dirname, *cmd)

	// signal handling
	c := make(chan os.Signal, 1)
	signal.Notify(c,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM)

	for {
		select {
		case sig := <-c:
			log.Println("Caught signal", sig)
			if sig == syscall.SIGHUP {
				continue
			}
		case <-ctx.Done():
		}
		// Cleanup
		cancel()
		os.RemoveAll(dirname)
		return
	}
}
