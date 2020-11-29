package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

type Job struct {
	Name       string
	cancel     func()
	stopped    bool
	stallCount int
	lastSpeed  float64
}

func (j *Job) Stop() {
	j.stopped = true
	j.cancel()
}

func (j *Job) Running() bool {
	return !j.stopped
}

type Overload struct{}

func (j *Job) parseSpeed(status map[string]string) {
	speed, ok := status["speed"]
	if !ok {
		return
	}
	if speed == "N/A" {
		// log.Println("skip N/A")
		j.stallCount++
		return
	}
	speed = strings.TrimSpace(speed)
	fspeed, err := strconv.ParseFloat(speed[:len(speed)-1], 64)
	if err != nil {
		log.Println(err)
		j.stallCount++
		return
	}
	if fspeed < 1 && j.lastSpeed > fspeed {
		log.Println("speed", fspeed)
		j.stallCount++
	} else {
		j.stallCount = 0
	}
	j.lastSpeed = fspeed
}

func (j *Job) parseStatus(content string, status map[string]string) error {
	res := strings.Split(content, "\n")
	for _, line := range res {
		if line == "" {
			continue
		}
		split := strings.Split(line, "=")
		if len(split) != 2 {
			return fmt.Errorf("Invalid progress '%v'", line)
		}
		status[split[0]] = split[1]
	}
	return nil
}

func handleConnection(conn net.Conn, job *Job, overload <-chan Overload) {
	buf := make([]byte, 2048)
	status := make(map[string]string)
	defer job.Stop()
	for {
		length, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Println("read failed:", err)
			}
			break
		}
		str := string(buf[:length])
		if err := job.parseStatus(str, status); err != nil {
			log.Println("status parse failed", err)
			return
		}
		job.parseSpeed(status)
		if job.stallCount > 5 {
			log.Println("stall")
			job.Stop()
		}
	}
}

func launch(parentCtx context.Context, name string, dirname string, overload <-chan Overload) (*Job, error) {
	filename := path.Join(dirname, name+".sock")
	ctx, cancel := context.WithCancel(parentCtx)
	job := Job{
		cancel: cancel,
		Name:   name,
	}

	ln, err := net.Listen("unix", filename)
	if err != nil {
		return nil, err
	}

	// accept single connection
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("accept failed:", err)
		}
		handleConnection(conn, &job, overload)
	}()

	go func() {
		defer os.Remove(filename)
		defer ln.Close()

		args := append([]string{"-y", "-hide_banner", "-v", "warning", "-progress", "unix://" + filename}, os.Args[1:]...)

		cmd := exec.Command("ffmpeg", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err = cmd.Start()
		if err != nil {
			log.Println(err)
		}
		defer cmd.Process.Kill()

		// stop job if command ends
		go func() {
			cmd.Wait()
			job.Stop()
		}()

		// wait for exit
		<-ctx.Done()
	}()
	return &job, nil
}
