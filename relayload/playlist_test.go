package main

import (
	"context"
	"io"
	"net/url"
	"testing"
)

func TestPlaylistLoader_getSubURL(t *testing.T) {
	pl := &PlaylistLoader{}
	type args struct {
		ctx    context.Context
		reader io.Reader
	}
	tests := []struct {
		name        string
		playlistURL string
		subURI      string
		expected    string
	}{
		{"relativeURI", "https://cdn.c3voc.de/dash/s1/manifest.mpd", "seg1.webm", "https://cdn.c3voc.de/dash/s1/seg1.webm"},
		{"absoluteURI", "https://cdn.c3voc.de/hls/s1/master.m3u8", "/hls/s2/subplaylist.m3u8", "https://cdn.c3voc.de/hls/s2/subplaylist.m3u8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseURL, err := url.Parse(tt.playlistURL)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := pl.getSubURL(baseURL, tt.subURI); err != nil || got.String() != tt.expected {
				t.Errorf("PlaylistLoader.getSubURL() got = %v, expected %v", got, tt.expected)
			}
		})
	}
}
