BIN_DIR := ./bin
LDFLAGS := -s -w
GOFLAGS := -trimpath
CGO_ENABLED := 0

.PHONY: all build clean ginprov lint test fmt refresh tidy

all: build

build: ginprov

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

ginprov: | $(BIN_DIR)
	go build $(GOFLAGS) \
	         -ldflags "$(LDFLAGS)" \
	         -o $(BIN_DIR)/ginprov \
	         ./cmd/ginprov

lint:
	golangci-lint run

test:
	go test -race ./...

bench:
	go test -run=^$$ -bench=. -benchmem ./...

fmt:
	go fmt ./... && gofumpt -w .

clean:
	rm -rf $(BIN_DIR)

refresh: ginprov
	./testdata/refresh.sh

tidy:
	go mod tidy
