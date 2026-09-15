.PHONY: build lint test vet fmt configs up down logs

# Go workspaces (go.work) don't support a single "./..." from the repo
# root — each module's build list is its own, so every target below loops
# over shared/ and each services/<name>/ module individually. See
# docs/architecture/folder-structure.md for why the repo is laid out this
# way.
MODULES := shared $(wildcard services/*)

build:
	@for m in $(MODULES); do (cd $$m && go build ./...) || exit 1; done

vet:
	@for m in $(MODULES); do (cd $$m && go vet ./...) || exit 1; done

fmt:
	gofmt -l shared services

lint:
	@for m in $(MODULES); do (cd $$m && golangci-lint run ./...) || exit 1; done

test:
	@for m in $(MODULES); do (cd $$m && go test ./...) || exit 1; done

# Seeds deployments/configs/<service>.json from
# deployments/configs/<service>.template.json for every service that
# doesn't already have one — needed once before
# `go run ./services/<service>/cmd` (docker compose has its own
# deployments/configs/docker/ copies checked into git, so it doesn't need
# this).
configs:
	@for f in deployments/configs/*.template.json; do \
		dst=$${f%.template.json}.json; \
		if [ ! -f "$$dst" ]; then cp "$$f" "$$dst"; echo "wrote $$dst"; fi; \
	done

# Phase 1 quickstart — see README.md for the curl flow to run after this.
up:
	docker compose -f deployments/docker-compose.yaml up --build

down:
	docker compose -f deployments/docker-compose.yaml down -v
