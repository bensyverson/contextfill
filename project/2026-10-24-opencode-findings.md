# 2026-10-24 — OpenCode: where a session's fill lives

Answers the four questions in [2026-10-24-opencode-support.md](2026-10-24-opencode-support.md), from OpenCode **2.1.4** on Ben's machine. Sources: its SQLite store, read only; its local server's OpenAPI document; and a live session driven through `tmux`, with a subagent. Re-run the store survey with `scripts/opencode-shape.sh`, which prints field paths and counts and no content.

**The plan's shape needs one change:** OpenCode has no transcript files. Sessions live in a SQLite database, and the context window of a plugin-supplied model is known only to the running OpenCode server. How `contextfill` should read them is an open decision ([below](#decision-how-to-read-it)).

## 1. Where a session's messages live

- **The store is `~/.local/share/opencode/opencode.db`**, a SQLite database in WAL mode, which OpenCode keeps open. The binary also reads `OPENCODE_DB` and honors `XDG_DATA_HOME`. The older JSON files under `storage/` are gone, apart from `migration`, `project` and `session_diff`.
- **Two generations of tables.** `session`, `message` and `part` stopped updating on 2026-08-14. The current ones are `session_v2` (one row per session) and `session_message` (`session_id`, `type`, `seq`, `data` JSON). Reading the legacy tables gives stale answers that look valid. That was nearly this survey's first mistake.
- **A `session_message` of `type = 'assistant'` is one model step.** A turn with tool calls is several steps, each with its own `data.tokens`: `input`, `output`, `reasoning`, `cache.read`, `cache.write`. The model is `data.model.{id, providerID, variant}`. Other types seen: `user`, `system`, `synthetic`, `shell`, `idle`.
- **`tokens.input` excludes the cache.** In the legacy table, the stored `total` equals `input + output + reasoning + cache.read + cache.write` on almost every finished step, across every provider used. So the input side of a step is `input + cache.read + cache.write`, the same formula as Claude Code's `usage`.
- **OpenCode's own footer shows the step's total, not the input side.** It adds that step's output, so its figure runs a little above ours (`11.6K (1%)` against an input side of about 11.5k on the live session). Expect that small gap when comparing the two.
- **Skip steps with no reading.** An errored, aborted or still-running step has no `tokens` or a zero input side, and `finish` is `error`, `unknown` or missing. The latest reading is the newest assistant step whose input side is above zero.
- **`session_v2.tokens_*` are cumulative** across the session's steps. They are not the fill.

## 2. How a shell inside OpenCode names its session

- **`OPENCODE_SESSION_ID` is set in every shell OpenCode starts** (format `ses_…`), along with `OPENCODE=1`, `AGENT=1` and `OPENCODE_TERMINAL`. No fallback is needed.
- **A subagent's shell gets the subagent's own session ID**, not its parent's.
- **Detection must prefer `OPENCODE`.** An OpenCode started from a Claude Code shell inherits `CLAUDE_CODE_SESSION_ID` and the other `CLAUDE_CODE_*` variables. So when both are set, the innermost harness is OpenCode. `CLAUDECODE=1` is not a reliable marker on its own.

## 3. How it names the model and its window

- **No suffix convention.** The model is `providerID` plus `id`, and the window is that model's `limit.context`.
- **Three sources for `limit.context`, and only one is complete:**
  - the models.dev catalog cached at `~/.cache/opencode/models.json` (`<provider>.models.<id>.limit.context`) lacks plugin providers and models that have dropped out of the catalog;
  - `opencode.json` can declare `provider.<id>.models.<id>.limit`, but the configured custom provider declares none;
  - **the running server's `model.list` (`GET /api/model`)** returns every model, plugin providers included, with `limit.context`. On the live session it gave 1,048,576 for the default model, matching the footer's 1%.
- A model none of these know has no window. Report its tokens without a percentage, never a guessed one.

## 4. Subagents and hooks

