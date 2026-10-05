.PHONY: test build demo

test:
	go test -race ./...
	node --test tests/*.test.cjs
	python3 -m unittest discover -s tests -p 'test_*.py'

build:
	sh scripts/build.sh

demo:
	go run ./cmd/demo
