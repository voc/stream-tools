package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/zencoder/go-dash/mpd"
)

func segmentName(s *mpd.SegmentTimelineSegment, r *mpd.Representation, offset int64) string {
	st := *r.SegmentTemplate
	id := *r.ID
	number := *st.StartNumber + offset
	name := strings.ReplaceAll(*st.Media, "$RepresentationID$", id)
	return strings.ReplaceAll(name, "$Number$", strconv.FormatInt(number, 10))
}

// parseMpd parses DASH manifest and queues the segment download tasks
func parseMpd(ctx context.Context, reader io.Reader, u *url.URL, sample uint, tasks chan<- *Task) error {
	manifest, err := mpd.Read(reader)
	if err != nil {
		return err
	}

	avStart := *manifest.AvailabilityStartTime
	startTime, err := time.Parse(time.RFC3339, avStart)
	if err != nil {
		return err
	}

	presentationDelay := time.Second * 3
	if manifest.SuggestedPresentationDelay != nil {
		presentationDelay = time.Duration(*manifest.SuggestedPresentationDelay)
	}
	// all segments after that shall not be downloaded in this iteration
	presentationEdge := time.Now().Add(-presentationDelay)

	period := manifest.Periods[0]
	for _, as := range period.AdaptationSets {
		for _, representation := range as.Representations {
			offset := int64(0)
			timestamp := uint64(0)

			// Just support segmenttimeline for now
			template := representation.SegmentTemplate
			timescale := *template.Timescale
			timeline := template.SegmentTimeline

			for _, segment := range timeline.Segments {
				if segment.StartTime != nil {
					timestamp = *segment.StartTime
				}

				repeat := 0
				if segment.RepeatCount != nil {
					repeat = *segment.RepeatCount
				}

				for n := 0; n < repeat+1; n++ {
					name := segmentName(segment, representation, offset)
					segmentURL := fmt.Sprintf("%s://%s%s/%s", u.Scheme, u.Host, path.Dir(u.Path), name)
					ts := startTime.Add(time.Duration(int64(timestamp)/timescale) * time.Second)
					if presentationEdge.After(ts) && offset%int64(sample) == 0 {
						task := &Task{URL: segmentURL, Context: ctx}
						select {
						case <-ctx.Done():
							return errors.New("Rate Limit reached")
						case tasks <- task:
						}
					}
					offset++
					timestamp += segment.Duration
				}
			}
		}
	}
	return nil
}

func parseM3u8(ctx context.Context, reader io.Reader, u *url.URL, sample uint, tasks chan<- *Task) error {
	return nil
}

func get(parent context.Context, urlString string, interval time.Duration, sample uint, tasks chan<- *Task) error {
	u, err := url.Parse(urlString)
	if err != nil {
		return err
	}
	resp, err := http.Get(urlString)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	deadline := time.Now().Add(interval)
	ctx, _ := context.WithDeadline(parent, deadline)
	switch path.Ext(u.Path) {
	case ".mpd":
		return parseMpd(ctx, resp.Body, u, sample, tasks)
	case ".m3u8":
		return parseM3u8(ctx, resp.Body, u, sample, tasks)
	default:
		return fmt.Errorf("Unknown playlist format: '%v' for %v", path.Ext(u.Path), urlString)
	}
}

func main() {
	var segmentDuration = flag.Duration("segment-duration", time.Second*3, "segment duration")
	var numWorkers = flag.Int("workers", runtime.NumCPU(), "number of downloader threads")
	var limit = flag.Int64("limit", -1, "max requests per second, set to 0 for disable and -1 for auto")
	var sample = flag.Uint("sample", 5, "segments between simulated clients")
	flag.Parse()
	urls := flag.Args()

	tasks := make(chan *Task)
	limiter := make(chan struct{}, 1)
	results := make(chan *Result, 1)
	ctx, _ := context.WithCancel(context.Background())
	// Source routine
	go func() {
		ticker := time.NewTicker(*segmentDuration)
		for {
			for _, URL := range urls {
				err := get(ctx, URL, *segmentDuration, *sample, tasks)
				if err != nil {
					log.Println(err)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	// Stats routine
	count := make(chan uint64, 1)
	go func() {
		ticker := time.NewTicker(*segmentDuration)
		hits, errors, bytes := uint64(0), uint64(0), int64(0)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				log.Printf("success: %d, error: %d, rate: %0.2f Mbit/s", hits, errors, float64(bytes)/1048576*8)
				count <- uint64(hits + errors)
				hits, errors, bytes = 0, 0, 0
			case res := <-results:
				bytes += res.Loaded
				if res.Err != nil {
					if res.Err != context.DeadlineExceeded {
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
			*limit = 50
		}
		ticker := time.NewTicker(time.Second / time.Duration(*limit))
		for {
			select {
			case <-ticker.C:
				limiter <- struct{}{}
			case total := <-count:
				if auto {
					ticker.Stop()
					ticker = time.NewTicker(time.Second / time.Duration(float32(total)*1.2))
				}
			}
		}
	}()

	// Spawn workers
	for i := 0; i < *numWorkers; i++ {
		go NewDownloader(*segmentDuration, tasks, limiter, results)
	}
	<-ctx.Done()
}
