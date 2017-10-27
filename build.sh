#!/bin/bash

echo "Building..."

go install ./...

cd cmd/ca

GOARCH=386 go build

cd ../firechainClient

GOARCH=386 go build
