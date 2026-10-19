package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// HookPath is the pre-commit hook trilha hooks writes. It lives in the
// repository, not in .git/hooks, so every clone has the same one;
// core.hooksPath points git at it.
const HookPath = ".githooks/pre-commit"

// hookStamp opens the script: the shebang has to be its first line, so the
// stamp is the comment under it.
var hookStamp = stampFmt{"#!/bin/sh\n# trilha hooks ", ""}

// Hook results beyond the file ones: what happened to core.hooksPath.
const (
	HooksEnabled   = "enabled"   // core.hooksPath now points at .githooks
	HooksAlready   = "already"   // it already did
	HooksNoRepo    = "no-repo"   // not a git repository yet: run trilha hooks after git init
	HooksElsewhere = "elsewhere" // core.hooksPath points somewhere else; left alone
)

// WriteHooks writes the pre-commit hook under the rule of the other files the
// framework owns: created, kept, refreshed when untouched, refused when
// edited unless force.
func WriteHooks(dir string, force bool) (UIResult, error) {
	body, err := agentsFS.ReadFile("agents/pre-commit.sh")
	if err != nil {
		return UIResult{}, err
	}
	dst := filepath.Join(dir, filepath.FromSlash(HookPath))
	res, err := writeStamped(dst, hookStamp.apply(body), hookStamp, force)
	if err == nil {
		err = os.Chmod(dst, 0o755)
	}
	return UIResult{HookPath, res}, err
}

// EnableHooks points core.hooksPath at .githooks in the repository at dir,
// unless it already points somewhere else — a project with hooks of its own
// keeps them, and is told so.
func EnableHooks(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return HooksNoRepo
	}
	cur, _ := exec.Command("git", "-C", dir, "config", "--get", "core.hooksPath").Output()
	switch strings.TrimSpace(string(cur)) {
	case ".githooks":
		return HooksAlready
	case "":
		if err := exec.Command("git", "-C", dir, "config", "core.hooksPath", ".githooks").Run(); err != nil {
			return HooksNoRepo
		}
		return HooksEnabled
	}
	return HooksElsewhere
}
