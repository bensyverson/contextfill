// Command contextfill reports how full a Claude Code or OpenCode session's
// context window is.
package main

import "os"

// Exit codes, from the house CLI table.
const (
	exitOK          = 0
	exitOverMax     = 1 // the --max assertion failed
	exitUsage       = 2
	exitTarget      = 4 // the session is missing or unreadable
	exitEnvironment = 5 // no session to read, or no server to ask
)

func main() { os.Exit(run(os.Args[1:], os.Getenv, os.Stdin, os.Stdout, os.Stderr)) }
