export GOEXPERIMENT := jsonv2

BIN := dist/claude-kiro

.PHONY: build install run debug test test-e2e lint vet fmt fix clean

build:
	go build -o $(BIN) ./cmd/claude-kiro

install:
	go install ./cmd/claude-kiro

run:
	go run ./cmd/claude-kiro $(ARGS)

debug:
	go run ./cmd/claude-kiro -debug $(ARGS)

test:
	go test -race ./...

test-e2e:
	go test -tags e2e -race -timeout 120s ./internal/e2e/

lint:
	golangci-lint run

vet:
	go vet ./...

fmt:
	golangci-lint fmt

fix:
	go fix ./...

clean:
	rm -f $(BIN)
