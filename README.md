# contextfill

> Tells an AI coding assistant how much of its memory for this conversation is used up.

A [Claude Code](https://claude.com/claude-code) or [OpenCode](https://opencode.ai) session can only hold so much in its context window, and its reasoning gets looser as the window fills. `contextfill` reads the session's own record and reports how full the window is, so the assistant can check before it starts more work, or decide it's time to write a handoff. It also reports each subagent the session started, which helps when deciding whether to resume one or hand its work to a fresh agent.

## Install

```
brew install bensyverson/tap/contextfill
```

On macOS 15 or later with Apple silicon, that pours a prebuilt, signed binary; anywhere else it builds from source with Go. Homebrew also installs [the OpenCode plugin](#after-every-commit); `brew info contextfill` shows where. Or, with Go, `go install github.com/bensyverson/contextfill/cmd/contextfill@latest`, or `make install` from a clone.

## Quick start

From a shell inside Claude Code or OpenCode, with no arguments, it reads the current session:

```
$ contextfill
23% of 1M · 234.5k tokens · claude-opus-5-5 · session 5f3a9c21
```

Add `--agents` for the session's subagents, one row each:

```
$ contextfill --agents
31% of 1M · 312k tokens · claude-opus-5-5 · session 5f3a9c21
  a2c41e7b9d03f6158  37% of 1M    finished  opus    Add the export menu
  a7f0b3d95e12c4a86  12% of 200k  running   sonnet  Write the settings docs
```

`--max 40` exits 1 when the fill is over 40%, so a script or a hook can branch on it. `--json` gives the same reading, with every subagent, as JSON. `contextfill <session-id>` reads another session, and `--harness claude` (or `claude-code`) or `--harness opencode` picks whose. `contextfill --version` prints its version, and `contextfill --help` has the details.

## After every commit

As a Claude Code hook or an OpenCode plugin, it tells the model its fill each time a Bash call commits, however the commit is chained (`git add -A && git commit …`, `git -C <repo> commit`, a subshell). Inside a subagent it reports that agent's own fill. To set it up:

```
contextfill hook install
```

That sets up each app it finds (`~/.claude`, `~/.config/opencode`); `--harness claude-code` or `--harness opencode` picks one. Run it again any time: a second run changes nothing, and it prints one line per change. `contextfill hook uninstall` takes it out again.

In Claude Code it adds a PostToolUse hook on Bash to `~/.claude/settings.json` (or `$CLAUDE_CONFIG_DIR/settings.json`), keeping every other setting in order:

```json
{ "matcher": "Bash", "hooks": [{ "type": "command", "command": "/opt/homebrew/bin/contextfill hook --max 40 --main-max 33", "timeout": 10 }] }
```

It names the binary by its full path, since a session started before Homebrew was installed has no `/opt/homebrew/bin` on its `PATH`; under Homebrew that's the prefix's `bin` link, not the versioned Cellar path an upgrade deletes. An older contextfill entry, wherever its binary lived, is replaced rather than joined. A settings file that isn't valid JSON is left alone, and the command exits 4. `--max 40` adds a note once the fill passes 40%. `--main-max 33` gives the main thread (the session you talk to, not a subagent) its own, earlier limit; without it the main thread uses `--max` too. Pass either to `hook install` to change them. Any other command gets no output.

In OpenCode, the plugin does the same. `hook install` links the plugin Homebrew installed into OpenCode's global plugins directory, `~/.config/opencode/plugins/contextfill.js`; then run `opencode reload` so OpenCode's background server loads it. A contextfill installed without the plugin (with `go install`, say) says so and changes nothing. From a clone, link it yourself:

```
ln -s "$PWD/opencode/plugin/contextfill.js" ~/.config/opencode/plugins/contextfill.js
```

The limits (`--max 40 --main-max 33`) are at the top of [the plugin](opencode/plugin/contextfill.js); to change them in a Homebrew install, copy the plugin rather than linking it, since an upgrade replaces it. `hook install` leaves such a copy alone. Set `CONTEXTFILL` to the binary's full path if `contextfill` isn't on the `PATH` OpenCode runs with. A subagent's session is judged against `--max`, the main session against `--main-max`.

## How it works

Claude Code keeps each session's transcript at `~/.claude/projects/<project>/<session id>.jsonl` (under `$CLAUDE_CONFIG_DIR` if that's set), and sets `CLAUDE_CODE_SESSION_ID` in every shell it starts. Each assistant turn records the tokens it was sent. The fill is the input side of the latest turn (`input_tokens + cache_read_input_tokens + cache_creation_input_tokens`) divided by the window Claude Code gives the requested model. [Claude Code's docs](https://code.claude.com/docs/en/model-config) give Opus 4.7 and later, Sonnet 5 and later, Haiku 5.5 and the Fable models a million tokens by default, and any other model a million only when it is requested with the `[1m]` suffix; everything else gets 200k. Setting `CLAUDE_CODE_DISABLE_1M_CONTEXT=1` holds every model to 200k, in Claude Code and in `contextfill` alike. Subagents' transcripts sit beside it in `<session id>/subagents/`.

The transcript format belongs to Claude Code and isn't documented, so a Claude Code update can change it; `contextfill` exits 4 rather than guess when it can't find a reading.

OpenCode sets `OPENCODE=1` and `OPENCODE_SESSION_ID` in its shells, and `contextfill` checks for it first: an OpenCode started from a Claude Code shell inherits Claude Code's variables. OpenCode keeps sessions in a private database, and only its running server knows every model's window, so `contextfill` asks that server, over the same local HTTP API OpenCode's own clients use. It finds the server and its password in `~/.local/state/opencode/service.json` (under `$XDG_STATE_HOME` if that's set), and reads only the newest few steps and the model list. The fill is the newest finished step's `input + cache.read + cache.write` over the model's `limit.context`. OpenCode's own footer adds that step's output, so it reads slightly higher. A model whose window the server doesn't know reads "window unknown" rather than a guess. [The findings](project/2026-10-24-opencode-findings.md) have the details.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Reported, or `hook install` did its work or had none to do |
| 1 | The fill is over `--max` |
| 2 | Usage error |
| 4 | The session is missing or holds no reading, or `--max` was given for a model whose window is unknown, or `hook install` met a settings file that isn't valid JSON |
| 5 | No session to read (`CLAUDE_CODE_SESSION_ID` or `OPENCODE_SESSION_ID` isn't set and no id was given), or OpenCode's server isn't running, or `hook install` found neither app |

---

By [Ben Syverson](https://github.com/bensyverson/). Licensed under the [MIT License](LICENSE).
