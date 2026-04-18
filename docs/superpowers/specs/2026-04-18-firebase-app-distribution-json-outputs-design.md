# Firebase App Distribution — JSON-first output parsing

**Date:** 2026-04-18
**Status:** Approved design, ready for implementation plan

## Problem

The step currently extracts output URLs from `firebase appdistribution:distribute` stdout by picking the *N-th* HTTP URL with a regex. This is fragile: any extra URL printed by Firebase (warnings, AAB integration messages, test-devices feature, deprecation notices) shifts positions and silently breaks the `FIREBASE_BITRISE_CONSOLE_URL` and `FIREBASE_BITRISE_APP_DISTRIBUTION_URL` outputs.

The user also wants a new third output — the signed binary download URL (`binaryDownloadUri`) — that is not exposed today.

## Constraints

- `firebase appdistribution:distribute` has **no** `--json` flag and its action function returns `void`. Verified against firebase-tools master: `src/commands/appdistribution-distribute.ts` and `src/appdistribution/distribution.ts`.
- Three URIs are available by two independent paths in CLI output:
  1. Human-readable `✔ ...:` success lines (always present, anchored to stable label strings emitted by `utils.logSuccess()`).
  2. A JSON blob inside `[timestamp] <<< [apiv2][body] GET .../operations/... {…,"done":true,"response":{"release":{…}}}` — only present when `--debug` is passed.
- Existing output env var names must be preserved (downstream workflows may depend on them).
- New output name must follow the existing `FIREBASE_BITRISE_*` prefix convention.

## Design

### Overall approach

Always invoke `firebase appdistribution:distribute` with `--debug` appended internally, regardless of the user's `is_debug` input. Capture all stdout+stderr into a single buffer. Parse the buffer for the JSON `UploadReleaseResponse` body (primary) and fall back to the `✔ ...:` labeled success lines (fallback). Filter what is printed back to the Bitrise log so that users see only the meaningful lines unless they opted into debug output.

### Data flow

