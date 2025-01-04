.PHONY: all run-docker gen build test lint clean dev build-dev run

ifeq ($(OS),Windows_NT)
  EXE 	:= .exe
  RM 		= del
	CMD 	:= .cmd
else
  EXE 	:=
  RM 		= rm -f
	CMD 	:=
endif

VERSION         := 3.0.0
BUILD_DIR       := bin
MAIN_PACKAGE    := ./cmd
ENV 						?= production
BINARY_NAME   	:= main

BUILDPATH       = $(BUILD_DIR)/$(BINARY_NAME)$(EXE)
LDFLAGS 				= -ldflags "-w -s -X main.Version=$(VERSION) -X main.Env=$(ENV)"

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

dev: build-dev
	./$(BUILDPATH)

build-dev: ENV=development
build-dev:
	go build $(LDFLAGS) -o $(BUILDPATH) $(MAIN_PACKAGE)

run-docker:
	docker compose -p rolando3 up -d --build --force-recreate