# Go migration plan

## Goal and boundary

Move the Bitrise Step orchestration from `step.sh` to Go while preserving the existing Step inputs, Firebase CLI upload behavior, and four output values. The first Go release should continue to invoke `firebase appdistribution:distribute`. Replacing the CLI with direct Firebase REST calls would also change authentication, upload, distribution, and free-form `flags` behavior, so evaluate that as a separate project after parity is established.

## What exists today

| Area | Current behavior | Go migration requirement |
| --- | --- | --- |
| Bitrise contract | `step.yml` uses `toolkit.bash.entry_file: step.sh`; inputs become environment variables | Change to `toolkit.go.package_name` matching the Go module path; keep input names and defaults |
| Artifact path | `app_path` defaults to three Bitrise paths joined by `|`; `step.sh` strips empty leading/trailing segments and rejects multiple actual paths | Preserve the one-artifact rule; test all empty and multi-path combinations |
| Authentication | `firebase_token` or `service_credentials_file`; a credential URL is downloaded and exposed through `GOOGLE_APPLICATION_CREDENTIALS` | Preserve both modes and their precedence; download credentials to a private temporary file and clean it up |
| Firebase CLI | Reuses an installed CLI unless `upgrade_firebase_tools=true`; otherwise runs the Firebase installer | Keep the install/upgrade input behavior and fail clearly if installation or lookup fails |
| Upload options | Notes, note length, note file, testers, groups, and `flags` feed the CLI invocation | Build an argument slice without shell `eval`; define and test how `flags` is tokenized |
| Output parsing | Always enables Firebase `--debug`; first parses the final operation JSON, then falls back to labeled success lines | Use `encoding/json` for the operation response and retain the labeled fallback; `is_debug` still controls log display |
| Exports | Console URL, tester URL, one-hour binary URL, and release ID are sent through `envman` | Export all four under their established environment variable names |

The release ID mismatch reported in GitHub issue #74 is fixed on `develop`: both `FIREBASE_BITRISE_APP_DISTRIBUTION_RELEASE_ID` (the documented output) and `FIREBASE_APP_DISTRIBUTION_RELEASE_ID` (the existing exported name) are now declared, exported, and asserted by the workflows. The Go migration must preserve both names with the same value.

The current script also prints `firebase_token` and the constructed command. The Go implementation should redact token values and credential locations from ordinary logs and errors. Treat the signed binary URL as sensitive because it grants a temporary download; export it without echoing its query token.

## Reference projects and contracts checked

- [Bitrise Step development guide](https://docs.bitrise.io/en/bitrise-ci/workflows-and-pipelines/developing-your-own-bitrise-step/developing-a-new-step) describes Go Step scaffolding and the `step.yml` contract.
- [Gradle Runner `step.yml`](https://github.com/bitrise-steplib/steps-gradle-runner/blob/master/step.yml) and [Go implementation](https://github.com/bitrise-steplib/steps-gradle-runner/blob/master/main.go) show `toolkit.go.package_name`, typed environment input parsing, command execution, and output export with Bitrise's v2 Go utilities.
- [Android Build `step.yml`](https://github.com/bitrise-steplib/bitrise-step-android-build/blob/master/step.yml) is another published Go Step with a module package name in the toolkit declaration.
- [Firebase CLI distribution guide](https://firebase.google.com/docs/app-distribution/android/distribute-cli) lists the upload arguments and the three returned links, including the one-hour binary URL.
- [Firebase App Distribution REST API](https://firebase.google.com/docs/reference/app-distribution/rest) separates upload, release update, and distribution calls. This is the reference for a possible later CLI removal, not the initial migration.

## Implementation sequence

1. **Lock down compatibility.** Add table-driven tests for artifact-path normalization, authentication selection, release-note truncation, optional arguments, JSON operation parsing, labeled fallback, missing fields, and Firebase failure exit status. Extend the existing CLI stubs to record argument boundaries and environment variables. Resolve the release ID output-name mismatch in `step.yml` with a documented compatibility choice.
2. **Build the Go Step beside Bash.** Add `go.mod`, `main.go`, and small packages for configuration, CLI invocation, parsing, and output export. Read the existing input environment variables; use `exec.CommandContext` with an argument slice and separate stdout/stderr capture. Keep an injectable command runner and envman exporter for tests. Use the standard JSON decoder for the operation response. Avoid changing the Step toolkit until all current hermetic workflows pass against the Go executable.
3. **Handle installation and secrets.** Reuse an installed `firebase`; when installation or upgrade is requested, download the official installer to a temporary file and invoke it with an explicit process environment. Check the installed executable afterward. Keep service-account files private and short lived. Never log `--token` values, raw credentials, or signed download query strings in normal mode. Document that `is_debug=true` can reveal raw Firebase diagnostics.
4. **Switch the Bitrise entrypoint.** Set `toolkit.go.package_name` to the module path, remove `toolkit.bash.entry_file`, and update `bitrise.yml` so `test-parse-primary` and `test-parse-fallback` run the Go Step. Add a test for a Firebase CLI nonzero exit and one for malformed or partial debug JSON. Keep `step.sh` for one comparison cycle, then remove it after real Android and iOS runs match the Go outputs.
5. **Release gate.** Run `go test ./...`, `go vet ./...`, `stepman audit --step-yml ./step.yml`, and the hermetic Bitrise workflows on macOS and Linux where supported. Then run credential-backed Android and iOS uploads with both token and service-account auth before publishing a new Step version. Verify each output and check logs for secrets. Bump `BITRISE_STEP_VERSION` only when the new toolkit is ready to ship.

## Decisions and risks to settle during implementation

- **`flags` input:** the Bash script currently wraps the entire string as one shell argument. Split shell-style flags into an argument slice only with tests and release notes, because this fixes common multi-flag usage but changes exact legacy behavior. Reject malformed quoting instead of invoking a shell.
- **Debug JSON stability:** the final `UploadReleaseResponse` appears in Firebase debug output, which is not a formal machine-output interface. Keep the labeled-line fallback and fixtures from more than one Firebase CLI version. If both paths fail, return a clear error or warning according to the existing output contract.
- **Direct API option:** consider a second phase only if eliminating the Firebase CLI is a requirement. It would need OAuth/ADC support, upload operation polling, release-note updates, tester/group distribution, and a replacement policy for arbitrary CLI `flags`.
