.PHONY: all run-docker build clean dev run lint clean

VERSION         := 3.9.2
BUILD_DIR       := bin
MAIN_PACKAGE    := ./cmd
ENV             ?= production
BINARY_NAME     := main

VOSK_LIB_DOWNLOAD := https://github.com/alphacep/vosk-api/releases/download
VOSK_LIB_RELEASE := v0.3.45

VOSK_MODELS_DOWNLOAD := https://alphacephei.com/vosk/models
VOSK_MODEL_EN := vosk-model-small-en-us-0.15
VOSK_MODEL_IT := vosk-model-small-it-0.22
VOSK_MODEL_DE := vosk-model-small-de-0.15
VOSK_MODEL_ES := vosk-model-small-es-0.42

ifeq ($(OS),Windows_NT)
  EXE   := .exe
  RM     = del
  CMD   := .cmd
  VOSK_ARCHIVE := vosk-win64-0.3.45
else
  EXE   :=
  RM     = rm -rf
  CMD   :=
  VOSK_ARCHIVE := vosk-linux-x86_64-0.3.45
endif

VOSK_LIB_URL := $(VOSK_LIB_DOWNLOAD)/$(VOSK_LIB_RELEASE)/$(VOSK_ARCHIVE).zip
VOSK_LIB := vosk/lib
VOSK_LIB_PATH := $(PWD)/$(VOSK_LIB)
VOSK_MODELS := vosk/models
VOSK_MODELS_PATH := $(PWD)/$(VOSK_MODELS)

# tells the linker where to find libvosk.so at RUN TIME
LD_LIBRARY_PATH := $(VOSK_LIB_PATH)
# tells the compiler (CGO preprocessor) where to find vosk_api.h at COMPILE TIME
CGO_CPPFLAGS := -I $(VOSK_LIB_PATH)
# tells the CGO linker where to find libvosk.so at COMPILE* TIME (*link time actually)
CGO_LDFLAGS := -L $(VOSK_LIB_PATH) -lvosk -lpthread -dl

CGO_FLAGS = CGO_CPPFLAGS="$(CGO_CPPFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)"
RUNTIME_LD_FLAGS = GO_ENV=$(ENV) LD_LIBRARY_PATH=$(LD_LIBRARY_PATH)

BUILDPATH       = $(BUILD_DIR)/$(BINARY_NAME)$(EXE)
LDFLAGS         = -ldflags "-w -s -X main.Version=$(VERSION)"
LDFLAGS_DEV     = -ldflags "-X main.Version=$(VERSION)"

all: dev

build: vosk
	$(CGO_FLAGS) go build $(LDFLAGS) -o $(BUILDPATH) $(MAIN_PACKAGE)

$(BUILDPATH): vosk
	$(CGO_FLAGS) go build $(LDFLAGS) -o $(BUILDPATH) $(MAIN_PACKAGE)

lint:
	go fmt ./...
	staticcheck ./...

clean:
	go clean
	$(RM) $(BUILD_DIR)
	$(RM) vosk

run: $(BUILDPATH)
	$(RUNTIME_LD_FLAGS) ./$(BUILDPATH)

dev: ENV=development
dev: build run

run-docker:
ifeq ($(ENV),production)
	docker compose -p rolando --profile prod up -d --build --force-recreate
else
	docker compose -p rolando up -d --build --force-recreate
endif

vosk: $(VOSK_LIB) $(VOSK_MODELS)

$(VOSK_LIB):
	@rm -rf vosk/lib
	@mkdir -p vosk
	@wget $(VOSK_LIB_URL)
	@unzip $(VOSK_ARCHIVE).zip -d vosk
	@mv vosk/$(VOSK_ARCHIVE)/ vosk/lib
	@rm $(VOSK_ARCHIVE).zip

$(VOSK_MODELS):
	@rm -rf vosk/models
	@mkdir -p vosk/models

	@wget $(VOSK_MODELS_DOWNLOAD)/$(VOSK_MODEL_EN).zip
	@unzip $(VOSK_MODEL_EN).zip -d vosk/models
	@mv vosk/models/$(VOSK_MODEL_EN)/ vosk/models/en
	@rm $(VOSK_MODEL_EN).zip

	@wget $(VOSK_MODELS_DOWNLOAD)/$(VOSK_MODEL_IT).zip
	@unzip $(VOSK_MODEL_IT).zip -d vosk/models
	@mv vosk/models/$(VOSK_MODEL_IT)/ vosk/models/it
	@rm $(VOSK_MODEL_IT).zip

	@wget $(VOSK_MODELS_DOWNLOAD)/$(VOSK_MODEL_ES).zip
	@unzip $(VOSK_MODEL_ES).zip -d vosk/models
	@mv vosk/models/$(VOSK_MODEL_ES)/ vosk/models/es
	@rm $(VOSK_MODEL_ES).zip

	@wget $(VOSK_MODELS_DOWNLOAD)/$(VOSK_MODEL_DE).zip
	@unzip $(VOSK_MODEL_DE).zip -d vosk/models
	@mv vosk/models/$(VOSK_MODEL_DE)/ vosk/models/de
	@rm $(VOSK_MODEL_DE).zip


