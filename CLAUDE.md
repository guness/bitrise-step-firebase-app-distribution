# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

A Bitrise Step that wraps the Firebase CLI's `appdistribution:distribute` command to upload Android/iOS binaries to Firebase App Distribution. It is not a compiled program — it is a single bash script (`step.sh`) plus a step contract (`step.yml`) consumed by the Bitrise runner.

## Common commands

Requires the `bitrise` CLI locally. Before running any workflow, populate `.bitrise.secrets.yml` with `FIREBASE_APP_ID_ANDROID`, `FIREBASE_APP_ID_IOS`, and `FIREBASE_CI_TOKEN` (file is git-ignored). Bitrise merges it into env at run time.

- `bitrise run test` — full CI: audit + build & deploy Android + build & deploy iOS
- `bitrise run test-android` / `bitrise run test-ios` — one platform only
- `bitrise run build-android` / `bitrise run build-ios` — build a sample app into `./_tmp` without deploying (the `_tmp` dir is recreated each run)
- `bitrise run deploy-android` / `bitrise run deploy-ios` — run just this step against an already-built artifact
- `bitrise run audit-this-step` — `stepman audit` on `step.yml` (run this after editing `step.yml`)
- `bitrise run share-this-step` — publishes a new version to a StepLib fork (see `BITRISE_STEP_VERSION` in `bitrise.yml`; bump before sharing)

To invoke `step.sh` outside Bitrise, export the input variables named in `step.yml` (`app`, `app_path`, `firebase_token` or `service_credentials_file`, etc.) and run `./step.sh` directly — the script reads them from the environment.

## Architecture

Three files form the contract with Bitrise; they must stay in sync:

1. **`step.yml`** — public contract. Declares `inputs` (become env vars in `step.sh`) and `outputs` (env vars the step exports back). Also declares `toolkit: bash` with `entry_file: step.sh`. Any new input or output must be added here or the runner rejects it.
2. **`step.sh`** — implementation. Sourced as a bash script; Bitrise has already exported the declared inputs as lowercase env vars. It validates inputs, installs/upgrades the Firebase CLI via `curl -sL firebase.tools | bash` (skipped if already installed unless `upgrade_firebase_tools=true`), constructs a `firebase appdistribution:distribute` command string, `eval`s it, then parses stdout to extract outputs.
3. **`bitrise.yml`** — local test harness. Defines workflows that build sample Android/iOS apps into `./_tmp` and then invoke the step via `path::./`.

### Key behaviors in `step.sh` that are easy to break

- **Release ID output compatibility**: export both `FIREBASE_BITRISE_APP_DISTRIBUTION_RELEASE_ID` (the documented name) and `FIREBASE_APP_DISTRIBUTION_RELEASE_ID` (the legacy alias) with the same value. Both are declared in `step.yml` and asserted by the parsing workflows.
- **Authentication is one-of**: `firebase_token` XOR `service_credentials_file`. If `service_credentials_file` starts with `http`, it's downloaded via `curl` to `./credentials.json` and `GOOGLE_APPLICATION_CREDENTIALS` is set. Both-set is a warning, not an error.
- **`app_path` has a pipe-path workaround**: Bitrise sometimes chains `$BITRISE_IPA_PATH|$BITRISE_APK_PATH|$BITRISE_AAB_PATH`, leaving leading/trailing `|` when some vars are empty. The `case` block at lines ~145-165 strips these; an internal `|` (multiple real paths) is a hard fail. Don't remove this logic.
- **Outputs come from the Firebase `--debug` JSON body, with a labeled-line fallback**: `step.sh` always passes `--debug` to `firebase appdistribution:distribute` internally (regardless of the `is_debug` input). It parses the `UploadReleaseResponse` JSON from the `[apiv2][body]` debug line via `jq` to populate `FIREBASE_BITRISE_CONSOLE_URL`, `FIREBASE_BITRISE_APP_DISTRIBUTION_URL`, `FIREBASE_BITRISE_BINARY_DOWNLOAD_URL`, and `FIREBASE_APP_DISTRIBUTION_RELEASE_ID`. If the JSON is missing (future Firebase CLI could change), it falls back to grepping the `✔ View/Share/Download …` labeled success lines. The `is_debug` input now controls *display only*: `false` (default) filters the Bitrise log to `✔`/`i`/`⚠`/`!`/`Error:` prefixed lines; `true` echoes the full raw debug output. **`jq` is a hard dependency** — Bitrise stacks ship it, but the step hard-fails with a clear message if it's missing. Hermetic tests for both paths live in `bitrise run test-parse-primary` / `test-parse-fallback` using stubs in `_fixtures/`.
- **Release notes escaping**: `escape()` only escapes `"`. The distribute command is assembled into a string and run via `eval`, so any input flowing into `submit_cmd` must be safe inside double quotes.
- **Release notes truncation**: if `release_notes_length > 0`, notes are trimmed to that length with `...` appended (Firebase rejects overlong notes).

## Contributing workflow

Changes to `step.sh` or `step.yml` should be validated with `bitrise run test` (requires working Firebase credentials in `.bitrise.secrets.yml`). Bump `BITRISE_STEP_VERSION` in `bitrise.yml` when shipping a new version.
