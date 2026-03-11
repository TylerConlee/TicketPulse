.PHONY: build run dev test lint cover clean docker-build

BINARY := ticketpulse

build:
	go build -o $(BINARY) .

run: build
	./$(BINARY)

dev:
	@which air > /dev/null 2>&1 || (echo "Installing air..." && go install github.com/air-verse/air@latest)
	air

test:
	go test -race ./...

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@echo "---"
	@go tool cover -func=coverage.out | grep total | awk '{print "Total coverage: " $$3}'

lint:
	golangci-lint run ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY) coverage.out

docker-build:
	docker build -t tylerconlee/ticketpulse:latest .
