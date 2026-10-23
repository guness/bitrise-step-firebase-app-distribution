# Repository guide

## What this repo is

A Go Bitrise Step wrapping `firebase appdistribution:distribute` for Android and iOS.
The Firebase CLI continues to perform uploads, release-note updates, and tester/group
distribution. The public input and output contract is in `step.yml`.

## Commands

- `go test ./...` — unit and subprocess regression tests, no credentials or Firebase upload.
- `go test -race ./...` and `go vet ./...` — race detection and static checks.
- `bitrise run test-hermetic` — Step audit, Go tests, and both existing parser workflows
  through the actual Go toolkit. Does not contact Firebase.
- `bitrise run test-parse-primary` / `test-parse-fallback` — individual CLI stub workflows.
- `stepman audit --step-yml ./step.yml` — required after editing the Step contract.
- `bitrise run test-android` / `test-ios` — build and distribute real sample applications;
  requires credentials and sends the configured Firebase distribution notifications.
- `bitrise run test` — hermetic checks followed by both real sample uploads.
- `bitrise run share-this-step` — publish a version to StepLib; bump `BITRISE_STEP_VERSION`
  in `bitrise.yml` only when shipping a release.

## Architecture

- `main.go` handles process exit status and cancellation.
- `internal/step/config.go` reads all 12 public environment inputs, normalizes the
  pipe-delimited artifact path, truncates inline notes, and constructs argument slices.
- `internal/step/runner.go` manages CLI installation/upgrades, private temporary
  credential downloads, CLI execution, diagnostics, cleanup, and envman exports.
- `internal/step/output.go` decodes the final upload operation JSON and uses labeled
  success lines for missing fields. It also controls debug versus ordinary log display.
- `internal/step/redact.go` removes known credentials and download tokens from logs.
- `step.yml` uses the native Go toolkit; its package name must match `go.mod`.
- `step.sh` is a compatibility launcher for direct callers. It builds the Go executable
  and invokes it in the caller's original working directory.
- `_fixtures/` contains the original Firebase output stubs. Tests also use temporary
  executable stubs and local HTTP servers; they never need real credentials.

## Compatibility rules

Keep all input names and defaults unchanged. `TestStepContract` freezes those names,
checks that every input is read, and matches declared outputs to actual exports.

Export all three URLs and both release ID names. `FIREBASE_BITRISE_APP_DISTRIBUTION_RELEASE_ID`
is the documented name; `FIREBASE_APP_DISTRIBUTION_RELEASE_ID` is the existing alias.
Both must have identical values. Missing fields warn and are exported as empty strings.

Exactly one artifact path must remain after removing empty leading/trailing pipe
segments. Keep APK, AAB, and IPA support. Do not interpret any input as shell code.
Quotes, backticks, dollar signs, backslashes, and newlines in ordinary inputs are literal.
`flags` uses shell-style word splitting only, without shell evaluation or expansion.
Bitrise performs environment expansion before the process receives its inputs.

Inline release notes use `--release-notes=<value>` so leading dashes are safe. A positive
length limits Unicode characters including the ellipsis; zero/negative/empty disables
truncation. Missing release-note files warn and are skipped. File contents are not truncated.

An explicit `firebase_token` takes precedence if both auth inputs are supplied.
An explicit service account suppresses inherited `FIREBASE_TOKEN` only in the child
process. Downloaded credentials are mode 0600 and removed on success and failure;
caller-owned files must never be removed.

Always request Firebase `--debug` internally to obtain operation JSON. `is_debug`
controls display only. Print captured diagnostics even when the CLI fails, and preserve
its nonzero exit code. Redact known secrets in both output and errors. Export signed
URLs intact to envman while redacting their tokens from logs.

## Dependencies and release checks

Go 1.22+ is required. The sole nonstandard dependency is the pinned, vendored
`github.com/kballard/go-shellquote`, also used by other Bitrise Go Steps. Run
`go mod tidy` and `go mod vendor` after changing dependencies. Go code does not need jq.
The official Firebase installer still runs under Bash when installation/upgrade is needed.

Use the macOS/Linux GitHub Actions checks and `test-hermetic` for ordinary development.
Real Android/iOS uploads remain a release gate; record whether they were actually run.
