# contextfill

> Tells an AI coding assistant how much of its memory for this conversation is used up.

A Claude Code session can only hold so much in its context window, and its reasoning gets looser as the window fills. `contextfill` reads the session's own transcript and reports how full the window is, so the assistant can check before it starts more work, or decide it's time to write a handoff. It also reports each subagent the session started, which helps when deciding whether to resume one or hand its work to a fresh agent.

## Install

```
go install github.com/bensyverson/contextfill/cmd/contextfill@latest
```

Or, from a clone, `make install`.

## Quick start

From a shell inside Claude Code, with no arguments, it reads the current session:

```
$ contextfill
23% of 1M · 234.5k tokens · claude-opus-5-5[1m] · session 5f3a9c21
```

Add `--agents` for the session's subagents, one row each:

```
$ contextfill --agents
31% of 1M · 312k tokens · claude-opus-5-5[1m] · session 5f3a9c21
  a2c41e7b9d03f6158  37% of 1M    finished  opus    Add the export menu
  a7f0b3d95e12c4a86  12% of 200k  running   sonnet  Write the settings docs
```

`--max 40` exits 1 when the fill is over 40%, so a script or a hook can branch on it. `--json` gives the same reading, with every subagent, as JSON. `contextfill <session-id>` reads another session. `contextfill --help` has the details.

## How it works

Claude Code keeps each session's transcript at `~/.claude/projects/<project>/<session id>.jsonl` (under `$CLAUDE_CONFIG_DIR` if that's set), and sets `CLAUDE_CODE_SESSION_ID` in every shell it starts. Each assistant turn records the tokens it was sent. The fill is the input side of the latest turn (`input_tokens + cache_read_input_tokens + cache_creation_input_tokens`) divided by the model's window: a million tokens when the requested model carries the `[1m]` suffix, otherwise 200k. Subagents' transcripts sit beside it in `<session id>/subagents/`.

The transcript format belongs to Claude Code and isn't documented, so a Claude Code update can change it; `contextfill` exits 4 rather than guess when it can't find a reading.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Reported |
| 1 | The fill is over `--max` |
| 2 | Usage error |
| 4 | The transcript is missing or holds no reading |
| 5 | No session to read: `CLAUDE_CODE_SESSION_ID` isn't set and no id was given |

---

By [Ben Syverson](https://github.com/bensyverson/). Licensed under the [MIT License](LICENSE).
