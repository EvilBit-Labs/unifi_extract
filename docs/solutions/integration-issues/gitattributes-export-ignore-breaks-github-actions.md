---
title: ".gitattributes export-ignore on .github/ breaks every GitHub Actions run"
date: 2026-07-19
category: integration-issues
module: ci
problem_type: integration_issue
component: development_workflow
symptoms:
  - "Every workflow run concludes startup_failure with no jobs, including GitHub's dynamic workflows (Dependabot Updates, CodeQL default setup, Dependency Graph, Copilot)"
  - "Run API shows empty workflow name and path BuildFailed with display_title (Unknown event)"
  - "gh run view prints 'This run likely failed because of a workflow file issue' even though actionlint passes locally"
  - "No github-actions check suite is created on commits; only third-party app checks (DCO, CodeRabbit) appear"
root_cause: config_error
resolution_type: config_change
severity: high
tags: [github-actions, gitattributes, export-ignore, startup-failure, dependabot, codeql]
---

# .gitattributes export-ignore on .github/ breaks every GitHub Actions run

## Problem

Every GitHub Actions run in the repo — the repo's own workflows and GitHub's
dynamic ones (Dependabot version updates, CodeQL default setup, Dependency
Graph, Copilot) — failed instantly with `startup_failure` and zero jobs,
making CI appear completely dead on a freshly created repository.

## Symptoms

- `gh run list` shows only `startup_failure` conclusions; for `pull_request`
  events the run has an empty `workflowName` and `display_title: "(Unknown event)"`.
- `gh api .../actions/runs/<id>` returns `"path": "BuildFailed"` and no jobs.
- `gh run view <id>` says "This run likely failed because of a workflow file
  issue" — while `actionlint` and pre-commit YAML checks pass locally.
- No `github-actions` check suite appears on commits at all; only app-based
  checks (DCO, CodeRabbit, etc.) register.

## What Didn't Work

- **Fixing `dependabot.yml`** — removing `docker`/`devcontainers` ecosystems
  that had no manifests in the repo. Worth doing on its own, but the
  startup failures continued because Dependabot never got far enough to read
  the config.
- **Patching CodeQL default setup languages** — the default setup reported
  `languages: []`, which looked like the cause of its startup failures.
  Repo-level `PATCH /repos/{o}/{r}/code-scanning/default-setup` was rejected
  ("controlled by organization administrators"), and it was a symptom anyway:
  language detection and workflow resolution were both starved by the same
  root cause.

## Solution

Remove the `.github/ export-ignore` line from `.gitattributes`:

```diff
 # Export ignore (excluded from archives)
-.github/ export-ignore
 .claude/ export-ignore
```

Shipped in PR #1 (`EvilBit-Labs/unifi_extract`). The line had come in with a
boilerplate `.gitattributes` copied from a template. The fix commit also left
a warning comment in `.gitattributes` so the line does not get re-added.

## Why This Works

`export-ignore` excludes paths from `git archive` output. GitHub resolves
repository content for Actions workflow parsing — and for Dependabot and
CodeQL default setup — through that archive codepath. With `.github/`
export-ignored, every workflow file was invisible to the workflow parser, so
run creation failed at graph-build time (`path: "BuildFailed"`) before any
job, runner, or log existed. That is also why GitHub's *dynamic* workflows
failed: they run inside the same repo context that could no longer see
`.github/`.

Verification: immediately after the fix merged, the next Dependabot run
entered `queued` state instead of instantly concluding `startup_failure`
(full green runs were delayed by an unrelated GitHub runner outage at the
time of writing).

## Prevention

- Never add `.github/ export-ignore` to `.gitattributes`. If release
  tarballs must exclude CI config, do it in the packaging step (e.g.,
  GoReleaser archive `files:` allowlists) instead of `export-ignore`.
- Audit template-derived `.gitattributes` files before the first push —
  the offending line shipped inside generic boilerplate.
- Diagnostic shortcut for `startup_failure` with empty workflow name across
  *all* workflows (including dynamic ones): suspect repo-level content
  resolution (`.gitattributes`) or org-level Actions policy first, not the
  workflow YAML — local `actionlint` passing while GitHub reports a
  "workflow file issue" is the tell.

## Related Issues

- PR #1 — plumbing PR whose CI runs surfaced the failure; carries the fix.
- PR #2 — first PR whose checks ran after the fix (validation).
