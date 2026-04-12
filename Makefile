.PHONY: all build run dev lint clean vosk dave run-docker

VERSION      := 4.0.0
BUILD_DIR    := bin
MAIN_PACKAGE := ./cmd
BINARY_NAME  := main
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

$(BUILDPATH): $(BUILD_DIR) vosk dave
	@echo "[build] compiling $(BINARY_NAME) $(VERSION) (env=$(ENV))"
	$(CGO_FLAGS) go build $(LDFLAGS) -o $(BUILDPATH) $(MAIN_PACKAGE)
	@echo "[build] done -> $(BUILDPATH)"

build: $(BUILDPATH)
build-force:
	@rm -rf $(BUILD_DIR)
	$(MAKE) build

run: $(BUILDPATH)
	@echo "[run] starting $(BINARY_NAME)"
	$(RUNTIME_FLAGS) ./$(BUILDPATH)

dev: ENV=development
dev: LDFLAGS=$(LDFLAGS_DEV)
dev: build-force run

lint:
	@echo "[lint] formatting"
	@go fmt ./...
	@echo "[lint] static analysis"
	@staticcheck ./...

clean:
	@echo "[clean] removing build artifacts"
	@go clean
	@rm -rf $(BUILD_DIR) vosk dave

run-docker:
ifeq ($(ENV),production)
	docker compose -p rolando --profile prod up -d --build --force-recreate
else
	docker compose -p rolando up -d --build --force-recreate
endif

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
