# Firebase App Distribution — JSON-first Output Parsing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace fragile positional URL regex in `step.sh` with JSON-first parsing of the Firebase CLI debug output, with labeled-line fallback. Add new `FIREBASE_BITRISE_BINARY_DOWNLOAD_URL` output. Preserve all existing output names and the user-facing meaning of `is_debug`.

**Architecture:** Always pass `--debug` to `firebase appdistribution:distribute` internally. Capture all stdout+stderr. Locate the single `[apiv2][body]` line containing `"done":true` and `"release":{`, extract the JSON with `jq`, export four output env vars. If the JSON extraction fails for any URI, fall back to grep on the `✔ ...:` labeled lines. Filter what gets echoed back to the Bitrise log so it stays clean unless `is_debug=true`.

**Tech Stack:** Bash 3.2+, `jq`, `grep -E`/`awk`/`sed`, `envman` (Bitrise), `firebase-tools` (Node CLI). Tests run via the `bitrise` CLI invoking stubbed `firebase` binaries on `PATH`.

**Spec:** `docs/superpowers/specs/2026-04-18-firebase-app-distribution-json-outputs-design.md`

---

## File structure

Files created:
- `_fixtures/firebase-stub-full-debug.sh` — hermetic `firebase` stub emitting realistic `--debug` output containing the `UploadReleaseResponse` JSON body. Used by the primary-path test.
- `_fixtures/firebase-stub-labeled-only.sh` — hermetic `firebase` stub emitting *only* the three `✔ ...:` success lines (no debug JSON). Used by the fallback-path test.

Files modified:
- `step.sh` — always append `--debug`; add `jq` dependency check; replace positional URL scraping with JSON-first + labeled-line fallback; add display filter.
- `step.yml` — add one new `outputs` entry for `FIREBASE_BITRISE_BINARY_DOWNLOAD_URL`.
- `bitrise.yml` — add `test-parse-primary` and `test-parse-fallback` workflows; modify `deploy-android` and `deploy-ios` to assert outputs are non-empty.
- `CLAUDE.md` — refresh the "Outputs are scraped from CLI stdout via regex" paragraph.

No other files touched. Existing input surface, auth handling, `app_path` pipe-handling, and release-notes logic stay identical.

---

## Task 1: Create full-debug firebase stub fixture

**Files:**
- Create: `_fixtures/firebase-stub-full-debug.sh`

- [ ] **Step 1: Create the fixture file**

Write `_fixtures/firebase-stub-full-debug.sh` with this exact content:

