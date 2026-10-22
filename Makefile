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
art:  ## re-render every world sprite from assets/source/blender, then their normal maps
	"$(BLENDER)" -b --factory-startup -P assets/source/blender/build.py -- $(FAMILIES)
	$(MAKE) normals
	python3 assets/source/tools/validate_assets.py

.PHONY: art-menu
art-menu:  ## render the main-menu colony diorama (assets/source/blender/artlib/scenes/menu.py)
	"$(BLENDER)" -b --factory-startup -P assets/source/blender/build.py -- menu
	python3 assets/source/tools/validate_assets.py

.PHONY: audio
audio:  ## render the procedural sound effects (cmd/audiogen) and update the sound list
	go run ./cmd/audiogen

.PHONY: music
music:  ## compose the music (cmd/audiogen/music.go) to Ogg Vorbis; needs ffmpeg with libvorbis
	go run ./cmd/audiogen -music

.PHONY: audio-check
audio-check:  ## validate the sound list against the files and the size budget
	go run ./cmd/audiogen -check

.PHONY: normals
normals:  ## write <sprite>_n.png normal maps (pkg/spritenormals, a port of spritefusion-sprite-normals)
	go run ./cmd/normalgen -dirs assets/generated/world $(NORMALS_ARGS)

.PHONY: art-check
art-check:  ## validate generated sprites against metadata
	python3 assets/source/tools/validate_assets.py

.PHONY: art-capture
art-capture:  ## in-game screenshot of a scripted colony (real renderer)
	go build -o $(BIN_DIR)/game ./cmd/game
	$(BIN_DIR)/game -capture assets/generated/ingame_capture.png
	$(BIN_DIR)/game -capture assets/generated/ingame_architect.png -capture-ui architect
	$(BIN_DIR)/game -capture assets/generated/ingame_orders.png -capture-ui orders
	$(BIN_DIR)/game -capture assets/generated/ingame_menu.png -capture-ui menu

.PHONY: art-preview
art-preview:  ## contact sheet + ART §34 prototype scene
	python3 assets/source/tools/contact_sheet.py
	python3 assets/source/tools/compose_scene.py

# ---- README media (docs/media, docs/clips): animated WebP only -------------
SHOT := $(BIN_DIR)/game -capture

# $(call webp,DIR,NAME,FLAGS[,ENCODER ARGS]): record frames to bin/clips/NAME,
# encode DIR/NAME.webp (default quality 60, budget 1.4 MB; ENCODER ARGS may override)
define webp
	rm -rf $(BIN_DIR)/clips/$(2)
	$(SHOT) $(BIN_DIR)/clips/$(2)/f.png $(3)
	python3 scripts/frames_to_webp.py --frames '$(BIN_DIR)/clips/$(2)/f_*.png' --out $(1)/$(2).webp --quality 60 --max-bytes 1400000 $(4)
endef
clip = $(call webp,docs/clips,$(1),$(2),$(3))
still = $(call webp,docs/media,$(1),$(2))

.PHONY: media
media:  ## README gallery as short animated WebPs in docs/media (needs python3 + Pillow)
	go build -o $(BIN_DIR)/game ./cmd/game
	$(call still,colony,-capture-ticks 12000 -capture-zoom 0.8 -capture-frames 180)
	$(call still,architect,-capture-ui architect -capture-frames 150)
	$(call still,farm,-capture-ui grow -capture-frames 150)
	$(call still,bills,-capture-ui bills -capture-frames 150)
	$(call still,siege,-scenario siege -capture-ui raid -capture-frames 180)
	$(call still,defense,-scenario defense -capture-ui defense -capture-ticks 60 -capture-zoom 0.8 -capture-frames 180)
	$(call still,combat,-scenario armory -capture-ui raid -capture-zoom 1.6 -capture-ticks 560 -capture-frames 180)
	$(call still,animals,-scenario animals -capture-ui pen -capture-frames 180)
	$(call still,research,-scenario research -capture-ui research -capture-frames 150)
	$(call still,medic,-scenario medic -capture-ui medic -capture-frames 150)
	$(call still,winter,-scenario winter -capture-ui winter -capture-frames 180)

.PHONY: clips
clips:  ## README animated WebP clips in docs/clips (needs python3 + Pillow)
	go build -o $(BIN_DIR)/game ./cmd/game
	$(call clip,lategame,-scenario lategame -capture-ui lategame -capture-ticks 1200 -capture-frames 360,--step 3 --fps 20 --quality 45 --max-bytes 2600000)
	$(call clip,colony_life,-capture-ui follow -capture-frames 240,--quality 45 --max-bytes 2000000)
	$(call clip,building,-capture-ticks 600 -capture-zoom 1.5 -capture-frames 240)
	$(call clip,combat,-capture-ui combat -capture-frames 180,--quality 50)
	$(call clip,raid,-scenario raid -capture-ui raid -capture-ticks 500 -capture-frames 300)
	$(call clip,animals,-scenario animals -capture-ui pen -capture-frames 240)
	$(call clip,melee,-scenario armory -capture-ui raid -capture-zoom 1.6 -capture-ticks 560 -capture-frames 240)
	$(call clip,chores,-scenario chores -capture-ui follow -capture-zoom 1.8 -capture-ticks 4000 -capture-frames 240)
	$(call clip,rain,-scenario rain -capture-ui weather -capture-zoom 1.4 -capture-frames 240,--quality 50)
	$(call clip,snow,-scenario snow -capture-ui weather -capture-zoom 1.4 -capture-ticks 12000 -capture-frames 240)
	$(call clip,grass,-capture-ui grass -capture-zoom 2 -capture-ticks 600 -capture-frames 240)
	$(call clip,burial,-scenario burial -capture-ui burial -capture-zoom 1.6 -capture-ticks 30 -capture-frames 300 -capture-speed 3)
	$(call clip,daycycle,-scenario sunny -capture-ui weather -capture-zoom 1.2 -capture-timelapse 250 -capture-frames 240,--fps 20 --quality 50 --max-bytes 2400000)
	$(call clip,roofs,-scenario roofs -hour 15 -capture-ui roofs -capture-ticks 7500 -capture-frames 240,--quality 50)
	$(call clip,zoo,-seed 2 -width 80 -height 60 -scenario zoo -hour 11 -capture-ui zoo -capture-zoom 0.85 -capture-frames 240,--quality 50 --max-bytes 2400000)
	$(call clip,shadows,-scenario sunny -hour 6 -capture-ui follow -capture-zoom 2 -capture-timelapse 125 -capture-frames 240,--fps 20 --quality 50 --max-bytes 2400000)

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)
