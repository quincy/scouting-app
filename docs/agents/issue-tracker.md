# Issue Tracker: GitHub Issues

This project uses [GitHub Issues](https://github.com/quincy/scouting-app/issues) for issue tracking.

## Workflow

- **Finding work:** Browse open issues on GitHub.
- **Creating issues:** Use `gh api repos/quincy/scouting-app/issues -X POST` with title, body, and labels.
- **Closing issues:** Use `gh api repos/quincy/scouting-app/issues/{number} -X PATCH -f state=closed`.
- **Viewing details:** Visit the issue URL on GitHub.

## Rules

- Use GitHub Issues for ALL task tracking — do NOT use beads, TodoWrite, or markdown TODO lists
- Use `gh api` for issue management
- Do not create or modify `.beads/` files

## Session Completion

**When ending a work session**, you MUST complete ALL steps below.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Clean up** - Clear stashes, prune remote branches
5. **Verify** - All changes committed
6. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- You are not responsible for pushing work to any remote git repo. Don't try it.
- You are not responsible for pulling from any remote git repo. Don't try it.
- When implementation is finished and committed, prompt the user to review and 
  push. Be helpful and provide the commands to run.
