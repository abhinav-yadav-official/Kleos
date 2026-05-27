SHELL := /usr/bin/env bash

.PHONY: lint test build up down migrate run-api

lint:
	go vet ./...

test:
	go test ./... -count=1

build:
	mkdir -p bin
	go build -o bin/api ./cmd/api
	go build -o bin/migrate ./cmd/migrate
	go build -o bin/worker-jobscraper ./cmd/worker-jobscraper
	go build -o bin/worker-campaigntick ./cmd/worker-campaigntick
	go build -o bin/worker-emailfinder ./cmd/worker-emailfinder
	go build -o bin/worker-contentgen ./cmd/worker-contentgen
	go build -o bin/worker-sender ./cmd/worker-sender
	go build -o bin/worker-warmuprollover ./cmd/worker-warmuprollover
	go build -o bin/worker-prefetchpool ./cmd/worker-prefetchpool

up:
	docker compose -f deploy/docker-compose.dev.yml up -d

down:
	docker compose -f deploy/docker-compose.dev.yml down

migrate:
	go run ./cmd/migrate up

run-api:
	go run ./cmd/api
