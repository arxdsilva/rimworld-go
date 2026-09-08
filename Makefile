MODULE   := rimworld-go
BIN_DIR  := bin
CMDS     := game simulate benchmark replay contentlint

.PHONY: all
all: fmt vet lint test build

.PHONY: build
build:
	@mkdir -p $(BIN_DIR)
	@for cmd in $(CMDS); do \
		echo "building $$cmd"; \
		go build -o $(BIN_DIR)/$$cmd ./cmd/$$cmd || exit 1; \
	done

.PHONY: fmt
fmt:
	gofmt -l .

.PHONY: fmt-fix
fmt-fix:
	gofmt -w .

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint:
	staticcheck ./...

.PHONY: test
test: test-unit

.PHONY: test-unit
test-unit:
	go test -timeout 30s -race -cover ./...

.PHONY: simulate
simulate:
	go run ./cmd/simulate --seed 1 --ticks 100 --pawns 3 --width 20 --height 20

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)
