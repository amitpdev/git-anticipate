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

## SEE ALSO

git-merge(1), git-rebase(1)

## AUTHOR

[Amit Palomo](https://github.com/amitpdev)

## LICENSE

MIT
