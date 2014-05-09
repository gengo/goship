.PHONY: all build

all: build

install:
	godep restore

build:
	godep go build goship.go

clean:
	rm goship
