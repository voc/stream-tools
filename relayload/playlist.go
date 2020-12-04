package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/quangngotan95/go-m3u8/m3u8"
	"github.com/zencoder/go-dash/mpd"
)

type PlaylistLoader struct {
	sample   uint
	taskChan chan<- *Task
	interval time.Duration
	client   *http.Client
}

func NewPlaylistLoader(sample uint, taskChan chan<- *Task, interval time.Duration) *PlaylistLoader {
	return &PlaylistLoader{
		sample:   sample,
		taskChan: taskChan,
		interval: interval,
		client: &http.Client{
			Timeout:   interval,
			Transport: transport,
		},
	}
}

func (pl *PlaylistLoader) Load(parent context.Context, urlString string) error {
	deadline := time.Now().Add(pl.interval)
	ctx, _ := context.WithDeadline(parent, deadline)
	return pl.get(ctx, urlString)
}

func (pl *PlaylistLoader) get(ctx context.Context, urlString string) error {

	playlistURL, err := url.Parse(urlString)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlString, nil)
	if err != nil {
		return err
	}
	resp, err := pl.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("Playlist %s: got %s\n", urlString, resp.Status)
		return nil
	}
	defer resp.Body.Close()

	switch path.Ext(playlistURL.Path) {
	case ".mpd":
		return pl.parseMpd(ctx, resp.Body, playlistURL)
	case ".m3u8":
		return pl.parseM3u8(ctx, resp.Body, playlistURL)
	default:
		return fmt.Errorf("Unknown playlist format: '%v' for %v", path.Ext(playlistURL.Path), urlString)
	}
}

var (
	errAvailabilityStartMissing = errors.New("Dash Manifest missing AvailabilityStartTime")
	errNoSegmentTemplate        = errors.New("Dash Manifest loader requires Representations with SegmentTemplate for now")
	errTimescaleMissing         = errors.New("Dash Manifest SegmentTemplate is missing timescale")
)

// parseMpd parses DASH manifest and queues the segment download tasks
func (pl *PlaylistLoader) parseMpd(ctx context.Context, reader io.Reader, playlistURL *url.URL) error {
	manifest, err := mpd.Read(reader)
	if err != nil {
		return err
	}

	if manifest.AvailabilityStartTime == nil {
		log.Println("url", playlistURL)
		return errAvailabilityStartMissing
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
			if representation.SegmentTemplate == nil {
				return errNoSegmentTemplate
			}
			template := representation.SegmentTemplate
			if template.Timescale == nil {
				return errTimescaleMissing
			}
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
					ts := startTime.Add(time.Duration(int64(timestamp)/timescale) * time.Second)
					// Only fetch segments before the recommended presentation edge
					if presentationEdge.After(ts) && offset%int64(pl.sample) == 0 {
						name := dashSegmentName(segment, representation, offset)
						segmentURL := fmt.Sprintf("%s://%s%s/%s", playlistURL.Scheme, playlistURL.Host, path.Dir(playlistURL.Path), name)
						task := &Task{URL: segmentURL, Context: ctx}
						select {
						case <-ctx.Done():
							return errors.New("Rate Limit reached")
						case pl.taskChan <- task:
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

// dashSegmentName templates a MPEG-DASH SegmentTemplate name
func dashSegmentName(s *mpd.SegmentTimelineSegment, r *mpd.Representation, offset int64) string {
	st := r.SegmentTemplate
	id := *r.ID
	number := *st.StartNumber + offset
	name := strings.ReplaceAll(*st.Media, "$RepresentationID$", id)
	return strings.ReplaceAll(name, "$Number$", strconv.FormatInt(number, 10))
}

// parseMpd parses m3u8 playlists and creates download tasks for all segments.
// Can work with multi-quality master-playlists.
func (pl *PlaylistLoader) parseM3u8(ctx context.Context, reader io.Reader, playlistURL *url.URL) error {
	playlist, err := m3u8.Read(reader)
	if err != nil {
		return fmt.Errorf("playlist %v error: %v", playlistURL, err)
	}

	if playlist.IsMaster() {
		// Recursively fetch sub-playlists
		for _, item := range playlist.Items {
			if subPlaylist, ok := item.(*m3u8.PlaylistItem); ok {
				subURL := fmt.Sprintf("%s://%s%s/%s", playlistURL.Scheme, playlistURL.Host, path.Dir(playlistURL.Path), subPlaylist.URI)
				err := pl.get(ctx, subURL)
				if err != nil {
					return err
				}
			}
		}
	} else {
		// Create tasks for segments in each playlist
		segmentCount := playlist.SegmentSize()
		offset := 0
		for _, item := range playlist.Items {
			if segment, ok := (item).(*m3u8.SegmentItem); ok {
				// Don't rqeuest last 2 segments of a HLS playlist as per RFC
				if offset < segmentCount-2 && offset%int(pl.sample) == 0 {
					segmentURL := fmt.Sprintf("%s://%s%s/%s", playlistURL.Scheme, playlistURL.Host, path.Dir(playlistURL.Path), segment.Segment)
					task := &Task{URL: segmentURL, Context: ctx}
					select {
					case <-ctx.Done():
						return errors.New("Rate Limit reached")
					case pl.taskChan <- task:
					}
				}
				offset++
			}
		}
	}

	return nil
}
