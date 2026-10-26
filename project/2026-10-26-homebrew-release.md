# Homebrew release

contextfill installs with `brew install bensyverson/tap/contextfill`. The formula, `Formula/contextfill.rb` in [bensyverson/homebrew-tap](https://github.com/bensyverson/homebrew-tap), builds from the tagged source tarball with Go and puts the OpenCode plugin in `$(brew --prefix)/opt/contextfill/share/contextfill/`, with a caveat that says how to link it.

## Decisions

- **A version constant in source** (`cmd/contextfill/version.go`), not ldflags. `contextfill --version` is right however it was built (`go install`, `make install`, Homebrew), and the release script bumps it in a commit the tag then points at.
- **One release script, in the tap.** Woodcase had a resumable `scripts/release` (bump, check, tag, hash the tarball, rewrite the formula, `brew test` and `brew audit --strict`). Porting it showed that only five things differ per tool: the formula, the tag's name, the version file, the version line and the check command. Two copies of a 300-line script and its test would drift, so the script and its test (`scripts/test-release`) moved to the tap, and each tool describes itself in `scripts/release.conf`. Each tool's own `scripts/release` is a wrapper that runs the tap's copy, checked out beside it at `../homebrew-tap`.
- **`make check` is contextfill's check**: gofmt, `go vet`, `go test`. The OpenCode tests bind a local port, so a release runs outside the sandbox.

## Releasing

```
scripts/release 0.2.0        # bump, check, tag, push, update the formula, brew test and audit
scripts/release --dry-run    # the plan, changing nothing
```

`scripts/release --help` (the tap's) has the steps, the `release.conf` keys and the exit codes.
