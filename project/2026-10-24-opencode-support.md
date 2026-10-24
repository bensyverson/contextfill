# 2026-10-24 — OpenCode support: what's known before starting

`contextfill` reads Claude Code transcripts today. Supporting OpenCode would let the shared rules (`agents-md` `core` or `delegation`) tell any agent to check its fill, not just one running in Claude Code. Nothing below was verified against OpenCode's files yet: the session that built the tool reached its own 40% line first.

## Known

- `opencode` is installed on Ben's machine, but `which opencode` finds nothing inside Claude Code's sandbox, and `ls ~/.local/share/opencode` and `ls ~/.config/opencode` fail with "Operation not permitted". The sandbox denies reads under `~/` outside its allowlist, so investigate with the sandbox off for those calls, or add a read rule.
- Ben suggests driving it through `tmux` to get a live session to inspect.

## To find out, in order

1. **Where a session's messages live** and which file holds the latest assistant turn's token counts. Expect per-message JSON with input, output and cache read/write counts; confirm the field names from a real file rather than from memory.
2. **How a shell inside OpenCode names its session**, the equivalent of `CLAUDE_CODE_SESSION_ID`. If there's no variable, find the most reliable fallback, and say what it costs (for example "the most recently written session in this project" breaks with parallel sessions).
3. **How it names the model and its window**, the equivalent of Claude Code's `[1m]` suffix.
4. **Whether it has subagents and hooks**, and what a hook receives.

## Shape

Keep `transcript` Claude Code's reader, and add an `opencode` package beside it behind one small interface: find a session, latest reading, list agents. `cmd/contextfill` picks the reader from the environment (`CLAUDECODE=1`, or OpenCode's own marker) and takes a `--harness` override. Fixtures stay synthetic: the repo is public, and transcripts are private.
