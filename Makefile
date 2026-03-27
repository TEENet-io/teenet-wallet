.PHONY: build test lint docker frontend clean

build:
	CGO_ENABLED=1 go build -o teenet-wallet .

test:
	go test ./... -v -race

lint:
	go vet ./...
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"

# Build frontend locally and copy to ./frontend/
frontend:
	git submodule update --init --recursive
	cd frontend-src && npm ci --silent && npm run build
	rm -rf frontend
	cp -r frontend-src/dist frontend

# Build Docker image (frontend + backend in one image)
docker:
	git submodule update --init --recursive
	docker build -t teenet-wallet:latest .

clean:
	rm -f teenet-wallet
	rm -rf frontend