```bash
#!/usr/bin/env bash
# Stub that mimics the output of `firebase appdistribution:distribute --debug`.
# Emits a realistic debug log including the UploadReleaseResponse JSON body
# and the three trailing `✔ ...:` labeled success lines. Ignores all arguments.
cat <<'STUB_OUT'
(node:61133) [DEP0040] DeprecationWarning: The `punycode` module is deprecated.
[2026-04-17T23:22:09.691Z] Checked if tokens are valid: true, expires at: 1776471135808
[2026-04-17T23:22:10.929Z] <<< [apiv2][body] GET https://firebaseappdistribution.googleapis.com/v1/projects/TEST_PROJ/apps/TEST_APP/aabInfo {"name":"projects/TEST_PROJ/apps/TEST_APP/aabInfo","integrationState":"INTEGRATED"}
i  uploading binary...
[2026-04-17T23:22:30.586Z] <<< [apiv2][body] POST https://firebaseappdistribution.googleapis.com/upload/v1/projects/TEST_PROJ/apps/TEST_APP/releases:upload {"name":"projects/TEST_PROJ/apps/TEST_APP/releases/-/operations/OPID"}
[2026-04-17T23:22:31.404Z] <<< [apiv2][body] GET https://firebaseappdistribution.googleapis.com/v1/projects/TEST_PROJ/apps/TEST_APP/releases/-/operations/OPID {"name":"projects/TEST_PROJ/apps/TEST_APP/releases/-/operations/OPID"}
[2026-04-17T23:22:33.405Z] [App Distribution Upload Poller] Retrying task index 0
[2026-04-17T23:22:34.270Z] <<< [apiv2][body] GET https://firebaseappdistribution.googleapis.com/v1/projects/TEST_PROJ/apps/TEST_APP/releases/-/operations/OPID {"name":"projects/TEST_PROJ/apps/TEST_APP/releases/-/operations/OPID","done":true,"response":{"@type":"type.googleapis.com/google.firebase.appdistro.v1.UploadReleaseResponse","result":"RELEASE_CREATED","release":{"name":"projects/TEST_PROJ/apps/TEST_APP/releases/PRIMARYID","displayVersion":"1.0.0","buildVersion":"1","createTime":"2026-04-17T23:02:39.062816Z","firebaseConsoleUri":"https://console.firebase.google.com/project/TEST_PROJ/appdistribution/app/TEST_APP/releases/PRIMARYID?utm_source=firebase-tools","testingUri":"https://appdistribution.firebase.google.com/testerapps/TEST_APP/releases/PRIMARYID?utm_source=firebase-tools","binaryDownloadUri":"https://firebaseappdistribution.googleapis.com/app-binary-downloads/projects/TEST_PROJ/apps/TEST_APP/releases/PRIMARYID/binaries/BINID/app.aab?utm_source=firebase-tools&token=FAKETOKEN","updateTime":"2026-04-17T23:02:39.156501Z","expireTime":"2026-09-14T23:02:39.062816Z"}}}
✔  uploaded new release 1.0.0 (1) successfully!
✔  View this release in the Firebase console: https://console.firebase.google.com/project/TEST_PROJ/appdistribution/app/TEST_APP/releases/PRIMARYID?utm_source=firebase-tools
✔  Share this release with testers who have access: https://appdistribution.firebase.google.com/testerapps/TEST_APP/releases/PRIMARYID?utm_source=firebase-tools
✔  Download the release binary (link expires in 1 hour): https://firebaseappdistribution.googleapis.com/app-binary-downloads/projects/TEST_PROJ/apps/TEST_APP/releases/PRIMARYID/binaries/BINID/app.aab?utm_source=firebase-tools&token=FAKETOKEN
STUB_OUT
```

- [ ] **Step 2: Make it executable**

Run: `chmod +x _fixtures/firebase-stub-full-debug.sh`

- [ ] **Step 3: Sanity-check the stub**

Run: `./_fixtures/firebase-stub-full-debug.sh appdistribution:distribute app.aab --app foo --debug | tail -4`

Expected last four lines:
```
✔  uploaded new release 1.0.0 (1) successfully!
✔  View this release in the Firebase console: https://console.firebase.google.com/project/TEST_PROJ/appdistribution/app/TEST_APP/releases/PRIMARYID?utm_source=firebase-tools
✔  Share this release with testers who have access: https://appdistribution.firebase.google.com/testerapps/TEST_APP/releases/PRIMARYID?utm_source=firebase-tools
✔  Download the release binary (link expires in 1 hour): https://firebaseappdistribution.googleapis.com/app-binary-downloads/projects/TEST_PROJ/apps/TEST_APP/releases/PRIMARYID/binaries/BINID/app.aab?utm_source=firebase-tools&token=FAKETOKEN
```

- [ ] **Step 4: Commit**

```bash
git add _fixtures/firebase-stub-full-debug.sh
git commit -m "test: add firebase CLI stub emitting full --debug output"
```

---

## Task 2: Create labeled-only firebase stub fixture

**Files:**
- Create: `_fixtures/firebase-stub-labeled-only.sh`

- [ ] **Step 1: Create the fixture file**

Write `_fixtures/firebase-stub-labeled-only.sh` with this exact content:

