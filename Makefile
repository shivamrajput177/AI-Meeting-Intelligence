.PHONY: build lint test vet fmt configs up down logs

build:
	go build ./...

vet:
	go vet ./...

fmt:
	gofmt -l .

lint:
	golangci-lint run ./...

test:
	go test ./...

# Seeds configs/<service>.json from configs/<service>.template.json for
# every service that doesn't already have one — needed once before
# `go run ./cmd/<service>` (docker compose has its own configs/docker/
# copies checked into git, so it doesn't need this).
configs:
	@for f in configs/*.template.json; do \
		dst=$${f%.template.json}.json; \
		if [ ! -f "$$dst" ]; then cp "$$f" "$$dst"; echo "wrote $$dst"; fi; \
	done

# Phase 1 quickstart — see README.md for the curl flow to run after this.
up:
	docker compose up --build

down:
	docker compose down -v
