.PHONY: all build

all: build clean

install:
	gom install

build:
	gom build goship.go

clean:
	rm goship
