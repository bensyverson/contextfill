// contextfill's OpenCode plugin: after a shell command that commits, it tells
// the model how full its session's context window is. Every decision (is
// this a commit, which limit applies, what to say) belongs to
// `contextfill hook`; this file only hands it each finished tool call and
// appends whatever it prints. Install by copying or linking it into
// ~/.config/opencode/plugins/.

import { spawn } from "node:child_process"

// The command to run. Use a full path if `contextfill` isn't on the PATH
// OpenCode's server runs with. --max is a subagent's limit, --main-max the
// main session's.
const command = process.env.CONTEXTFILL ?? "contextfill"
const args = ["hook", "--harness", "opencode", "--max", "40", "--main-max", "25"]
const timeoutMs = 10_000

// ask pipes the payload to `contextfill hook` and resolves to what it
// printed, or "" if it failed: a broken hook must never break a tool call.
function ask(payload) {
  return new Promise((resolve) => {
    const child = spawn(command, args, { stdio: ["pipe", "pipe", "ignore"], timeout: timeoutMs })
    let out = ""
    child.stdout.on("data", (chunk) => (out += chunk))
    child.on("error", () => resolve(""))
    child.on("close", (code) => resolve(code === 0 ? out.trim() : ""))
    child.stdin.on("error", () => {})
    child.stdin.end(JSON.stringify(payload))
  })
}

export default {
  id: "contextfill",
  setup(ctx) {
    ctx.tool.hook("execute.after", async (event) => {
      // Only a completed call has a result to append to.
      if (event.status !== "completed") return
      const text = await ask({ tool: event.tool, sessionID: event.sessionID, input: event.input })
      if (!text) return
      event.result = { ...event.result, content: [...(event.result.content ?? []), { type: "text", text }] }
    })
  },
}
