BINARY := contextfill
PKG    := ./cmd/contextfill

.PHONY: build install test check fmt fix vet clean hooks help

build:
	go build -o $(BINARY) $(PKG)

install:
	go install $(PKG)

test:
	go test ./...

# What scripts/release runs before it tags: formatting, vet, the suite. Prunes .claude,
# as the pre-commit hook does, so an agent's worktree can't fail it.
check:
	@unformatted=$$(find . -path ./.claude -prune -o -name '*.go' -print0 | xargs -0 gofmt -l); \
	if [ -n "$$unformatted" ]; then echo "gofmt needs to format: $$unformatted"; exit 1; fi
	go vet ./...
	go test ./...

fmt:
	go fmt ./...

fix:
	go fix ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY)

# Point git at the checked-in hooks (one-time per clone).
hooks:
	git config core.hooksPath scripts/git-hooks

help:
	@echo "Targets: build install test check fmt fix vet clean hooks"
