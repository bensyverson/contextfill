package hook_test

import (
	"testing"

	"github.com/bensyverson/contextfill/hook"
)

func TestCommitsTellsAGitCommitFromAMention(t *testing.T) {
	commits := []string{
		"git commit -F msg.txt",
		"cd repo && git commit -m done",
		"git add -A; git commit -q -F m 2>&1 | tail -3",
		"git add .\ngit commit -F m",
		"(cd repo && git commit -F m)",
		"out=$(git commit -F m)",
		"x=`git commit -F m`",
		"FOO=1 git commit -F m",
		"env GIT_EDITOR=true git commit",
		"/usr/bin/git commit -F m",
		"git -C /repo commit -F m -- a b",
		`git -C "/path with spaces" commit -F m`,
		"git -c core.hooksPath=x commit -F m",
		"git --git-dir=/r/.git --work-tree=/r commit -F m",
		"git --no-pager commit --amend",
		"make test || git commit -F m",
		"PENUMBRA_DEFER_TESTS=1 git commit -F m -- a",
	}
	// Commands that only mention a commit, or run a different git verb.
	others := []string{
		"echo 'git commit'",
		`echo "run git commit; then push"`,
		"grep -n 'git commit' notes.md",
		"git commit-tree abc -p HEAD -m wip",
		"git log --grep commit",
		"git status",
		"git show HEAD:commit.txt",
		"gitcommit",
		"",
	}
	for _, command := range commits {
		if !hook.Commits(command) {
			t.Errorf("Commits(%q) = false, want true", command)
		}
	}
	for _, command := range others {
		if hook.Commits(command) {
			t.Errorf("Commits(%q) = true, want false", command)
		}
	}
}
