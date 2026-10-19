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

# ---- art (ART spec §25) -----------------------------------------------------
BLENDER ?= $(shell command -v blender 2>/dev/null || echo /Applications/Blender.app/Contents/MacOS/Blender)

.PHONY: art
art:  ## re-render every world sprite from assets/source/blender
	"$(BLENDER)" -b --factory-startup -P assets/source/blender/build.py -- $(FAMILIES)
	python3 assets/source/tools/validate_assets.py

.PHONY: art-check
art-check:  ## validate generated sprites against metadata
	python3 assets/source/tools/validate_assets.py

.PHONY: art-capture
art-capture:  ## in-game screenshot of a scripted colony (real renderer)
	go build -o $(BIN_DIR)/game ./cmd/game
	$(BIN_DIR)/game -capture assets/generated/ingame_capture.png
	$(BIN_DIR)/game -capture assets/generated/ingame_architect.png -capture-ui architect
	$(BIN_DIR)/game -capture assets/generated/ingame_orders.png -capture-ui orders

.PHONY: art-preview
art-preview:  ## contact sheet + ART §34 prototype scene
	python3 assets/source/tools/contact_sheet.py
	python3 assets/source/tools/compose_scene.py

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)
