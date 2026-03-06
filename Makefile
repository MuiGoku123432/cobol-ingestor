BINARY_DIR := bin
INSTALL_DIR ?= /usr/local/bin

BINARIES := \
	$(BINARY_DIR)/cobol-graph \
	$(BINARY_DIR)/cobol-graph-api \
	$(BINARY_DIR)/cobol-graph-mcp \
	$(BINARY_DIR)/cobol-graph-modernize

.PHONY: build clean install uninstall run-ingest run-api run-mcp run-modernize test test-integration vet lint swagger docker docker-build

build: $(BINARIES)

$(BINARY_DIR)/cobol-graph: $(shell find cmd/ingest internal prompts -type f)
	go build -o $@ ./cmd/ingest

$(BINARY_DIR)/cobol-graph-api: $(shell find cmd/server internal api -type f)
	go build -o $@ ./cmd/server

$(BINARY_DIR)/cobol-graph-mcp: $(shell find cmd/mcp internal -type f)
	go build -o $@ ./cmd/mcp

$(BINARY_DIR)/cobol-graph-modernize: $(shell find cmd/modernize internal web -type f)
	go build -o $@ ./cmd/modernize

clean:
	rm -rf $(BINARY_DIR)

install: build
	install -d $(INSTALL_DIR)
	install -m 755 $(BINARIES) $(INSTALL_DIR)/

uninstall:
	rm -f $(addprefix $(INSTALL_DIR)/,$(notdir $(BINARIES)))

run-ingest: $(BINARY_DIR)/cobol-graph
	./bin/cobol-graph ingest --dir=$(DIR)

run-api: $(BINARY_DIR)/cobol-graph-api
	./bin/cobol-graph-api

run-mcp: $(BINARY_DIR)/cobol-graph-mcp
	./bin/cobol-graph-mcp

run-modernize: $(BINARY_DIR)/cobol-graph-modernize
	./bin/cobol-graph-modernize

test:
	go test ./... -v -race

test-integration:
	go test ./... -v -race -tags=integration

vet:
	go vet ./...

lint:
	golangci-lint run ./...

swagger:
	swag init -g cmd/server/main.go -o api/docs

docker:
	docker compose up -d

docker-build:
	docker build -t cobol-graph-api .
