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
