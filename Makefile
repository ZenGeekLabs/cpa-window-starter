.PHONY: test build demo

test:
	go test -race ./...
	node --test tests/*.test.cjs

build:
	sh scripts/build.sh

demo:
	go run ./cmd/demo
