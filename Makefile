.PHONY: gen lint tidy

gen:
	buf generate

lint:
	buf lint

tidy:
	go mod tidy

all: lint gen tidy
