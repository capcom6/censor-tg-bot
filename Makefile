# Go parameters
GOCMD=go
AIRCMD=air
GORUN=$(GOCMD) run
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=telegram-bot

all: test build

air:
	$(AIRCMD)

build:
	$(GOBUILD) -o $(BINARY_NAME) -v

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

run:
	$(GORUN) .

deps:
	$(GOCMD) mod download

.PHONY: all air build test clean run deps