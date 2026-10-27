# The window comes from the model name, not a `[1m]` suffix

2026-10-26 · finding and decision

## What broke

Up to Claude Code 2.1.310, an Opus 5.5 session at 1M recorded `requestedModel: "claude-opus-5-5[1m]"`, and `transcript.WindowFor` read the suffix as its only sign of a 1M window. From 2.1.311 the same session records a bare `claude-opus-5-5`. So `contextfill` reported a 1M session as a fraction of 200k, overstating its fill five times over.

Surveyed with `jq -r 'select(.type=="assistant" and .isSidechain==false) | [.version, .requestedModel] | @tsv'` over the newest few hundred transcripts in `~/.claude/projects`: every 2.1.296–2.1.310 Opus 5.5 session carries `[1m]`, and every 2.1.311 one doesn't.

## Nothing per-session records the window

- **The transcript:** no field on an assistant line or a logged `api-request` names the window. The request's `betas` carry no 1M flag, and the params of a 1M session match a 200k-default one's.
- **Hook payloads:** no hook event documents a window field (https://code.claude.com/docs/en/hooks).
- **Environment:** no variable names the model or window for commands Claude Code runs (https://code.claude.com/docs/en/env-vars).
- **`~/.claude/settings.json`:** `model` is global and can be overridden per project or by `/model`, so it doesn't describe a given session. The `contextWindow` fields in `stats-cache.json` are all zero.
- **The statusline:** its stdin JSON has `context_window.context_window_size`, the one exact per-session source (https://code.claude.com/docs/en/statusline). But Claude Code only sends it to the statusline command, and `contextfill` isn't one.

## Decision

`WindowFor` applies the rule from Claude Code's docs ("Extended context", https://code.claude.com/docs/en/model-config): Fable 5 and 5.1, Sonnet 5 and later, Haiku 5.5, and Opus 4.7 and later run at 1M by default, on every provider and plan. Any other model is 1M only with the `[1m]` suffix, which also still covers older transcripts. `CLAUDE_CODE_DISABLE_1M_CONTEXT=1` holds every model to 200k, as it does in Claude Code. `contextfill` inherits that variable from Claude Code's shell and hooks.

Known gaps: a gateway that enforces a lower limit than the model's, which Claude Code can't detect either, and a future family the table doesn't name, which reads 200k.

**What would reopen this:** Claude Code recording the window in the transcript or a hook payload. Read it from there and retire the table.
