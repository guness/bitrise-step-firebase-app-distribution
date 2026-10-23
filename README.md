# Firebase App Distribution

Upload APK, AAB, and IPA builds to Firebase App Distribution and distribute them to testers.

The step is implemented in Go and uses the Firebase CLI for distribution. It keeps the
existing Bitrise input names and defaults, and exports the same release links and IDs.

## Usage

```yaml
- firebase-app-distribution:
    inputs:
    - app: "$FIREBASE_APP_ID"
    - app_path: "$BITRISE_APK_PATH"
    - service_credentials_file: "$SERVICE_ACCOUNT_FILE_OR_URL"
    - release_notes: "Bug fixes and improvements"
    - groups: "qa-team"
```

Use `firebase_token` instead of `service_credentials_file` for an existing Firebase CI
token setup. An explicit token takes precedence when both inputs are supplied, with a
warning. When a service account is selected, an unrelated `FIREBASE_TOKEN` environment
variable is ignored in the Firebase subprocess so it cannot override that account.

A service credential input may be a local file or an HTTP(S) URL. Downloaded files have
private permissions and are removed after execution, including on failure. Local files
provided by the caller are left in place.

## Inputs

| Input | Behavior / default |
| --- | --- |
| `firebase_token` | Firebase CI token; sensitive. Required if no service credential input is provided. |
| `service_credentials_file` | Path or HTTP(S) URL to a service account JSON file. |
| `app_path` | APK, AAB, or IPA path. Default: `$BITRISE_IPA_PATH\|$BITRISE_APK_PATH\|$BITRISE_AAB_PATH`; exactly one path must be populated. |
| `app` | Required Firebase App ID, such as `1:1234567890:android:0a1b2c3d4e5f67890`. |
| `release_notes` | Inline release notes, passed literally including quotes, backticks, dollar signs, backslashes, and newlines. |
| `release_notes_length` | Positive maximum Unicode character count for inline notes, including the trailing ellipsis. Empty, zero, or negative disables truncation. |
| `release_notes_file` | Optional path to a text file. Missing files warn and are skipped; file contents are not truncated. |
| `testers` | Comma-separated tester email addresses. |
| `groups` | Comma-separated tester group aliases. |
| `flags` | Additional Firebase CLI arguments; supports quoting and backslash escapes. |
| `upgrade_firebase_tools` | `false` by default. When `true`, run the official Firebase installer in upgrade mode. |
| `is_debug` | `false` by default. When `true`, display Firebase debug diagnostics as well as ordinary messages. |

### Quoting and additional flags

Ordinary input values are passed as individual process arguments. They are never run as
shell commands. Release notes beginning with a dash are supported, including bullet lists.

The `flags` input is split into arguments using shell-style quoting. For example:

```text
--test-devices-file "$PROJECT_LOCATION/test devices.txt" --test-non-blocking
```

Use a space or `=` after the option name, **not a colon**. Bitrise expands environment
references before the step receives its inputs. The step does not perform a second round
of variable expansion, command substitution, redirection, or glob expansion. Malformed
quoting produces a clear error before an upload is attempted.

## Outputs

| Environment variable | Value |
| --- | --- |
| `FIREBASE_BITRISE_CONSOLE_URL` | Firebase console URL for the release. |
| `FIREBASE_BITRISE_APP_DISTRIBUTION_URL` | Release URL for testers with access. |
| `FIREBASE_BITRISE_BINARY_DOWNLOAD_URL` | Signed binary download URL; expires after one hour. |
| `FIREBASE_BITRISE_APP_DISTRIBUTION_RELEASE_ID` | Release ID under the documented output name. |
| `FIREBASE_APP_DISTRIBUTION_RELEASE_ID` | Existing compatibility alias for the same release ID. |

The step first reads the final upload operation JSON from Firebase debug output and then
uses labeled success lines for any missing fields. Missing values warn and are exported
as empty strings. Firebase failure diagnostics are printed and its nonzero exit code is
preserved; a failed distribution does not export success outputs.

Firebase `--debug` is always requested internally for parsing. `is_debug` controls log
display. Known tokens and signed download query tokens are redacted in logs; the exported
URLs retain their full values. Debug diagnostics can still contain application metadata.

## Development

Requires Go 1.22+, the Bitrise CLI, and its `envman` tool. The small argument quoting
library is pinned and vendored. The Go implementation has no `jq` dependency.

```sh
go test -race ./...
go vet ./...
bitrise run test-hermetic
```

`test-hermetic` audits `step.yml`, runs the Go regression tests, and runs the two original
Firebase output stub workflows through Bitrise's Go toolkit. It needs no Firebase
credentials and does not upload a release. Go tests cover argument boundaries, GitHub
escaping regressions, credential downloads and cleanup, installer behavior, output
parsing, failure diagnostics, and compatibility of the public contract.

For a direct invocation, export the inputs from `step.yml` and run `go run .` or `./step.sh`.
The shell entrypoint is a small Go build launcher and preserves the caller's working
directory. Ensure `envman` is on `PATH` so outputs can be exported.

### Real application validation and release

`bitrise run test-android` and `bitrise run test-ios` build and distribute the sample apps
configured in `bitrise.yml`. Supply the relevant credentials through the ignored
`.bitrise.secrets.yml`. These workflows upload actual builds and may notify testers.
`bitrise run test` runs the hermetic checks followed by both platform workflows.

Before publishing, validate real Android and iOS releases, bump `BITRISE_STEP_VERSION`,
run `stepman audit --step-yml ./step.yml`, and use `bitrise run share-this-step` to prepare
the StepLib update. Development branches can be tested with a `git::` Step reference:

```yaml
- git::https://github.com/guness/bitrise-step-firebase-app-distribution.git@dev/go:
    inputs:
    - app: "$FIREBASE_APP_ID"
    - app_path: "$BITRISE_APK_PATH"
    - service_credentials_file: "$SERVICE_ACCOUNT_FILE_OR_URL"
```

See [the migration record](docs/go-migration-plan.md) for the design and reference projects.
