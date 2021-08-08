package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var cmd = flag.String("cmd", "ffmpeg", "command")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	dirname, err := ioutil.TempDir(os.TempDir(), "*")
	if err != nil {
		log.Fatal(err)
	}

	// start running jobs
	r := NewRunner(ctx, dirname, *cmd)

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
		r.Wait()
		os.RemoveAll(dirname)
		return
	}
}
