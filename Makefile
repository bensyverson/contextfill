BINARY := contextfill
PKG    := ./cmd/contextfill

.PHONY: build install test fmt fix vet clean hooks help

build:
	go build -o $(BINARY) $(PKG)

install:
	go install $(PKG)

test:
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
	@echo "Targets: build install test fmt fix vet clean hooks"
