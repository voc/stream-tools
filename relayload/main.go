package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

func main() {
	var segmentDuration = flag.Duration("segment-duration", time.Second*3, "segment duration")
	var numWorkers = flag.Int("workers", runtime.NumCPU(), "number of downloader threads")
	var limit = flag.Int64("limit", -1, "max requests per second, set to 0 for disable and -1 for auto")
	var sample = flag.Uint("sample", 5, "segments between simulated clients")
	flag.Parse()
	urls := flag.Args()
	log.Printf("Fetching from %d playlist\n", len(urls))

	tasks := make(chan *Task, 1)
	limiter := make(chan struct{}, *numWorkers)
	results := make(chan *Result, 1)
	iteration := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())

	// Source routine
	go func() {
		ticker := time.NewTicker(*segmentDuration)
		pl := NewPlaylistLoader(*sample, tasks, *segmentDuration)
		for {
			for _, URL := range urls {
				err := pl.Load(ctx, URL)
				if err != nil {
					log.Println(err)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				iteration <- struct{}{}
			}
		}
	}()

	// Stats routine
	count := make(chan uint64, 1)
	go func() {
		hits, timeouts, errors := uint64(0), uint64(0), uint64(0)
		last := time.Now()
		bytes := int64(0)
		for {
			select {
			case <-ctx.Done():
				return
			case <-iteration:
				now := time.Now()
				timeFactor := float64(now.Sub(last) / time.Second)
				last = now
				bits := float64(bytes) / 1048576 * 8 / timeFactor
				log.Printf("success: %d, timeouts: %d, error: %d, rate: %0.2f Mbit/s", hits, timeouts, errors, bits)
				count <- uint64(hits + errors)
				hits, timeouts, errors = 0, 0, 0
				bytes = 0
			case res := <-results:
				bytes += res.Loaded
				if res.Err != nil {
					if res.Err == context.DeadlineExceeded {
						timeouts++
					} else {
						log.Println("Request error:", res.Err)
						errors++
					}
				} else {
					hits++
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
		ticker := time.NewTicker(time.Second / time.Duration(*limit))
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				limiter <- struct{}{}
			case total := <-count:
				if total < 50 {
					total = 50
				}
				if auto {
					ticker.Stop()
					ticker = time.NewTicker(time.Second / time.Duration(float32(total)*1.2))
				}
			}
		}
	}()

	// Spawn workers
	for i := 0; i < *numWorkers; i++ {
		go NewDownloader(ctx, *segmentDuration, tasks, limiter, results)
	}

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
