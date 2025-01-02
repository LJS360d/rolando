.PHONY: all gen build test lint clean dev build-dev run

ifeq ($(OS),Windows_NT)
  EXE 	:= .exe
  RM 		= del
else
  EXE 	:=
  RM 		= rm -f
endif

VERSION         := 3.0.0
BUILD_DIR       := bin
MAIN_PACKAGE    := ./cmd
BUILD						:= $(shell git rev-parse --short HEAD)
ENV 						?= production

ifeq ($(ENV), production)
	BINARY_NAME   = rolando@$(VERSION)
else
	BINARY_NAME   = dev
endif

BUILDPATH       = $(BUILD_DIR)/$(BINARY_NAME)$(EXE)
LDFLAGS 				= -ldflags "-w -s -X main.Version=$(VERSION) -X main.Build=$(BUILD) -X main.Env=$(ENV)"

all: dev

build:
	go build $(LDFLAGS) -o $(BUILDPATH) $(MAIN_PACKAGE)

test:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	go fmt ./...
	staticcheck ./...

clean:
	go clean
	$(RM) $(BUILD_DIR)

run: build
	./$(BUILDPATH)

dev:
	air

build-dev: ENV=development
build-dev:
	go build $(LDFLAGS) -o $(BUILDPATH) $(MAIN_PACKAGE)


GRPC_OUT			:=.
PB_OUT				:=.
GRPC_OPT 			:=paths=source_relative
PB_OPT 				:=paths=source_relative
PROTO 				?= $(shell { git diff --name-only -- '*.proto'; git diff --name-only --cached -- '*.proto'; git ls-files --others --exclude-standard -- '*.proto'; } | sort -u)

gen:
	protoc --go_out=$(PB_OUT) --go_opt=$(PB_OPT) --go-grpc_out=$(GRPC_OUT) --go-grpc_opt=$(GRPC_OPT) $(PROTO)