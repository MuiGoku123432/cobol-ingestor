.PHONY: build run-ingest run-api test swagger docker lint

build:
	go build -o bin/cobol-graph ./cmd/ingest
	go build -o bin/cobol-graph-api ./cmd/server

run-ingest:
	./bin/cobol-graph ingest --dir=$(DIR)

run-api:
	./bin/cobol-graph-api

test:
	go test ./... -v -race

swagger:
	swag init -g cmd/server/main.go -o api/docs

docker:
	docker compose up -d

lint:
	golangci-lint run ./...
