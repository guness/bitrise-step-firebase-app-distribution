#!/bin/bash
set -e

THIS_SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

#=======================================
# Functions
#=======================================

RESTORE='\033[0m'
RED='\033[00;31m'
YELLOW='\033[00;33m'
BLUE='\033[00;34m'
GREEN='\033[00;32m'

function color_echo {
    color=$1
    msg=$2
    echo -e "${color}${msg}${RESTORE}"
}

function echo_fail {
    msg=$1
    echo
    color_echo "${RED}" "${msg}"
    exit 1
}

function echo_warn {
    msg=$1
    color_echo "${YELLOW}" "${msg}"
}

function echo_info {
    msg=$1
    echo
    color_echo "${BLUE}" "${msg}"
}

function echo_details {
    msg=$1
    echo "  ${msg}"
}

function echo_done {
    msg=$1
    color_echo "${GREEN}" "  ${msg}"
}

function validate_required_input {
    key=$1
    value=$2
    if [ -z "${value}" ] ; then
        echo_fail "Missing required input: ${key}"
    fi
}

function validate_required_input_with_options {
    key=$1
    value=$2
    options=$3

    validate_required_input "${key}" "${value}"

    found="0"
    for option in "${options[@]}" ; do
        if [ "${option}" == "${value}" ] ; then
            found="1"
        fi
    done

    if [ "${found}" == "0" ] ; then
        echo_fail "Invalid input: (${key}) value: (${value}), valid options: ($( IFS=$", "; echo "${options[*]}" ))"
    fi
}

#=======================================
# Main
#=======================================

#
# Validate parameters
echo_info "Configs:"
echo_details "* firebase_token: $firebase_token"
echo_details "* service_credentials_file: $service_credentials_file"
echo_details "* app_path: $app_path"
echo_details "* app: $app"
echo_details "* release_notes: $release_notes"
echo_details "* release_notes_file: $release_notes_file"
echo_details "* testers: $testers"
echo_details "* groups: $groups"
echo_details "* flags: $flags"
echo_details "* is_debug: $is_debug"
echo_details "* upgrade_firebase_tools: $upgrade_firebase_tools"

echo

if [ -z "${app_path}" ] ; then
    echo_fail "App path for APK or IPA is not defined"
fi

case "${app_path}" in
   \|*)
       echo_warn "App path starts with | . Manually fixing path: ${app_path}"
       app_path="${app_path:1}"
       ;;
    *\|)
       echo_warn "App path ends with | . Manually fixing path: ${app_path}"
       app_path="${app_path%?}"
       ;;
    *\|*)
       echo_fail "App path contains | . You need to choose one build path: ${app_path}"
       ;;
    *)
       echo_info "App path contains one file 👍"
       ;;
esac

if [ ! -f "${app_path}" ] ; then
    echo_fail "App path defined but the file does not exist at path: ${app_path}"
fi

if [ -z "${firebase_token}" ] ; then
    if [ -z "${service_credentials_file}" ]; then
        echo_fail "No authentication input was defined, please fill one of Firebase Token or Service Credentials Field."
    elif [ ! -f "${service_credentials_file}" ]; then
        echo_fail "Service Credentials File defined but does not exist at path: ${service_credentials_file}"
    fi
fi

if [ -n "${firebase_token}" ]  && [ -n "${service_credentials_file}" ]; then
    echo_info "Both authentication inputs are defined: Firebase Token and Service Credentials Field, one is enough."
fi

if [ -z "${app}" ] ; then
    echo_fail "Firebase App ID is not defined"
fi

if [ ! -z "${release_notes_file}" ] && [ ! -f "${release_notes_file}" ] ; then
    echo_warn "Path for Release Notes specified, but file does not exist at path: ${release_notes_file}"
fi

# Install Firebase
if [ "${upgrade_firebase_tools}" = true ] ; then
    curl -sL firebase.tools | upgrade=true bash
else
    curl -sL firebase.tools | bash
fi

# Export Firebase Token
if [ -n "${firebase_token}" ] ; then
    export FIREBASE_TOKEN="${firebase_token}"
fi

# Export Service Credentials File
if [ -n "${service_credentials_file}" ] ; then
    export GOOGLE_APPLICATION_CREDENTIALS="${service_credentials_file}"
fi

# Deploy
echo_info "Deploying build to Firebase"

submit_cmd="firebase appdistribution:distribute '${app_path}'"
submit_cmd="$submit_cmd --app \"${app}\""

## Optional params
if [ -n "${release_notes}" ] ; then
    submit_cmd="$submit_cmd --release-notes ${release_notes}"
fi

if [ -n "${release_notes_file}" ] && [ -f "${release_notes_file}" ] ; then
    submit_cmd="$submit_cmd --release-notes-file '${release_notes_file}'"
fi

if [ -n "${testers}" ] ; then
    submit_cmd="$submit_cmd --testers '${testers}'"
fi

if [ -n "${groups}" ] ; then
    submit_cmd="$submit_cmd --groups '${groups}'"
fi

if [ -n "${flags}" ] ; then
    submit_cmd="$submit_cmd '${flags}'"
fi

if [ "${is_debug}" = true ] ; then
    submit_cmd="$submit_cmd --debug"
fi

echo_details "$submit_cmd"
echo

${submit_cmd}

if [ $? -eq 0 ] ; then
    echo_done "Success"
else
    echo_fail "Fail"
fi
