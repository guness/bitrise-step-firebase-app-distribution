# Go migration record

## Compatibility baseline

Commit `8eb2b85` on `develop` fixes [#74](https://github.com/guness/bitrise-step-firebase-app-distribution/issues/74)
by exporting the documented `FIREBASE_BITRISE_APP_DISTRIBUTION_RELEASE_ID` and retaining
`FIREBASE_APP_DISTRIBUTION_RELEASE_ID` as an alias. Both are declared and tested.

The `dev/go` implementation preserves all 12 inputs and their defaults, all three URL
outputs, both release ID names, token/service-account auth, credential URLs, APK/AAB/IPA
support, note truncation and files, testers/groups, extra CLI flags, CLI installation and
upgrade, and debug log selection. A contract test freezes these names and defaults.

## Implemented design

- Bitrise uses `toolkit.go.package_name`, matching the Go module path.
- `main.go` handles cancellation and exit status. The small `internal/step` package
  separates configuration, process/network execution, parsing, and log redaction.
- Firebase CLI continues to perform uploads and distribution. Direct REST API migration
  would require new auth, upload polling, distribution, and CLI flag compatibility work;
  it is outside this migration.
- Process argument slices replace Bash `eval`. A pinned, vendored `go-shellquote`
  dependency handles additional flag quoting without evaluation or variable expansion.
- Inline notes use the equals form of the CLI option to preserve leading dashes.
- Go's JSON decoder replaces `jq`. Labeled-line parsing remains a fallback per field.
- Downloaded service credentials are private temporary files, removed on every path.
  An explicit service account is protected from an inherited `FIREBASE_TOKEN` override.
- The official Firebase installer still runs when required, using a downloaded temporary
  script and an explicit upgrade environment. Its result is checked before uploading.
- Known secrets are redacted from logs and final errors. Exported URLs are kept intact.
- `step.sh` remains as a safe Go build launcher for callers using the old entrypoint.

## GitHub regressions covered

| Report | Behavior now tested |
| --- | --- |
| [#17](https://github.com/guness/bitrise-step-firebase-app-distribution/issues/17) — quotes in notes | Double and single quotes remain part of one literal argument. |
| [#33](https://github.com/guness/bitrise-step-firebase-app-distribution/issues/33) — backticks execute commands | Backticks and command substitutions remain text; subprocess tests confirm no marker command runs. |
| [#40](https://github.com/guness/bitrise-step-firebase-app-distribution/issues/40) — notes beginning with a dash | Notes use `--release-notes=<value>`. |
| [#79](https://github.com/guness/bitrise-step-firebase-app-distribution/issues/79) — flags treated as one string | Multiple flags and quoted file paths retain correct argument boundaries. Documentation uses the valid option spelling without a colon. |
| [#80](https://github.com/guness/bitrise-step-firebase-app-distribution/issues/80) — no failure diagnostics | Both output streams are displayed on failure and the CLI exit status is preserved. |
| [#43](https://github.com/guness/bitrise-step-firebase-app-distribution/issues/43) — inherited auth override | Explicit service-account auth removes `FIREBASE_TOKEN` only from the child environment. |
| [#74](https://github.com/guness/bitrise-step-firebase-app-distribution/issues/74) — output name mismatch | Both release ID names hold the same value in Go and the existing Bitrise workflows. |

## Validation and release gates

Automated checks are `go test -race ./...`, `go vet ./...`, and `bitrise run test-hermetic`.
The latter audits the Step and exercises the original primary/fallback fixtures through
Bitrise's actual Go toolkit. Subprocess tests also exercise the executable entrypoint and
the shell launcher's working-directory behavior. Temporary HTTP servers cover credentials
and installation without accessing Firebase.

GitHub Actions tests Linux with Go 1.22 and current stable Go, plus macOS with current
stable Go. Release validation still requires real Android/iOS uploads and both auth modes;
hermetic tests cannot establish remote Firebase permissions or distribution success.
No new Step version is published as part of this development branch.

## Reference projects and contracts checked

- [Bitrise Step development guide](https://docs.bitrise.io/en/bitrise-ci/workflows-and-pipelines/developing-your-own-bitrise-step/developing-a-new-step)
- [Gradle Runner Go implementation](https://github.com/bitrise-steplib/steps-gradle-runner/blob/master/main.go) — CLI argument parsing and Go Step conventions
- [Android Build Step metadata](https://github.com/bitrise-steplib/bitrise-step-android-build/blob/master/step.yml) — native Go toolkit declaration
- [go-shellquote](https://github.com/kballard/go-shellquote) — quoting without shell evaluation
- [Firebase CLI distribution guide](https://firebase.google.com/docs/app-distribution/android/distribute-cli)
- [Firebase App Distribution REST API](https://firebase.google.com/docs/reference/app-distribution/rest)
