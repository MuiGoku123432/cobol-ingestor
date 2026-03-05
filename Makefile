.PHONY: build run-ingest run-api test test-integration swagger docker docker-build lint

build:
	go build -o bin/cobol-graph ./cmd/ingest
	go build -o bin/cobol-graph-api ./cmd/server

run-ingest:
	./bin/cobol-graph ingest --dir=$(DIR)

run-api:
	./bin/cobol-graph-api

test:
	go test ./... -v -race

test-integration:
	go test ./... -v -race -tags=integration

swagger:
	swag init -g cmd/server/main.go -o api/docs

docker:
	docker compose up -d

docker-build:
	docker build -t cobol-graph-api .

lint:
	golangci-lint run ./...
