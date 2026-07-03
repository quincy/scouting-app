# Agent Instructions

# Documentation

Documentation for the project is found in the docs folder. This is considered
the source of truth for what the project is supposed to do. Some features may
not be implemented yet. Care should be taken to implement features with the
overall plan in mind.

### Issue tracker

Issue tracking uses GitHub Issues. See `docs/agents/issue-tracker.md`.

### Triage labels

Standard canonical roles are used. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout with root `CONTEXT.md`. See `docs/agents/domain.md`.

### Test-Driven Development (TDD)

All code modifications must use the `tdd` skill (Test-Driven Development with the red-green-refactor loop) to verify
implementation correctness and ensure core logic is thoroughly tested.

### Non-Interactive Shell Commands

**ALWAYS use non-interactive flags** with file operations to avoid hanging on
confirmation prompts.

Shell commands like `cp`, `mv`, and `rm` may be aliased to include `-i`
(interactive) mode on some systems, causing the agent to hang indefinitely
waiting for y/n input.

**Use these forms instead:**

```bash
# Force overwrite without prompting
cp -f source dest           # NOT: cp source dest
mv -f source dest           # NOT: mv source dest
rm -f file                  # NOT: rm file

# For recursive operations
rm -rf directory            # NOT: rm -r directory
cp -rf source dest          # NOT: cp -r source dest
```

**IMPORTANT**: If you are ever about to run `rm -rf` you must first verify that the path that is being removed is inside
this git repository. You are NEVER allowed to remove files outside the git repository.

**Other commands that may prompt:**

- `scp` - use `-o BatchMode=yes` for non-interactive
- `ssh` - use `-o BatchMode=yes` to fail instead of prompting
- `apt-get` - use `-y` flag
- `brew` - use `HOMEBREW_NO_AUTO_UPDATE=1` env var


