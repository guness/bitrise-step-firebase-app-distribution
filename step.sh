#!/bin/bash
set -ex

# Install Firebase
npm install -g firebase-tools

# Enable Firebase App Distribution
firebase --open-sesame appdistribution

# Export Firebase Token
export FIREBASE_TOKEN="${firebase_token}"

# Deploy
firebase appdistribution:distribute "${app_path}" \
--app "${app}" \
--release-notes "${release_notes}" \
--testers "${testers}" \
--groups "${groups}"
