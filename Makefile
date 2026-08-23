.PHONY: init test test-race vet run clean

init:
	go mod download
	go mod vendor
	@echo "Dependencies downloaded and vendored."

test:
	go test -mod=vendor ./...

test-race:
	go test -mod=vendor -race ./...

vet:
	go vet -mod=vendor ./...

run:
	go run -mod=vendor ./example

clean:
	rm -rf vendor/
	rm -rf bin/