# Stops a commit whose staged files are over the size limits of AGENTS.md:
# 5,000 tokens for any file an agent writes, 500 lines for source files.
# Written by `trilha hooks`; `trilha precommit` is the check, and its message
# says how to split what is over.
if command -v trilha >/dev/null 2>&1; then
	exec trilha precommit
fi
# No CLI on the PATH: the version the project's go.mod requires.
exec go run github.com/emersonjoe/trilha/cmd/trilha precommit