```bash
#!/usr/bin/env bash
# Stub that mimics the output of `firebase appdistribution:distribute` WITHOUT
# the `--debug` log lines — i.e., a hypothetical future CLI version where the
# `[apiv2][body]` debug format has been removed or renamed. Exercises the
# fallback labeled-line parser. Ignores all arguments.
cat <<'STUB_OUT'
✔  uploaded new release 2.0.0 (5) successfully!
✔  View this release in the Firebase console: https://console.firebase.google.com/project/TEST_PROJ/appdistribution/app/TEST_APP/releases/FALLBACKID?utm_source=firebase-tools
✔  Share this release with testers who have access: https://appdistribution.firebase.google.com/testerapps/TEST_APP/releases/FALLBACKID?utm_source=firebase-tools
✔  Download the release binary (link expires in 1 hour): https://firebaseappdistribution.googleapis.com/app-binary-downloads/projects/TEST_PROJ/apps/TEST_APP/releases/FALLBACKID/binaries/BIN2/app.aab?utm_source=firebase-tools&token=FAKE2
STUB_OUT
```

- [ ] **Step 2: Make it executable**

Run: `chmod +x _fixtures/firebase-stub-labeled-only.sh`

- [ ] **Step 3: Sanity-check the stub**

Run: `./_fixtures/firebase-stub-labeled-only.sh | wc -l`

Expected: `4`

- [ ] **Step 4: Commit**

```bash
git add _fixtures/firebase-stub-labeled-only.sh
git commit -m "test: add firebase CLI stub emitting labeled-only output"
```

---

## Task 3: Add `test-parse-primary` workflow (RED)

**Files:**
- Modify: `bitrise.yml` (append new workflow at end of `workflows:` block)

- [ ] **Step 1: Append the new workflow**

Add this block to `bitrise.yml` under `workflows:`:

```yaml
  test-parse-primary:
    description: |-
      Exercises the JSON-first parser against a hermetic `firebase` stub that
      emits realistic `--debug` output. Does not touch real Firebase.
    steps:
    - script:
        title: Prepare firebase stub (full-debug)
        inputs:
        - content: |
            #!/usr/bin/env bash
            set -ex
            STUB_DIR="$BITRISE_SOURCE_DIR/_tmp/stub-primary"
            rm -rf "$STUB_DIR"
            mkdir -p "$STUB_DIR"
            cp "$BITRISE_SOURCE_DIR/_fixtures/firebase-stub-full-debug.sh" "$STUB_DIR/firebase"
            chmod +x "$STUB_DIR/firebase"
            echo "stub" > "$STUB_DIR/app.aab"
            envman add --key PATH --value "$STUB_DIR:$PATH"
            envman add --key STUB_APP_PATH --value "$STUB_DIR/app.aab"
    - path::./:
        title: Run step against stub
        inputs:
        - app: "1:1:android:1"
        - app_path: $STUB_APP_PATH
        - firebase_token: stub-token
        - is_debug: "false"
    - script:
        title: Assert outputs
        inputs:
        - content: |
            #!/usr/bin/env bash
            set -e
            fail=0
            check() {
              local name="$1" expected="$2" actual="$3"
              if [ "$actual" != "$expected" ] ; then
                echo "FAIL $name: expected [$expected] got [$actual]"
                fail=1
              else
                echo "OK   $name"
              fi
            }
            check FIREBASE_BITRISE_CONSOLE_URL \
              "https://console.firebase.google.com/project/TEST_PROJ/appdistribution/app/TEST_APP/releases/PRIMARYID?utm_source=firebase-tools" \
              "$FIREBASE_BITRISE_CONSOLE_URL"
            check FIREBASE_BITRISE_APP_DISTRIBUTION_URL \
              "https://appdistribution.firebase.google.com/testerapps/TEST_APP/releases/PRIMARYID?utm_source=firebase-tools" \
              "$FIREBASE_BITRISE_APP_DISTRIBUTION_URL"
            check FIREBASE_BITRISE_BINARY_DOWNLOAD_URL \
              "https://firebaseappdistribution.googleapis.com/app-binary-downloads/projects/TEST_PROJ/apps/TEST_APP/releases/PRIMARYID/binaries/BINID/app.aab?utm_source=firebase-tools&token=FAKETOKEN" \
              "$FIREBASE_BITRISE_BINARY_DOWNLOAD_URL"
            check FIREBASE_APP_DISTRIBUTION_RELEASE_ID "PRIMARYID" "$FIREBASE_APP_DISTRIBUTION_RELEASE_ID"
            exit $fail
```

- [ ] **Step 2: Run the workflow to confirm it fails**

Run: `bitrise run test-parse-primary`

