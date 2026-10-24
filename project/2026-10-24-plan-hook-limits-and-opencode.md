# 2026-10-24 — Plan: separate hook limits, then OpenCode support

Two roots. The hook-limits change is small and independent, so it goes first. OpenCode support follows the shape and question list in [2026-10-24-opencode-support.md](2026-10-24-opencode-support.md); its investigation decides the details of every later leaf, so each leaf is blocked by the one before.

```yaml
tasks:
  - title: Separate hook limits for the main thread and subagents
    children:
      - title: Add --main-max to the hook
        desc: |
          The shared rules draw two lines: a subagent hands off past 40% of its context, and the integrator stops delegating at 25%. The hook has one `--max`. A subagent's PostToolUse payload carries `agent_id` and the main thread's doesn't, so add `--main-max <percent>` for payloads without an `agent_id`, falling back to `--max`. Red first, in `hook/hook_test.go` and `cmd/contextfill/run_test.go`. Afterwards Ben changes `~/.claude/settings.json` to `contextfill hook --max 40 --main-max 25` (the sandbox can't write it).
        criteria:
          - A payload without agent_id is judged against --main-max when given, else --max.
          - A payload with agent_id is always judged against --max.
          - README and contextfill --help document --main-max.
  - title: OpenCode support
    children:
      - title: Investigate OpenCode's session storage
        ref: oc-investigate
        desc: |
          Answer the four questions in project/2026-10-24-opencode-support.md from real files, not memory: where a session's messages and token counts live, how a shell inside OpenCode names its session, how it names the model and its window, and whether it has subagents and hooks. OpenCode's data is under ~/.local/share/opencode and ~/.config/opencode, which the sandbox blocks; Ben approved reading those paths with the sandbox off, and driving a live session through tmux. Nothing lifted from those files is committed.
        criteria:
          - A dated findings doc under project/ answers all four questions, citing field names from real files.
          - The doc names the session-identification fallback and what it costs, if there is no session variable.
      - title: Put a reader interface over the transcript package
        ref: oc-interface
        blockedBy: [oc-investigate]
        desc: |
          One small interface: find a session, read its latest usage, list its agents. `transcript` stays Claude Code's reader and implements it. No change in behavior.
        criteria:
          - cmd/contextfill depends on the interface, not on transcript directly.
          - Existing tests pass unchanged.
      - title: Add the opencode reader package
        ref: oc-reader
        blockedBy: [oc-interface]
        desc: |
          An `opencode` package beside `transcript`, implementing the interface from the findings doc. Fixtures are synthetic and built in the test.
        criteria:
          - Every storage shape in the findings doc has a synthetic fixture case.
          - A missing field is an error, never a guess.
      - title: Pick the harness in the CLI
        blockedBy: [oc-reader]
        desc: |
          cmd/contextfill picks the reader from the environment (CLAUDECODE=1, or OpenCode's own marker from the findings doc) and takes a `--harness` override.
        criteria:
          - The reader is chosen from the environment, and --harness overrides it.
          - README and contextfill --help cover OpenCode and --harness.
```
