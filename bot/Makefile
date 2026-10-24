.PHONY: all build run dev test lint clean vosk dave pico pico-gen

VERSION      ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BUILD_DIR    := bin
MAIN_PACKAGE := ./cmd/rolando
BINARY_NAME  := rolando
ENV          ?= production

BUILDPATH    := $(BUILD_DIR)/$(BINARY_NAME)

# ── Download ──────────────────────────────────────────────────────────────────

CURL := curl -fsSL

# ── DAVE ──────────────────────────────────────────────────────────────────────

DAVE_DIR     := dave
DAVE_INCLUDE := $(DAVE_DIR)/include
DAVE_LIB_DIR := $(DAVE_DIR)/lib
DAVE_LIB     := $(DAVE_LIB_DIR)/libdave.so
DAVE_LIB_URL := https://github.com/discord/libdave/releases/download/v1.1.1%2Fcpp/libdave-Linux-X64-boringssl.zip

# ── VOSK ──────────────────────────────────────────────────────────────────────

VOSK_LIB_RELEASE  := v0.3.45
VOSK_ARCHIVE      := vosk-linux-x86_64-0.3.45
VOSK_LIB_URL      := https://github.com/alphacep/vosk-api/releases/download/$(VOSK_LIB_RELEASE)/$(VOSK_ARCHIVE).zip

VOSK_LIB          := vosk/lib
VOSK_LIB_PATH     := $(PWD)/$(VOSK_LIB)
VOSK_MODELS       := vosk/models
VOSK_MODELS_PATH  := $(PWD)/$(VOSK_MODELS)

VOSK_MODELS_BASE  := https://alphacephei.com/vosk/models
VOSK_MODEL_EN     := vosk-model-small-en-us-0.15
VOSK_MODEL_IT     := vosk-model-small-it-0.22
VOSK_MODEL_DE     := vosk-model-small-de-0.15
VOSK_MODEL_ES     := vosk-model-small-es-0.42

# ── PICO ──────────────────────────────────────────────────────────────────────

# SVOX Pico, transpiled to pure Go (internal/tts/pico, regenerate with pico-gen);
# only the voice files are downloaded
PICO_REPO        := https://android.googlesource.com/platform/external/svox/+
PICO_COMMIT      := c5a33e4fdc55b63d4aa33aecefcc91b63da921e7
PICO_LANG        := pico/lang
PICO_VOICES      := en-GB_ta.bin en-GB_kh0_sg.bin it-IT_ta.bin it-IT_cm0_sg.bin \
                    de-DE_ta.bin de-DE_gl0_sg.bin es-ES_ta.bin es-ES_zl0_sg.bin
PICO_GEN         := internal/tts/pico/ccgo_linux_amd64.go
CCGO             := modernc.org/ccgo/v4@v4.38.0

# ── CGO ───────────────────────────────────────────────────────────────────────

DAVE_LIB_PATH    := $(PWD)/$(DAVE_LIB_DIR)
DAVE_PKGCONFIG   := $(DAVE_LIB_PATH)/pkgconfig

CGO_CPPFLAGS := -I$(VOSK_LIB_PATH) -I$(DAVE_LIB_PATH) -I$(PWD)/$(DAVE_INCLUDE)
CGO_LDFLAGS  := -L$(VOSK_LIB_PATH) -lvosk -L$(DAVE_LIB_PATH) -ldave -lpthread -ldl

CGO_FLAGS     = PKG_CONFIG_PATH="$(DAVE_PKGCONFIG)" CGO_CPPFLAGS="$(CGO_CPPFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)"
RUNTIME_FLAGS = GO_ENV=$(ENV) LD_LIBRARY_PATH=$(VOSK_LIB_PATH):$(DAVE_LIB_PATH)

LDFLAGS     := -ldflags "-w -s -X main.Version=$(VERSION)"
LDFLAGS_DEV := -ldflags "-X main.Version=$(VERSION)"

# ── Targets ───────────────────────────────────────────────────────────────────

all: dev

$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

$(BUILDPATH): $(BUILD_DIR) vosk dave pico
	@echo "[build] compiling $(BINARY_NAME) $(VERSION) (env=$(ENV))"
	$(CGO_FLAGS) go build $(LDFLAGS) -o $(BUILDPATH) $(MAIN_PACKAGE)
	@echo "[build] done -> $(BUILDPATH)"

build: $(BUILDPATH)
build-force:
	@rm -rf $(BUILD_DIR)
	$(MAKE) build

run: $(BUILDPATH)
	@echo "[run] starting $(BINARY_NAME)"
	set -a; [ ! -f .env ] || . ./.env; set +a; $(RUNTIME_FLAGS) ./$(BUILDPATH)

dev: ENV=development
dev: LDFLAGS=$(LDFLAGS_DEV)
dev: build-force run

test: vosk dave pico
	$(CGO_FLAGS) $(RUNTIME_FLAGS) go test ./...