- **Subagents are child sessions:** `session_v2.parent_id` is the parent, `agent` is the subagent type (`explore`, …), and `title` is its task. A trailing `idle` message marks it finished. Listing agents is `select … from session_v2 where parent_id = ?`.
> **Corrected 2026-10-24, while building the plugin:** the next bullet describes the v1 plugin API, from stale type definitions, and is wrong for 2.1.4. In v2:
> - A plugin is a default export `{ id, setup(ctx) }` (`Plugin.define` from `@opencode/plugin` is the identity, so no import is needed).
> - It hooks `ctx.tool.hook("execute.after", event => …)`. The event carries `tool`, `sessionID`, `agent`, `messageID`, `id`, `input` and `status`, plus `result` (`{output, content, metadata}`) when the status is `completed`.
> - The shell tool is named `shell`, not `bash`, and `input.command` is its command.
> - Replacing `event.result` with a `content` array that has an extra `{type: "text", text}` item reaches the model; verified live.
> - Plugins load from `.opencode/plugins/` or `~/.config/opencode/plugins/` (plural). The long-lived background server keeps a project's plugins loaded until `opencode reload`.
> - Sources: [opencode.ai/v2/docs/build/plugins](https://opencode.ai/v2/docs/build/plugins/) and the `@opencode/plugin` 2.1.4 type definitions on npm.

- **No shell-command hooks.** Extension is through JavaScript plugins (`@opencode-ai/plugin`). The nearest equivalent of our PostToolUse hook is `"tool.execute.after"`, which receives `{tool, sessionID, callID, args}` and a mutable `{title, output, metadata}`. A `contextfill` hook for OpenCode would be a small plugin that shells out to `contextfill` after a committing bash call and appends the reading to `output`. Not verified: whether an appended `output` reaches the model, and whether the installed type definitions (plugin package 1.3.15, older than the 2.1.4 binary) are current. `"shell.env"` also exists, to set variables in shells.

## Decision: how to read it

> **Ruled 2026-10-24 (Ben): option 1, over HTTP.** Reading the plaintext password from `service.json` is fine; OpenCode generates it and its own clients (TUI, `opencode api`, `opencode pair`) authenticate the same way. Fetch only what's needed: `GET /api/session/{id}/message?type=assistant&order=desc&limit=5` for the newest steps, and `GET /api/model` for the window.

> **Found while building, 2026-10-24:** a list's `cursor` carries its own order, and the server answers `400 InvalidCursorError` ("Cursor cannot be combined with order") to a request that sends both. So the next page sends `type`, `limit` and `cursor`, without `order`. This only shows when the newest steps have no reading, for example while a step is still running, which is exactly when `contextfill` runs from inside OpenCode. The session list's cursor carries its `parentID` and accepts it alongside.

`transcript` reads plain files with the standard library. OpenCode needs one of these:

1. **Ask the running server** through its HTTP API: `session.message.list` (`GET /api/session/{id}/message`, newest first, same `tokens` shape) and `model.list` for the window. It's the only source with every window, needs no new dependency, and the API is a published OpenAPI surface rather than a private schema. Costs: it works only while OpenCode's server runs (always true inside an OpenCode shell). **Finding the server is solved:** the background service writes `~/.local/state/opencode/service.json` with `url` (a loopback address with a varying port), `pid`, `version` and `password`. The API answers 401 without credentials and 200 with HTTP Basic auth as user `opencode` and that password, which `~/.config/opencode/service.json` also holds. TUI sessions are served by that background service, so the live session's messages came back through it. One alternative is shelling out to `opencode api`, which costs a process spawn and ties `contextfill` to that CLI's flags. Its output also cut off at exactly 256 KiB when piped here, so a large session would need a file.
2. **Read the SQLite store directly**, with a pure-Go driver (`modernc.org/sqlite`, a large dependency) or cgo. Works with no server, but the schema is private: it changed generation two months ago, and plugin windows still need the server.
3. **Both:** the store for tokens, and the server or catalog for the window. That combines both sets of costs.

**Recommendation: 1, over HTTP, with the standard library.** It's the only option that knows every window, it adds no dependency, and its contract is the most stable. It means `contextfill` reads a password file and talks to a local port. That is a new kind of access for a tool that so far only read transcripts, so it's worth saying in the README. Tests use `httptest` with synthetic responses.
