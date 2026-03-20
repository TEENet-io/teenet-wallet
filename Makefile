.PHONY: build test lint docker clean

build:
	CGO_ENABLED=1 go build -o teenet-wallet .

test:
	go test ./... -v -race

lint:
	go vet ./...
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"

docker:
	docker build -t teenet-wallet:latest .

clean:
	rm -f teenet-wallet
