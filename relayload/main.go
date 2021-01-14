package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// ResultQueueLength max number of results to buffer
const ResultQueueLength = 10000

func main() {
	var segmentDuration = flag.Duration("segment-duration", time.Second*3, "segment duration")
	var numWorkers = flag.Uint("workers", 50, "number of workers")
	var limit = flag.Int64("limit", -1, "max requests per second, set to 0 for disable and -1 for auto")
	var sample = flag.Uint("sample", 5, "segments between simulated clients")
	var factor = flag.Uint("factor", 1, "client factor")
	var auth = flag.String("auth", "", "auth type (basic)")
	var user = flag.String("user", "", "auth username")
	var password = flag.String("password", "", "auth password")
	flag.Parse()
	urls := flag.Args()
	log.Printf("Fetching from %d playlist\n", len(urls))

	tasks := make(chan *Task, 50)
	limiter := make(chan struct{}, *numWorkers)
	results := make(chan *Result, ResultQueueLength)
	iteration := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	var lastLimit atomic.Value

	var authFunc SetAuthFunc
	if *auth == "basic" {
		authFunc = func(req *http.Request) {
			req.SetBasicAuth(*user, *password)
		}
	} else {
		authFunc = func(req *http.Request) {}
	}

	// Source routine
	go func() {
		loaderConfig := &LoaderConfig{
			sample:   *sample,
			factor:   *factor,
			taskChan: tasks,
			interval: *segmentDuration,
			authFunc: authFunc,
		}
		pl := NewPlaylistLoader(loaderConfig)
		for {
			for _, URL := range urls {
				err := pl.Load(ctx, URL)
				if err != nil && !strings.HasSuffix(err.Error(), "context canceled") {
					log.Println(err)
				}
			}
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(time.Millisecond * 20)
				iteration <- struct{}{}
			}
		}
	}()

	// Stats routine
	go func() {
		hits, errors, fails := uint64(0), uint64(0), uint64(0)
		last := time.Now()
		bytes := int64(0)
		for {
			select {
			case <-ctx.Done():
				return
			case <-iteration:
				now := time.Now()
				timeFactor := float64(now.Sub(last)) / float64(time.Second)
				last = now
				bits := float64(bytes) / 1048576 * 8 / timeFactor
				ops := float64(hits) / timeFactor
				log.Printf("success: %d, errors: %d, fails: %d, rate: %0.2f Mbit/s, ops: %0.2f Req/s",
					hits, errors, fails, bits, ops)
				lastLimit.Store(uint32(hits))
				hits, errors, fails = 0, 0, 0
				bytes = 0
			case res := <-results:
				bytes += res.Size
				if res.Err != nil {
					fails++
				} else {
					if res.Code == 200 {
						hits++
					} else {
						errors++
					}
				}
			}
		}
	}()

	// Limiter routine
	go func() {
		var auto bool
		if *limit == 0 {
			close(limiter)
			return
		} else if *limit == -1 {
			auto = true
			*limit = int64(len(urls) * 50 / int(*sample))
		}
		lastLimit.Store(uint32(*limit))
		ticker := time.NewTicker(time.Second / time.Duration(*limit))
		tickerReset := time.NewTicker(time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				limiter <- struct{}{}
			case <-tickerReset.C:
				limit := lastLimit.Load().(uint32)
				if limit < 50 {
					limit = 50
				}
				if auto {
					ticker.Stop()
					ticker = time.NewTicker(time.Second / time.Duration(float32(limit)*1.2))
				}
			}
		}
	}()

	// Spawn workers
	d := NewDownloader(*segmentDuration, authFunc)
	d.RunWorkers(ctx, *numWorkers, tasks, limiter, results)

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
			return
		}
	}
}
