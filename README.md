# git-anticipate

## NAME

git-anticipate — preemptively resolve merge conflicts before they occur

## SYNOPSIS

```
git anticipate <branch>
git anticipate --continue [--no-verify]
git anticipate --abort
git anticipate --status
```

## DESCRIPTION

`git-anticipate` performs a trial merge of the current branch with the specified target branch. Conflicts are surfaced for manual resolution in your working directory. Once resolved, the changes are applied as a regular commit on your current branch. Subsequent merges with the target branch will apply cleanly, avoiding repeated conflict resolution.

## INSTALLATION

```bash
go install github.com/amitpdev/git-anticipate@latest
```

Or build from source:

```bash
git clone https://github.com/amitpdev/git-anticipate.git
cd git-anticipate
go build -o git-anticipate
sudo mv git-anticipate /usr/local/bin/
```

## OPTIONS

| Option | Description |
|--------|-------------|
| `<branch>` | Target branch to anticipate conflicts with |
| `--continue` | Apply resolved conflicts as a commit |
| `--abort` | Abort and restore original state |
| `--status` | Show current anticipate status |
| `--no-verify` | Skip pre-commit hooks when committing |
| `-h` | Show help |
| `-v, --version` | Show version |

## EXAMPLE

```bash
$ git anticipate main
⚠️  Conflicts detected!
Conflicting files (2):
    ❌ src/api.ts
    ❌ src/utils.ts

$ vim src/api.ts src/utils.ts
$ git add src/api.ts src/utils.ts

$ git anticipate --continue
✨ Success! Resolution committed to feat/my-feature
```

## HOW IT WORKS

`git-anticipate` operates in-place on your working directory, similar to `git rebase`:

```
1. git anticipate <branch>
   ├── Verify clean working tree
   ├── Perform trial merge with <branch>
   ├── If no conflicts → abort merge, exit success
   └── If conflicts → save state, leave markers in files

2. User resolves conflicts manually
   └── Edit files, then: git add <resolved-files>

3. git anticipate --continue
   ├── Capture resolved file contents
   ├── Abort the trial merge, reset to original HEAD
   ├── Write resolved contents back
   └── Commit as "Preemptive conflict resolution vs <branch>"
```

The resulting commit contains your conflict resolutions. When you later merge with `<branch>`, Git sees no conflicts—your branch already incorporates the necessary changes.

If no conflicts are found, nothing is committed—your branch is already compatible.

## EXIT CODES

| Code | Meaning |
|------|---------|
| 0 | Success (no conflicts, or resolution committed) |
| 1 | Conflicts detected (expected, resolve and continue) |
| 2 | Error (invalid arguments, not a git repo, etc.) |

## GIT-ANTICIPATE VS GIT-RERERE

Both tools help with merge conflicts, but serve different purposes:

| | `git rerere` | `git-anticipate` |
|---|---|---|
| **When** | At merge/rebase time | Anytime before merge |
| **Approach** | Reactive—records resolutions after conflicts | Proactive—resolve while code context is fresh |
| **Output** | Local cache (not in history) | Commit on your branch |
| **Workflow** | Resolve once, auto-replay on same conflict | Resolve once now, actual merge is clean |

**`git rerere`** is useful when rebasing repeatedly or redoing merges—it remembers how you resolved a conflict and applies it automatically next time.

**`git-anticipate`** lets you resolve conflicts *now*, while you still remember why you made certain changes, rather than waiting until merge day when the code is stale in your mind. The resolution becomes a permanent commit, so the actual merge has no conflicts at all.

Both tools can complement each other.

## SEE ALSO

git-merge(1), git-rebase(1), git-rerere(1)

## AUTHOR

[Amit Palomo](https://github.com/amitpdev)

## LICENSE

MIT
