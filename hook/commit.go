package hook

import (
	"path/filepath"
	"slices"
	"strings"
)

// Commits reports whether a shell command line runs `git commit` anywhere in
// it: after `&&`, `;`, `|` or a newline, inside a subshell or a command
// substitution, behind variable assignments or `env`, and past git's global
// options. A commit that is only quoted, or a different verb such as
// `commit-tree`, doesn't count.
func Commits(command string) bool {
	return slices.ContainsFunc(simpleCommands(command), isGitCommit)
}

// simpleCommands splits a command line into its simple commands' words. It
// knows quotes, backslashes and the operators that separate commands; it
// does not expand anything.
func simpleCommands(line string) [][]string {
	var (
		commands [][]string
		words    []string
		word     strings.Builder
		inWord   bool
	)
	endWord := func() {
		if inWord {
			words = append(words, word.String())
			word.Reset()
			inWord = false
		}
	}
	endCommand := func() {
		endWord()
		if len(words) > 0 {
			commands = append(commands, words)
			words = nil
		}
	}
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '\\' && i+1 < len(line):
			i++
			word.WriteByte(line[i])
			inWord = true
		case c == '\'':
			end := strings.IndexByte(line[i+1:], '\'')
			if end < 0 {
				end = len(line) - i - 1
			}
			word.WriteString(line[i+1 : i+1+end])
			i += end + 1
			inWord = true
		case c == '"':
			i++
			for ; i < len(line) && line[i] != '"'; i++ {
				if line[i] == '\\' && i+1 < len(line) {
					i++
				}
				word.WriteByte(line[i])
			}
			inWord = true
		case c == ' ' || c == '\t':
			endWord()
		case strings.IndexByte(";&|\n()`", c) >= 0:
			endCommand()
		case c == '$' && i+1 < len(line) && line[i+1] == '(':
			endCommand()
			i++
		default:
			word.WriteByte(c)
			inWord = true
		}
	}
	endCommand()
	return commands
}

// gitOptionsWithValue are git's global options that take the next word as
// their value when written without "=".
var gitOptionsWithValue = map[string]bool{
	"-C": true, "-c": true, "--git-dir": true, "--work-tree": true,
	"--namespace": true, "--super-prefix": true, "--config-env": true,
}

// isGitCommit reports whether one simple command is `git … commit`.
func isGitCommit(words []string) bool {
	i := 0
	for i < len(words) && isAssignment(words[i]) {
		i++
	}
	if i < len(words) && words[i] == "env" {
		i++
		for i < len(words) && (isAssignment(words[i]) || strings.HasPrefix(words[i], "-")) {
			i++
		}
	}
	if i >= len(words) || filepath.Base(words[i]) != "git" {
		return false
	}
	for i++; i < len(words); i++ {
		word := words[i]
		if !strings.HasPrefix(word, "-") {
			return word == "commit"
		}
		if gitOptionsWithValue[word] {
			i++
		}
	}
	return false
}

// isAssignment reports whether a word is a NAME=value prefix assignment.
func isAssignment(word string) bool {
	name, _, ok := strings.Cut(word, "=")
	if !ok || name == "" {
		return false
	}
	for j, r := range name {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || j > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
