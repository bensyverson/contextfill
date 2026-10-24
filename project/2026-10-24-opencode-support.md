# 2026-10-24 — OpenCode support: what's known before starting

`contextfill` reads Claude Code transcripts today. Supporting OpenCode would let the shared rules (`agents-md` `core` or `delegation`) tell any agent to check its fill, not just one running in Claude Code. Nothing below was verified against OpenCode's files yet: the session that built the tool reached its own 40% line first.

## Known

- `opencode` is installed on Ben's machine, but `which opencode` finds nothing inside Claude Code's sandbox, and `ls ~/.local/share/opencode` and `ls ~/.config/opencode` fail with "Operation not permitted". The sandbox denies reads under `~/` outside its allowlist, so investigate with the sandbox off for those calls, or add a read rule.
- Ben suggests driving it through `tmux` to get a live session to inspect.

## To find out, in order

> **2026-10-24, later:** answered in [2026-10-24-opencode-findings.md](2026-10-24-opencode-findings.md). The guess in 1 was wrong: there are no per-message JSON files. Sessions live in a SQLite database, and plugin models' windows are known only to the running server.

1. **Where a session's messages live** and which file holds the latest assistant turn's token counts. Expect per-message JSON with input, output and cache read/write counts; confirm the field names from a real file rather than from memory.
2. **How a shell inside OpenCode names its session**, the equivalent of `CLAUDE_CODE_SESSION_ID`. If there's no variable, find the most reliable fallback, and say what it costs (for example "the most recently written session in this project" breaks with parallel sessions).
3. **How it names the model and its window**, the equivalent of Claude Code's `[1m]` suffix.
4. **Whether it has subagents and hooks**, and what a hook receives.

## Also next: two limits for the hook

The shared rules now draw two lines: a subagent hands off past 40% of its context, and the integrator stops delegating at 25% (`agents-md` `942f807`). The hook has one `--max`, so `contextfill hook --max 40` nudges the main thread late, and `--max 25` would tell agents to hand off early. A subagent's payload carries `agent_id` and the main thread's doesn't, so the hook can tell them apart: add `--main-max <percent>` for payloads without an `agent_id`, falling back to `--max`, then change the hook in `~/.claude/settings.json` to `contextfill hook --max 40 --main-max 25`. Red first, in `hook/hook_test.go` and `cmd/contextfill/run_test.go`.

> **2026-10-24, later:** done as planned; `--main-max` is accepted only by `contextfill hook`. Tracked in [the plan](2026-10-24-plan-hook-limits-and-opencode.md).

## Shape

Keep `transcript` Claude Code's reader, and add an `opencode` package beside it behind one small interface: find a session, latest reading, list agents. `cmd/contextfill` picks the reader from the environment (`CLAUDECODE=1`, or OpenCode's own marker) and takes a `--harness` override. Fixtures stay synthetic: the repo is public, and transcripts are private.