```
firebase ... --debug
   │
   ▼
captured buffer ($output, stdout+stderr)
   │
   ├── display filter ─────► Bitrise log
   │                          (raw if is_debug=true,
   │                           ✔/i/⚠/error lines only otherwise)
   │
   └── extractor
         │
         ├── primary: find last [apiv2][body] line containing
         │           "done":true and "release":{…}
         │           strip prefix up to first `{`
         │           jq -r over .response.release.*
         │
         └── fallback (if primary returned empty):
                     grep -Eo on the three labeled success lines,
                     awk '{print $NF}' the URL
```

### Inputs

No change. The existing `is_debug` input is preserved with its current meaning from the user's perspective: `true` shows the full raw Firebase output; `false` (default) shows only the filtered subset. Internally the step always passes `--debug` to the CLI so the JSON is available for parsing.

### Outputs

| Env var | Source (primary JSON) | Source (fallback label) | Status |
|---|---|---|---|
| `FIREBASE_BITRISE_CONSOLE_URL` | `.response.release.firebaseConsoleUri` | `View this release in the Firebase console: <url>` | kept |
| `FIREBASE_BITRISE_APP_DISTRIBUTION_URL` | `.response.release.testingUri` | `Share this release with testers who have access: <url>` | kept |
| `FIREBASE_BITRISE_BINARY_DOWNLOAD_URL` | `.response.release.binaryDownloadUri` | `Download the release binary (link expires in 1 hour): <url>` | **new** |
| `FIREBASE_APP_DISTRIBUTION_RELEASE_ID` | last segment of `.response.release.name` | last segment of any `/releases/<id>` in the URIs above | kept |

### step.yml changes

Add one `outputs` entry:

```yaml
- FIREBASE_BITRISE_BINARY_DOWNLOAD_URL:
  opts:
    title: Firebase App Distribution Binary Download URL
    summary: Signed download URL for the uploaded APK/AAB/IPA (expires in one hour)
```

No removals. No changes to existing input declarations. The pre-existing inconsistency where `step.yml` declares `FIREBASE_BITRISE_APP_DISTRIBUTION_RELEASE_ID` while `step.sh` exports `FIREBASE_APP_DISTRIBUTION_RELEASE_ID` is **intentionally left alone** — fixing it is a separate breaking change and is out of scope for this work.

### step.sh changes

#### Append `--debug` unconditionally

Before the command is `eval`ed:

```bash
submit_cmd="$submit_cmd --debug"
```

The existing `is_debug` block is removed from the command-construction section (it no longer gates `--debug` on the CLI). The input is consulted only for display filtering, below.

#### Display filter

After `output=$(eval "${submit_cmd}" 2>&1)`:

```bash
if [ "${is_debug}" = true ] ; then
    echo "$output"
else
    printf '%s\n' "$output" | grep -E '^(✔|i |⚠|!|Error:)' || true
fi
```

`|| true` prevents `set -e` from failing the step when grep finds no matches.

#### Primary extraction

```bash
release_json=$(printf '%s\n' "$output" \
  | grep -F '[apiv2][body]' \
  | grep -F '"done":true' \
  | grep -F '"release":' \
  | tail -n1 \
  | sed -E 's/^[^{]*//')

firebase_console_uri=$(printf '%s' "$release_json" | jq -r '.response.release.firebaseConsoleUri // empty' 2>/dev/null)
testing_uri=$(printf         '%s' "$release_json" | jq -r '.response.release.testingUri // empty'         2>/dev/null)
binary_download_uri=$(printf '%s' "$release_json" | jq -r '.response.release.binaryDownloadUri // empty'  2>/dev/null)
release_name=$(printf        '%s' "$release_json" | jq -r '.response.release.name // empty'               2>/dev/null)
release_id="${release_name##*/}"
```

#### Fallback extraction

For any field whose primary value is empty, fall back to labeled-line parsing:

```bash
if [ -z "${firebase_console_uri}" ] ; then
    firebase_console_uri=$(printf '%s\n' "$output" \
      | grep -Eo 'View this release in the Firebase console: \S+' \
      | awk '{print $NF}' | tail -n1)
fi
# same pattern for testing_uri and binary_download_uri

if [ -z "${release_id}" ] && [ -n "${firebase_console_uri}" ] ; then
    release_id=$(printf '%s' "$firebase_console_uri" \
      | grep -Eo '/releases/[a-zA-Z0-9]+' | head -n1 | awk -F'/' '{print $3}')
fi
```

#### Export

Each URI is exported via `envman add --key KEY --value "${value}"` only if non-empty; if still empty after fallback, print a yellow warning and export an empty value (current behavior preserved).

### Error handling

| Condition | Behavior |
|---|---|
| `jq` not installed | Hard fail with a clear message instructing the user to install `jq`. Defensive only — Bitrise stacks ship jq. |
| Primary JSON extraction empty | Silently fall back to labeled-line parsing. No log output. |
| Both paths empty for a URI | Yellow warning via `echo_warn`, export empty value, do not fail build. |
| Firebase CLI exits non-zero | Unchanged: `eval` returns non-zero exit, `set -e` fails the step. |

### Testing

1. **`bitrise run audit-this-step`** — validates `step.yml` after adding the new output.
2. **`bitrise run test-android`** — existing workflow, flip `is_debug` to `false` so filter logic is exercised. Assertions added via an `after_run` script that calls `envman print` and fails if any of the four outputs are empty.
3. **`bitrise run test-ios`** — same as above, exercises the Firebase-token auth path.
4. **`bitrise run test-fallback-parsing`** — new workflow. Shims `firebase` on `PATH` with a bash stub that emits only the three `✔ ...:` lines (no `[apiv2][body]` debug JSON), runs the step, and asserts all four outputs land correctly. ~30 lines in `bitrise.yml`.

### CLAUDE.md

Replace the "Outputs are scraped from CLI stdout via regex" bullet with an updated description of the JSON-first / labeled-line-fallback strategy, noting that `--debug` is now always passed internally and that `jq` is required.

## Out of scope

- Exposing `displayVersion`, `buildVersion`, `createTime`, `expireTime`, `updateTime`, or `result` as outputs.
- Renaming `FIREBASE_APP_DISTRIBUTION_RELEASE_ID` to match the `FIREBASE_BITRISE_*` prefix.
- Removing the existing `is_debug` input.
- Switching away from `eval` for command invocation.