Expected: the "Assert outputs" step fails. Specifically, `FIREBASE_BITRISE_BINARY_DOWNLOAD_URL` will be empty (current `step.sh` doesn't export it), and `FIREBASE_APP_DISTRIBUTION_RELEASE_ID` will be `OPID` (first `/releases/…` match in output is from a polling URL, not the final release). The workflow exits non-zero.

- [ ] **Step 3: Commit the failing test**

```bash
git add bitrise.yml
git commit -m "test: add test-parse-primary workflow (currently RED)"
```

---

## Task 4: Add `test-parse-fallback` workflow (RED)

**Files:**
- Modify: `bitrise.yml` (append new workflow after `test-parse-primary`)

- [ ] **Step 1: Append the new workflow**

Add this block to `bitrise.yml` directly below the `test-parse-primary` workflow:

```yaml
  test-parse-fallback:
    description: |-
      Exercises the labeled-line fallback parser against a hermetic `firebase`
      stub that omits the `[apiv2][body]` debug JSON entirely. Simulates a
      future CLI version that drops the debug log format we rely on.
    steps:
    - script:
        title: Prepare firebase stub (labeled-only)
        inputs:
        - content: |
            #!/usr/bin/env bash
            set -ex
            STUB_DIR="$BITRISE_SOURCE_DIR/_tmp/stub-fallback"
            rm -rf "$STUB_DIR"
            mkdir -p "$STUB_DIR"
            cp "$BITRISE_SOURCE_DIR/_fixtures/firebase-stub-labeled-only.sh" "$STUB_DIR/firebase"
            chmod +x "$STUB_DIR/firebase"
            echo "stub" > "$STUB_DIR/app.aab"
            envman add --key PATH --value "$STUB_DIR:$PATH"
            envman add --key STUB_APP_PATH --value "$STUB_DIR/app.aab"
    - path::./:
        title: Run step against stub
        inputs:
        - app: "1:1:android:1"
        - app_path: $STUB_APP_PATH
        - firebase_token: stub-token
        - is_debug: "false"
    - script:
        title: Assert outputs
        inputs:
        - content: |
            #!/usr/bin/env bash
            set -e
            fail=0
            check() {
              local name="$1" expected="$2" actual="$3"
              if [ "$actual" != "$expected" ] ; then
                echo "FAIL $name: expected [$expected] got [$actual]"
                fail=1
              else
                echo "OK   $name"
              fi
            }
            check FIREBASE_BITRISE_CONSOLE_URL \
              "https://console.firebase.google.com/project/TEST_PROJ/appdistribution/app/TEST_APP/releases/FALLBACKID?utm_source=firebase-tools" \
              "$FIREBASE_BITRISE_CONSOLE_URL"
            check FIREBASE_BITRISE_APP_DISTRIBUTION_URL \
              "https://appdistribution.firebase.google.com/testerapps/TEST_APP/releases/FALLBACKID?utm_source=firebase-tools" \
              "$FIREBASE_BITRISE_APP_DISTRIBUTION_URL"
            check FIREBASE_BITRISE_BINARY_DOWNLOAD_URL \
              "https://firebaseappdistribution.googleapis.com/app-binary-downloads/projects/TEST_PROJ/apps/TEST_APP/releases/FALLBACKID/binaries/BIN2/app.aab?utm_source=firebase-tools&token=FAKE2" \
              "$FIREBASE_BITRISE_BINARY_DOWNLOAD_URL"
            check FIREBASE_APP_DISTRIBUTION_RELEASE_ID "FALLBACKID" "$FIREBASE_APP_DISTRIBUTION_RELEASE_ID"
            exit $fail
```

- [ ] **Step 2: Run the workflow to confirm it fails**

Run: `bitrise run test-parse-fallback`

Expected: the "Assert outputs" step fails — `FIREBASE_BITRISE_BINARY_DOWNLOAD_URL` is empty (not yet exported by `step.sh`).

- [ ] **Step 3: Commit the failing test**

```bash
git add bitrise.yml
git commit -m "test: add test-parse-fallback workflow (currently RED)"
```

---

## Task 5: Rewire `step.sh` — unconditional `--debug`, `jq` check, JSON-first parser, fallback, display filter

**Files:**
- Modify: `step.sh`

This task replaces the block from `if [ "${is_debug}" = true ]` (line ~254) through the end of the file with the new logic. Do it in four surgical edits, running the tests after the final edit.

- [ ] **Step 1: Remove the conditional `--debug` append**

In `step.sh`, find these four lines:

```bash
if [ "${is_debug}" = true ] ; then
    submit_cmd="$submit_cmd --debug"
fi
```

Replace with:

```bash
# --debug is always appended so we can parse the UploadReleaseResponse JSON
# from the debug body line. Display filtering below decides what to echo.
submit_cmd="$submit_cmd --debug"
```

- [ ] **Step 2: Add jq dependency check**

In `step.sh`, directly *above* the `echo_details "$submit_cmd"` line (~line 258 originally), insert:

```bash
if ! command -v jq >/dev/null 2>&1 ; then
    echo_fail "jq is required by this step but was not found on PATH. Please install jq and try again."
fi
```

- [ ] **Step 3: Replace the parsing-and-display block**

In `step.sh`, find and delete the entire block from (and including) the line:

```bash
# Adjust the number of `sed -n 2p` if the position of the URL changes in the output
```

through the final line of the file (ending with `fi`). Replace the deleted block with the following (place immediately after `output=$(eval "${submit_cmd}" 2>&1)`):

```bash
# --- Display filter ----------------------------------------------------------
# If the caller asked for debug, echo everything. Otherwise show only the
# meaningful lines Firebase emits (✔ successes, i bullets, ! warnings, ⚠
# warnings, explicit Error: lines). `|| true` so no-match doesn't trip `set -e`.
if [ "${is_debug}" = true ] ; then
    echo "$output"
else
    printf '%s\n' "$output" | grep -E '^(✔|i |⚠|!|Error:)' || true
fi

# --- Primary extraction: JSON from [apiv2][body] debug line ------------------
# Locate the single debug body line that contains both "done":true and a
# "release":{ object — that's the final long-running operation response that
# carries the Release fields we need. Then strip everything before the first
# `{` to hand jq a clean JSON document.
release_json=$(printf '%s\n' "$output" \
    | grep -F '[apiv2][body]' \
    | grep -F '"done":true' \
    | grep -F '"release":' \
    | tail -n1 \
    | sed -E 's/^[^{]*//')

firebase_console_uri=""
testing_uri=""
binary_download_uri=""
release_id=""

if [ -n "${release_json}" ] ; then
    firebase_console_uri=$(printf '%s' "$release_json" | jq -r '.response.release.firebaseConsoleUri // empty' 2>/dev/null)
    testing_uri=$(printf         '%s' "$release_json" | jq -r '.response.release.testingUri // empty'         2>/dev/null)
    binary_download_uri=$(printf '%s' "$release_json" | jq -r '.response.release.binaryDownloadUri // empty'  2>/dev/null)
    release_name=$(printf        '%s' "$release_json" | jq -r '.response.release.name // empty'               2>/dev/null)
    release_id="${release_name##*/}"
fi

# --- Fallback: labeled `✔ ...:` success lines --------------------------------
# Runs only for fields still empty after the primary path. Keeps the step
# working if Firebase ever drops or renames the debug JSON format.
if [ -z "${firebase_console_uri}" ] ; then
    firebase_console_uri=$(printf '%s\n' "$output" \
      | grep -Eo 'View this release in the Firebase console: \S+' \
      | awk '{print $NF}' | tail -n1)
fi
if [ -z "${testing_uri}" ] ; then
    testing_uri=$(printf '%s\n' "$output" \
      | grep -Eo 'Share this release with testers who have access: \S+' \
      | awk '{print $NF}' | tail -n1)
fi
if [ -z "${binary_download_uri}" ] ; then
    binary_download_uri=$(printf '%s\n' "$output" \
      | grep -Eo 'Download the release binary \(link expires in 1 hour\): \S+' \
      | awk '{print $NF}' | tail -n1)
fi
if [ -z "${release_id}" ] && [ -n "${firebase_console_uri}" ] ; then
    release_id=$(printf '%s' "$firebase_console_uri" \
      | grep -Eo '/releases/[a-zA-Z0-9]+' | head -n1 | awk -F'/' '{print $3}')
fi

# --- Export ------------------------------------------------------------------
export_output() {
    local key="$1" value="$2" label="$3"
    if [ -z "${value}" ] ; then
        echo_warn "${label} could not be extracted from Firebase output."
    else
        echo_info "${label}: ${value}"
    fi
    envman add --key "${key}" --value "${value}"
}

export_output FIREBASE_BITRISE_CONSOLE_URL            "${firebase_console_uri}" "firebase console url"
export_output FIREBASE_BITRISE_APP_DISTRIBUTION_URL   "${testing_uri}"          "firebase app distribution url"
export_output FIREBASE_BITRISE_BINARY_DOWNLOAD_URL    "${binary_download_uri}"  "firebase binary download url"
export_output FIREBASE_APP_DISTRIBUTION_RELEASE_ID    "${release_id}"           "firebase release id"

echo_done "Success"
```

Note: the trailing `if [ $? -eq 0 ] ; then echo_done "Success" else echo_fail "Fail" fi` from the old file is deliberately dropped. `set -e` at the top of the file already exits on the `eval` failing; reaching the bottom means success.

- [ ] **Step 4: Run the primary test to verify it passes**

Run: `bitrise run test-parse-primary`

Expected: all four `OK` lines in the "Assert outputs" step; workflow exits 0.

- [ ] **Step 5: Run the fallback test to verify it passes**

Run: `bitrise run test-parse-fallback`

Expected: all four `OK` lines; workflow exits 0.

- [ ] **Step 6: Commit**

```bash
git add step.sh
git commit -m "feat: parse Firebase --debug JSON body, add binary download URL output"
```

---

## Task 6: Declare the new output in `step.yml`

**Files:**
- Modify: `step.yml` (append to `outputs:` block, preserving existing entries)

- [ ] **Step 1: Add the new output entry**

In `step.yml`, locate the `outputs:` block (starts ~line 141) and append this entry below the existing `FIREBASE_BITRISE_APP_DISTRIBUTION_URL` entry and before `FIREBASE_BITRISE_APP_DISTRIBUTION_RELEASE_ID`:

```yaml
  - FIREBASE_BITRISE_BINARY_DOWNLOAD_URL:
    opts:
      title: Firebase App Distribution Binary Download URL
      summary: Signed download URL for the uploaded APK/AAB/IPA (expires in one hour)
```

- [ ] **Step 2: Audit the step manifest**

Run: `bitrise run audit-this-step`

Expected: `stepman audit` exits 0 with no validation errors.

- [ ] **Step 3: Re-run the two parse tests to confirm nothing regressed**

Run: `bitrise run test-parse-primary && bitrise run test-parse-fallback`

Expected: both pass.

- [ ] **Step 4: Commit**

```bash
git add step.yml
git commit -m "feat(step.yml): declare FIREBASE_BITRISE_BINARY_DOWNLOAD_URL output"
```

---

## Task 7: Update CLAUDE.md documentation

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Replace the outputs-scraping bullet**

In `CLAUDE.md`, find the bullet starting with:

```
- **Outputs are scraped from CLI stdout via regex**:
```

Replace that entire bullet (through the following blank line) with:

```
- **Outputs come from the Firebase `--debug` JSON body, with a labeled-line fallback**: `step.sh` always passes `--debug` to `firebase appdistribution:distribute` internally (regardless of the `is_debug` input). It parses the `UploadReleaseResponse` JSON from the `[apiv2][body]` debug line via `jq` to populate `FIREBASE_BITRISE_CONSOLE_URL`, `FIREBASE_BITRISE_APP_DISTRIBUTION_URL`, `FIREBASE_BITRISE_BINARY_DOWNLOAD_URL`, and `FIREBASE_APP_DISTRIBUTION_RELEASE_ID`. If the JSON is missing (future Firebase CLI could change), it falls back to grepping the `✔ View/Share/Download …` labeled success lines. The `is_debug` input now controls *display only*: `false` (default) filters the Bitrise log to `✔`/`i`/`⚠`/`!`/`Error:` prefixed lines; `true` echoes the full raw debug output. **`jq` is a hard dependency** — Bitrise stacks ship it, but the step hard-fails with a clear message if it's missing. Hermetic tests for both paths live in `bitrise run test-parse-primary` / `test-parse-fallback` using stubs in `_fixtures/`.
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs(CLAUDE.md): document JSON-first output parsing and jq dependency"
```

---

## Task 8: Add output assertions to real-Firebase E2E workflows

**Files:**
- Modify: `bitrise.yml` (workflows `deploy-android` and `deploy-ios`)

- [ ] **Step 1: Flip `is_debug` to `false` and append an assertion step to `deploy-android`**

In `bitrise.yml`, locate the `deploy-android` workflow. Change the `is_debug: "true"` input to `is_debug: "false"`. Then append a new assertion step to the workflow's `steps:` list:

```yaml
    - script:
        title: Assert outputs populated
        inputs:
        - content: |
            #!/usr/bin/env bash
            set -e
            fail=0
            for var in FIREBASE_BITRISE_CONSOLE_URL FIREBASE_BITRISE_APP_DISTRIBUTION_URL FIREBASE_BITRISE_BINARY_DOWNLOAD_URL FIREBASE_APP_DISTRIBUTION_RELEASE_ID ; do
              eval "value=\$$var"
              if [ -z "$value" ] ; then
                echo "FAIL $var is empty"
                fail=1
              else
                echo "OK   $var = $value"
              fi
            done
            exit $fail
```

- [ ] **Step 2: Apply the same two changes to `deploy-ios`**

Make the same edits in the `deploy-ios` workflow: flip `is_debug: "true"` to `is_debug: "false"`, append the same assertion step verbatim.

- [ ] **Step 3: Run full test (requires real Firebase credentials)**

Run: `bitrise run test`

Expected: full suite (android build+deploy+assert, ios build+deploy+assert) passes. If running without valid credentials in `.bitrise.secrets.yml`, skip this step and note it explicitly — the hermetic `test-parse-primary` / `test-parse-fallback` workflows already cover the parsing logic; this step only validates against the live Firebase CLI.

- [ ] **Step 4: Commit**

```bash
git add bitrise.yml
git commit -m "test(bitrise.yml): assert all four outputs populated in deploy-android/ios"
```

---

## Self-review — done inline

**Spec coverage:**

| Spec section | Covered by |
|---|---|
| "Always invoke with `--debug`" | Task 5, Step 1 |
| JSON extraction via jq | Task 5, Step 3 (Primary extraction block) |
| Labeled-line fallback | Task 5, Step 3 (Fallback block) |
| Display filter honoring `is_debug` | Task 5, Step 3 (Display filter block) |
| New `FIREBASE_BITRISE_BINARY_DOWNLOAD_URL` output env var | Task 5, Step 3 (export_output calls) |
| `step.yml` output declaration | Task 6 |
| `jq` hard-fail if missing | Task 5, Step 2 |
| Warn but don't fail when both paths empty | Task 5, Step 3 (`export_output` helper) |
| Primary-path test | Task 3 (RED) → passes at Task 5, Step 4 (GREEN) |
| Fallback-path test | Task 4 (RED) → passes at Task 5, Step 5 (GREEN) |
| Real E2E verification | Task 8 |
| `CLAUDE.md` update | Task 7 |
| Preserve `is_debug` user-visible semantics | Task 5, Step 3 (display filter branch) |
| Preserve existing output env var names | Task 5, Step 3 (unchanged keys) |
| Out-of-scope `RELEASE_ID` rename | Intentionally untouched (spec explicit) |

**Placeholder scan:** No TBD/TODO in the plan. All code blocks are complete. All file paths are absolute within the repo root. All commands include expected outputs.

**Type consistency:** Variable names used consistently across tasks — `firebase_console_uri`, `testing_uri`, `binary_download_uri`, `release_id`, `release_json`, `release_name`, `output`, `submit_cmd`. The `export_output` helper is defined once in Task 5 Step 3 and invoked four times in the same step. No forward references to undefined names.
