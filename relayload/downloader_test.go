package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func BenchmarkProcess(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		buf := make([]byte, 4000)
		resp.Write(buf)
	}))
	defer server.Close()

	d := NewDownloader(time.Second)

	client := server.Client()
	task := &Task{}
	for i := 0; i < b.N; i++ {
		d.process(client, task)
	}
}
