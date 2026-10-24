# contextfill

> Tells an AI coding assistant how much of its memory for this conversation is used up.

A [Claude Code](https://claude.com/claude-code) or [OpenCode](https://opencode.ai) session can only hold so much in its context window, and its reasoning gets looser as the window fills. `contextfill` reads the session's own record and reports how full the window is, so the assistant can check before it starts more work, or decide it's time to write a handoff. It also reports each subagent the session started, which helps when deciding whether to resume one or hand its work to a fresh agent.

## Install

```
go install github.com/bensyverson/contextfill/cmd/contextfill@latest
```

Or, from a clone, `make install`.

## Quick start

From a shell inside Claude Code or OpenCode, with no arguments, it reads the current session:

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

`--max 40` exits 1 when the fill is over 40%, so a script or a hook can branch on it. `--json` gives the same reading, with every subagent, as JSON. `contextfill <session-id>` reads another session, and `--harness claude` or `--harness opencode` picks whose. `contextfill --help` has the details.

## After every commit

As a Claude Code hook (OpenCode's equivalent, a plugin, isn't built yet), it tells the model its fill each time a Bash call commits, however the commit is chained (`git add -A && git commit …`, `git -C <repo> commit`, a subshell). Inside a subagent it reports that agent's own fill. Add it to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{ "type": "command", "command": "contextfill hook --max 40 --main-max 25", "timeout": 10 }]
      }
    ]
  }
}
```

Use the full path if `contextfill` isn't on the `PATH` hooks run with. `--max 40` adds a nudge to write the handoff once the fill passes 40%. `--main-max 25` gives the main thread (the session you talk to, not a subagent) its own, earlier limit; without it the main thread uses `--max` too. Any other command gets no output.

## How it works

Claude Code keeps each session's transcript at `~/.claude/projects/<project>/<session id>.jsonl` (under `$CLAUDE_CONFIG_DIR` if that's set), and sets `CLAUDE_CODE_SESSION_ID` in every shell it starts. Each assistant turn records the tokens it was sent. The fill is the input side of the latest turn (`input_tokens + cache_read_input_tokens + cache_creation_input_tokens`) divided by the model's window: a million tokens when the requested model carries the `[1m]` suffix, otherwise 200k. Subagents' transcripts sit beside it in `<session id>/subagents/`.

The transcript format belongs to Claude Code and isn't documented, so a Claude Code update can change it; `contextfill` exits 4 rather than guess when it can't find a reading.

OpenCode sets `OPENCODE=1` and `OPENCODE_SESSION_ID` in its shells, and `contextfill` checks for it first: an OpenCode started from a Claude Code shell inherits Claude Code's variables. OpenCode keeps sessions in a private database, and only its running server knows every model's window, so `contextfill` asks that server, over the same local HTTP API OpenCode's own clients use. It finds the server and its password in `~/.local/state/opencode/service.json` (under `$XDG_STATE_HOME` if that's set), and reads only the newest few steps and the model list. The fill is the newest finished step's `input + cache.read + cache.write` over the model's `limit.context`. OpenCode's own footer adds that step's output, so it reads slightly higher. A model whose window the server doesn't know reads "window unknown" rather than a guess. [The findings](project/2026-10-24-opencode-findings.md) have the details.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Reported |
| 1 | The fill is over `--max` |
| 2 | Usage error |
| 4 | The session is missing or holds no reading, or `--max` was given for a model whose window is unknown |
| 5 | No session to read (`CLAUDE_CODE_SESSION_ID` or `OPENCODE_SESSION_ID` isn't set and no id was given), or OpenCode's server isn't running |

---

By [Ben Syverson](https://github.com/bensyverson/). Licensed under the [MIT License](LICENSE).
