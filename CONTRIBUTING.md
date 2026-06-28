<!-- harness:begin -->
# Contributing

> Welcome! This guide covers how to contribute to this repo, from a one-line typo fix to a multi-day feature.
>
> **You don't need to be a developer to contribute.** If you can describe what you want in plain language, an AI assistant (Claude Code, Codex, or Cursor) walks you through every step.

---

## The 30-second decision tree

| If you are... | Go to |
|---|---|
| Anyone wanting to change something — including non-technical contributors | **[Path A — AI-driven (the default)](#path-a--ai-driven-the-default)** |
| A developer running code/tests locally without an AI tool | **[Path B — manual developer setup](#path-b--manual-developer-setup)** |
| Just looking up "how do I do X?" / "what's the rule about Y?" | **[Reference](#reference)** |

---

## Path A — AI-driven (the default)

This is how 95% of contributions should happen. You don't run any commands yourself — the AI assistant runs them on your behalf and tells you when to confirm.

### What you need

1. **The repo cloned to your machine** — `git clone <repo-url>` once.
2. **One AI tool installed**, any of:
   - [Claude Code](https://www.anthropic.com/claude-code) (recommended)
   - [Codex CLI](https://github.com/openai/codex)
   - [Cursor](https://www.cursor.com/) (an IDE)

That's it. The AI tool can install whatever else it needs as it goes.

### How to make a change

1. **Open the repo** in your AI tool of choice.
2. **Type what you want, in plain language.** Examples:
   > "I want to add a *priority* field to entries."
   > "Fix the wrong date format in the dashboard."
   > "Make the daily report skip weekends."
3. **Follow the assistant's instructions.** It will:
   - Ask 1–3 clarifying questions (you answer in plain language too)
   - Write a *task plan* into `.harness/active/<your-task>.md`
   - Walk you through the plan phase-by-phase, telling you exactly when to **commit** and **push** your work
   - Tell you when the task is done and prompt you to open a Pull Request

The assistant will hit you with prompts like:

```
Phase 1 complete. Please run:
  git commit -m "feat(...): ..."
  git push

Then come back and tell me you've pushed. I'll continue with Phase 2.
```

You don't need to know what `feat(...)` means or why we run `git push` — just copy the command into a terminal (or click your IDE's commit/push buttons) when prompted.

### When the task finishes

The assistant will tell you to open a Pull Request from `feature/<task-id>` to `main`. The PR template auto-fills — just check the boxes and submit.

### Troubleshooting Path A

| Problem | Fix |
|---|---|
| AI assistant doesn't seem to follow the protocol — just starts writing code | Open a terminal in the repo, run `harness start`, copy the printed prompt, paste it as your first message. This forces protocol activation. |
| Tests fail and you don't know why | Tell the assistant: "the tests are failing, please debug." It can read the error and fix it for you. |
| Assistant gets stuck or confused | Type "let's start over" and the assistant will run `harness end` to reset state, then begin again. |
| You want to pause for the day | Just close the session. State is saved in `.harness/active/<task-id>.md`. Tomorrow, run `harness implement <task-id>` (or just say "resume task <task-id>" to your AI tool). |

---

## Reference

### Branch model

```
feature/<task-id> ──(free merge, no template)──→ 
feature/<task-id> ──(PR + template required)───→ main  ← protected
```

- **`feature/* → main`**: open a PR — [.github/PULL_REQUEST_TEMPLATE.md](.github/PULL_REQUEST_TEMPLATE.md) loads. Self-merge is allowed; the discipline lives in honestly filling the protocol-status field.

### Forbidden actions

| Don't | Why |
|---|---|
| `git commit --no-verify` | Bypasses the format/test verification at commit points. The protocol depends on this. |
| `git push --force` to `main` | Destroys history that other contributors may have synced. |
| Skipping `` / `` at a `> ✋ Commit point` | The brief promises both pass at every commit. Don't lie to future you. |
| Editing code under  without an active task brief | Unless explicitly marked **out-of-protocol** in the PR, every code change must trace back to a task brief. |

### Useful one-liners

```bash
harness status                     # what task is in flight?
harness implement <task-id>        # resume a paused task (also switches to its branch)
harness end                        # archive a finished task
harness remember "<text>"          # save a long-term decision/convention to MEMORY.md
harness curate-memory              # quarterly MEMORY.md cleanup
harness help                       # full command list
```

### Where to look

- **Project map** ("which files do I edit to change X?"): [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) *(maintained by this repo)*
- **Protocol spec** (the rules every AI assistant follows): [AGENTS.md](AGENTS.md)
- **Step-by-step coordinator/implementer flow**: [.harness/workflow.md](.harness/workflow.md)
- **Cross-task knowledge** (conventions, decisions, pitfalls): [.harness/MEMORY.md](.harness/MEMORY.md)
- **Triage rules** (small change vs full task): [.harness/triage.md](.harness/triage.md)

### Getting unstuck

- `harness status` shows current state.
- See [Path A troubleshooting](#troubleshooting-path-a) for AI-driven flow issues.
- Project-specific gotchas, if any, are documented under [Project-specific pitfalls](#project-specific-pitfalls) below.
<!-- harness:end -->

<!--
============================================================================
EVERYTHING BELOW IS YOUR REPO'S TERRITORY.
The harness plugin will never overwrite content outside the harness:* fences.
Fill in the TODO blocks below with project-specific details, then delete the
"TODO (project maintainers)" callouts to mark each section done.
============================================================================
-->

## Path B — manual developer setup

### One-time environment setup

```bash
# Install Go 1.26+ (https://go.dev/doc/install)
go mod download
# No local services needed — TinyPulse uses embedded SQLite
# Optional: copy example config for local experimentation
cp example.config.yml config.yml
# Smoke check
make build
make test
```

### Day-to-day commands

```bash
make run          # run locally
make test         # run tests
make build        # build binary
gofmt -w .        # format code
go vet ./...      # static analysis
```

### When iterating manually, you still follow the protocol

Even without an AI assistant, **all changes still go through the harness protocol**:

```bash
harness start              # creates a task brief in .harness/active/
# ... edit code, run tests, commit at every "✋ Commit point" in the brief ...
harness end                # archive the brief when the task is complete
```

The brief is your checklist. Open it, work through the phases, tick the boxes, commit at the marked points. See [.harness/workflow.md](.harness/workflow.md) for the full protocol spec.

---

## Project-specific code style

This repo uses vanilla Go tooling:

- Formatting: `gofmt -w .`
- Static analysis: `go vet ./...`

No custom linter configuration is required.

---

## Project-specific pitfalls

- `config.yml` is gitignored. Do not commit real notifier tokens or passwords; use `example.config.yml` with `${ENV_VAR}` placeholders for shared templates.
- TinyPulse uses pure-Go SQLite (`modernc.org/sqlite`), so builds are `CGO_ENABLED=0` and no Postgres/MySQL service is required.
- The `Dockerfile` defaults to `GOARCH=arm64`, but `make build` produces a binary for the host architecture.
