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
