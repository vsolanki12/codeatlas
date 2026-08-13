---
name: check-repos
description: Check freshness of codeatlas and codeatlas-assistant repos, binaries, and graphs
---

# Check Repos Freshness

Run the following bash commands and report results in a summary table.

## 1. Repo sync status

For each repo, check ahead/behind and last commit:

```bash
git -C ~/codeatlas status -sb
git -C ~/codeatlas log -1 --format="%cr  %s"

git -C ~/codeatlas-assistant status -sb
git -C ~/codeatlas-assistant log -1 --format="%cr  %s"
```

## 2. Binary freshness

Compare binary mtime to latest commit timestamp. A binary is stale if its mtime is older than the latest commit.

```bash
# codeatlas
COMMIT_TS=$(git -C ~/codeatlas log -1 --format=%ct)
BINARY_TS=$(stat -f %m ~/codeatlas/atlas 2>/dev/null || echo 0)
echo "atlas: commit=$COMMIT_TS binary=$BINARY_TS stale=$([ "$BINARY_TS" -lt "$COMMIT_TS" ] && echo YES || echo no)"

# codeatlas-assistant
COMMIT_TS=$(git -C ~/codeatlas-assistant log -1 --format=%ct)
BINARY_TS=$(stat -f %m ~/codeatlas-assistant/assistant 2>/dev/null || echo 0)
echo "assistant: commit=$COMMIT_TS binary=$BINARY_TS stale=$([ "$BINARY_TS" -lt "$COMMIT_TS" ] && echo YES || echo no)"
```

## 3. Graph freshness

Find all `*-graph.json` files in `~/codeatlas/` and check each against its source repo:

```bash
for graph in ~/codeatlas/*-graph.json; do
  [ -f "$graph" ] || continue
  GRAPH_COMMIT=$(python3 -c "import json; d=json.load(open('$graph')); print(d.get('commit','unknown')[:7])")
  REPO_PATH=$(python3 -c "import json; d=json.load(open('$graph')); print(d.get('repository','unknown'))")
  REPO_HEAD=$(git -C "$REPO_PATH" rev-parse --short HEAD 2>/dev/null || echo "unknown")
  NAME=$(basename "$graph")
  echo "$NAME: graph=$GRAPH_COMMIT repo_head=$REPO_HEAD match=$([ "$GRAPH_COMMIT" = "$REPO_HEAD" ] && echo YES || echo no)"
done
```

## 4. Output format

Present results as a single table:

```
| Artifact              | Status       | Detail                          |
|-----------------------|--------------|---------------------------------|
| codeatlas repo        | ahead/behind | last commit: <age> <subject>    |
| codeatlas-assistant   | ahead/behind | last commit: <age> <subject>    |
| atlas binary          | OK / STALE   | binary vs commit timestamps     |
| assistant binary      | OK / STALE   | binary vs commit timestamps     |
| <name>-graph.json     | OK / STALE   | graph=<sha> vs HEAD=<sha>       |
```

If anything is STALE, add recommendation lines below the table:
- Stale binary: `cd ~/codeatlas && go build -o atlas ./cmd/atlas/`
- Stale graph: `cd ~/codeatlas && ./atlas scan -repo <path> -output <name>-graph.json`