lint:
	@echo "[lint] formatting"
	@go fmt ./...
	@echo "[lint] static analysis"
	@staticcheck ./...

clean:
	@echo "[clean] removing build artifacts"
	@go clean
	@rm -rf $(BUILD_DIR) vosk dave pico

# ── DAVE ──────────────────────────────────────────────────────────────────────

dave: $(DAVE_LIB)

$(DAVE_LIB):
	@echo "[dave] downloading libdave"
	@mkdir -p $(DAVE_INCLUDE) $(DAVE_LIB_DIR) $(DAVE_PKGCONFIG) $(DAVE_DIR)/.tmp
	@$(CURL) "$(DAVE_LIB_URL)" -o $(DAVE_DIR)/.tmp/libdave.zip
	@echo "[dave] extracting"
	@unzip -qo $(DAVE_DIR)/.tmp/libdave.zip -d $(DAVE_DIR)/.tmp/extract
	@mv $(DAVE_DIR)/.tmp/extract/lib/libdave.so $(DAVE_LIB)
	@cp -r $(DAVE_DIR)/.tmp/extract/include/dave/* $(DAVE_INCLUDE)/
	@rm -rf $(DAVE_DIR)/.tmp
	@echo "[dave] generating dave.pc"
	@printf 'prefix=$(DAVE_LIB_PATH)\nName: dave\nDescription: libdave\nVersion: 1.1.1\nLibs: -L$${prefix} -ldave\nCflags: -I$(PWD)/$(DAVE_INCLUDE)\n' \
		> $(DAVE_PKGCONFIG)/dave.pc
	@echo "[dave] ready"

# ── VOSK ──────────────────────────────────────────────────────────────────────

vosk: $(VOSK_LIB) $(VOSK_MODELS)

$(VOSK_LIB):
	@echo "[vosk] downloading library $(VOSK_LIB_RELEASE)"
	@mkdir -p vosk/.tmp
	@$(CURL) "$(VOSK_LIB_URL)" -o vosk/.tmp/vosk-lib.zip
	@unzip -q vosk/.tmp/vosk-lib.zip -d vosk/.tmp
	@mv vosk/.tmp/$(VOSK_ARCHIVE) $(VOSK_LIB)
	@rm -rf vosk/.tmp
	@echo "[vosk] library ready"

define download-model
	@echo "[vosk] downloading model $(1) -> $(2)"
	@$(CURL) "$(VOSK_MODELS_BASE)/$(1).zip" -o $(VOSK_MODELS)/.tmp/$(1).zip
	@unzip -q $(VOSK_MODELS)/.tmp/$(1).zip -d $(VOSK_MODELS)/.tmp
	@mv $(VOSK_MODELS)/.tmp/$(1) $(VOSK_MODELS)/$(2)
	@rm $(VOSK_MODELS)/.tmp/$(1).zip
endef

$(VOSK_MODELS):
	@echo "[vosk] downloading models"
	@mkdir -p $(VOSK_MODELS)/.tmp
	$(call download-model,$(VOSK_MODEL_EN),en)
	$(call download-model,$(VOSK_MODEL_IT),it)
	$(call download-model,$(VOSK_MODEL_DE),de)
	$(call download-model,$(VOSK_MODEL_ES),es)
	@rm -rf $(VOSK_MODELS)/.tmp
	@echo "[vosk] all models ready"

# ── PICO ──────────────────────────────────────────────────────────────────────

pico: $(PICO_LANG)

$(PICO_LANG):
	@echo "[pico] downloading voices"
	@mkdir -p $(PICO_LANG).tmp
	@set -e; for f in $(PICO_VOICES); do \
		$(CURL) "$(PICO_REPO)/$(PICO_COMMIT)/pico/lang/$$f?format=TEXT" -o $(PICO_LANG).tmp/$$f.b64; \
		base64 -d $(PICO_LANG).tmp/$$f.b64 > $(PICO_LANG).tmp/$$f; \
		rm $(PICO_LANG).tmp/$$f.b64; \
	done
	@mv $(PICO_LANG).tmp $(PICO_LANG)
	@echo "[pico] voices ready"

# The C sources need two fixes for ccgo: a missing <stdint.h> and GCC-only
# '\x010'-style character constants.
pico-gen:
	@echo "[pico] transpiling svox $(PICO_COMMIT) to $(PICO_GEN)"
	@rm -rf pico/.src && mkdir -p pico/.src $(dir $(PICO_GEN))
	@$(CURL) "$(PICO_REPO)archive/$(PICO_COMMIT)/pico/lib.tar.gz" | tar xz -C pico/.src
	@sed -i '1i #include <stdint.h>' $$(grep -l uintptr_t pico/.src/*.c)
	@sed -i -E "s/'\\\\x0([1248])0'/0x\10/" pico/.src/picoktab.c
	@cd pico/.src && go run $(CCGO) --package-name pico -o $(CURDIR)/$(PICO_GEN) -I. *.c
	@rm -rf pico/.src
	@echo "[pico] generated $(PICO_GEN)"
