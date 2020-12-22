package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

type Job struct {
	Name       string
	ctx        context.Context
	cancel     func()
	stopped    bool
	stallCount int
	lastSpeed  float64
	cmd        *exec.Cmd
	wg         sync.WaitGroup
}

func (j *Job) Stop() {
	j.cancel()
	j.wg.Wait()
}

func (j *Job) Running() bool {
	return !j.stopped
}

func (j *Job) parseSpeed(status map[string]string) {
	speed, ok := status["speed"]
	if !ok {
		return
	}
	if speed == "N/A" {
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

func (j *Job) handleConnection(conn net.Conn) {
	buf := make([]byte, 2048)
	status := make(map[string]string)
	defer j.cancel()
	for {
		length, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Println("read failed:", err)
			}
			return
		}
		str := string(buf[:length])
		if err := j.parseStatus(str, status); err != nil {
			log.Println("status parse failed", err)
			return
		}
		j.parseSpeed(status)
		if j.stallCount > 5 {
			log.Println("stall")
			return
		}
	}
}

func (j *Job) start(exitNotify chan<- struct{}) {
	j.wg.Add(1)
	j.cmd.Stdout = os.Stdout
	j.cmd.Stderr = os.Stderr

	err := j.cmd.Start()
	if err != nil {
		log.Println(err)
	}
	defer j.cmd.Process.Signal(syscall.SIGTERM)

	// stop job if command ends
	go func() {
		j.cmd.Wait()
		j.stopped = true
		j.cancel()
		j.wg.Done()
		select {
		case exitNotify <- struct{}{}:
		default:
		}
	}()

	// wait for exit
	<-j.ctx.Done()
}

func launch(parentCtx context.Context, name string, dirname string, cmd string, exitNotify chan<- struct{}) (*Job, error) {
	ctx, cancel := context.WithCancel(parentCtx)
	filename := path.Join(dirname, name+".sock")
	args := append([]string{"-v", "warning", "-progress", "unix://" + filename}, flag.Args()...)
	job := Job{
		ctx:    ctx,
		cancel: cancel,
		Name:   name,
		cmd:    exec.Command(cmd, args...),
	}

	ln, err := net.Listen("unix", filename)
	if err != nil {
		return nil, err
	}

	// accept single connection
	go func() {
		defer os.Remove(filename)
		defer ln.Close()
		conn, err := ln.Accept()
		job.wg.Add(1)
		defer job.wg.Done()
		if err != nil {
			log.Println("accept failed:", err)
		}
		job.handleConnection(conn)
	}()
	go job.start(exitNotify)

	return &job, nil
}
