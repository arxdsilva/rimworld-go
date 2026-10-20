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

# ---- README media (docs/screenshots, docs/clips) ----------------------------
SHOT := $(BIN_DIR)/game -capture

.PHONY: screenshots
screenshots:  ## README stills in docs/screenshots (real renderer)
	go build -o $(BIN_DIR)/game ./cmd/game
	$(SHOT) docs/screenshots/colony.png -capture-ticks 12000 -capture-zoom 0.8
	$(SHOT) docs/screenshots/architect.png -capture-ui architect
	$(SHOT) docs/screenshots/farm.png -capture-ui grow
	$(SHOT) docs/screenshots/bills.png -capture-ui bills
	$(SHOT) docs/screenshots/siege.png -scenario siege -capture-ui raid
	$(SHOT) docs/screenshots/defense.png -scenario defense -capture-ui defense -capture-ticks 60 -capture-zoom 0.8
	$(SHOT) docs/screenshots/combat.png -capture-ui combat
	$(SHOT) docs/screenshots/animals.png -scenario animals -capture-ui pen
	$(SHOT) docs/screenshots/research.png -scenario research -capture-ui research
	$(SHOT) docs/screenshots/medic.png -scenario medic -capture-ui medic
	$(SHOT) docs/screenshots/winter.png -scenario winter -capture-ui winter

# $(call clip,NAME,FLAGS): record frames to bin/clips/NAME, encode docs/clips/NAME.webp
define clip
	rm -rf $(BIN_DIR)/clips/$(1)
	$(SHOT) $(BIN_DIR)/clips/$(1)/f.png $(2)
	python3 scripts/frames_to_webp.py --frames '$(BIN_DIR)/clips/$(1)/f_*.png' --out docs/clips/$(1).webp
endef

.PHONY: clips
clips:  ## README animated WebP clips in docs/clips (needs python3 + Pillow)
	go build -o $(BIN_DIR)/game ./cmd/game
	$(call clip,colony_life,-capture-ui follow -capture-frames 240)
	$(call clip,building,-capture-ticks 600 -capture-zoom 1.5 -capture-frames 240)
	$(call clip,combat,-capture-ui combat -capture-frames 180)
	$(call clip,raid,-scenario raid -capture-ui raid -capture-ticks 500 -capture-frames 300)
	$(call clip,animals,-scenario animals -capture-ui pen -capture-frames 240)
	$(call clip,melee,-scenario armory -capture-ui raid -capture-zoom 1.6 -capture-ticks 560 -capture-frames 240)
	$(call clip,chores,-scenario chores -capture-ui follow -capture-zoom 1.8 -capture-ticks 4000 -capture-frames 240)

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)
