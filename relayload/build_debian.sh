#!/bin/sh
podman run --rm -v $(pwd):/root -v $HOME/.cache/go-build/:/mnt/gocache -e GOCACHE=/mnt/gocache --workdir /root golang:1.14-buster go build
