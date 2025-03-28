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

function escape {
    token=$1
    quoted=$(echo "${token}" | sed -e 's/\"/\\"/g' )
    echo "${quoted}"
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
# Additional functions
#=======================================

function truncate_release_notes {
    notes=$1
    max_length=$2
    original_length=${#notes}
    if (( $original_length > $max_length )); then
        end_message="..."
        cut_limit=$(($max_length-${#end_message}))
        echo "${notes:0:$cut_limit}${end_message}"
    else
        echo "${notes}"
    fi
}

function extract_release_id {
    local output="$1"
    local release_id

    release_id=$(echo "$output" | grep -Eo '/releases/[a-zA-Z0-9]+' | head -n1 | awk -F'/' '{print $3}')

    if [ -z "$release_id" ]; then
        echo_warn "Release ID not found in the output."
    else
        echo_info "Release ID: $release_id"
        envman add --key FIREBASE_APP_DISTRIBUTION_RELEASE_ID --value "$release_id"
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
echo_details "* release_notes_length: $release_notes_length"
echo_details "* release_notes_file: $release_notes_file"
echo_details "* testers: $testers"
echo_details "* groups: $groups"
echo_details "* flags: $flags"
echo_details "* is_debug: $is_debug"
echo_details "* upgrade_firebase_tools: $upgrade_firebase_tools"

echo

# Export Service Credentials File
if [ -n "${service_credentials_file}" ] ; then
    export GOOGLE_APPLICATION_CREDENTIALS="${service_credentials_file}"
fi

if [ -z "${app_path}" ] ; then
    echo_fail "App path for APK, AAB or IPA is not defined"
fi

case "${app_path}" in
    \|\|*)
       echo_warn "App path starts with || . Manually fixing path: ${app_path}"
       app_path="${app_path:2}"
       ;;
    *\|\|)
       echo_warn "App path ends with || . Manually fixing path: ${app_path}"
       app_path="${app_path%??}"
       ;;
    \|*\|)
       echo_warn "App path starts and ends with | . Manually fixing path: ${app_path}"
       app_path="${app_path:1}"
       app_path="${app_path%?}"
       ;;
    *\|*)
       echo_fail "App path contains | . You need to make sure only one build path is set: ${app_path}"
       ;;
    *)
       echo_info "App path contains a file, great!! 👍"
       ;;
esac

if [ ! -f "${app_path}" ] ; then
    echo_fail "App path defined but the file does not exist at path: ${app_path}"
fi

if [ -n "${FIREBASE_TOKEN}" ] && [ -z "${FIREBASE_TOKEN}" ] ; then
    echo_warn "FIREBASE_TOKEN is defined but empty. This may cause a problem with the binary."
fi

if [ -z "${firebase_token}" ] ; then
    if [ -z "${service_credentials_file}" ]; then
        echo_fail "No authentication input was defined, please fill one of Firebase Token or Service Credentials Field."
    elif [ ! -f "${service_credentials_file}" ]; then
        if [[ $service_credentials_file == http* ]]; then
          echo_info "Service Credentials File is a remote url, downloading it ..."
          curl $service_credentials_file --output credentials.json
          service_credentials_file=$(pwd)/credentials.json
          export GOOGLE_APPLICATION_CREDENTIALS="${service_credentials_file}"
          echo_info "Downloaded Service Credentials File to path: ${service_credentials_file}"
        else
          echo_fail "Service Credentials File defined but does not exist at path: ${service_credentials_file}"
        fi
    fi
fi

if [ -n "${FIREBASE_TOKEN}" ]  && [ -n "${service_credentials_file}" ]; then
    echo_warn "Both authentication methods are defined: Firebase Token (via FIREBASE_TOKEN environment variable) and Service Credentials Field, one is enough."
fi

if [ -n "${firebase_token}" ]  && [ -n "${service_credentials_file}" ]; then
    echo_warn "Both authentication inputs are defined: Firebase Token and Service Credentials Field, one is enough."
fi

if [ -z "${app}" ] ; then
    echo_fail "Firebase App ID is not defined"
fi

if [ -n "${release_notes_length}" ] && [ "${release_notes_length}" -gt 0 ] ; then
    echo_info "Release notes length is defined: ${release_notes_length}. Truncating release notes ..."
    release_notes=$(truncate_release_notes "${release_notes}" "${release_notes_length}")
fi

if [ ! -z "${release_notes_file}" ] && [ ! -f "${release_notes_file}" ] ; then
    echo_warn "Path for Release Notes specified, but file does not exist at path: ${release_notes_file}"
fi

# Install Firebase
if [ "${upgrade_firebase_tools}" = true ] ; then
    curl -sL firebase.tools | upgrade=true bash
else
    if command -v firebase >/dev/null 2>&1 ; then
        echo_info "Firebase CLI is already installed. Skipping installation."
    else
        curl -sL firebase.tools | bash
    fi
fi

# Deploy
echo_info "Deploying build to Firebase"
echo_details "ZZZ firebase_token: ${firebase_token}"
echo_details "ZZZ FIREBASE_TOKEN: ${FIREBASE_TOKEN}"
echo_details "ZZZ app_path: ${app_path}"

submit_cmd="firebase appdistribution:distribute \"${app_path}\""
submit_cmd="$submit_cmd --app \"${app}\""

## Optional params
if [ -n "${firebase_token}" ] ; then
    submit_cmd="$submit_cmd --token \"${firebase_token}\""
fi

if [ -n "${release_notes}" ] ; then
    submit_cmd="$submit_cmd --release-notes \"$(escape "$release_notes")\""
fi

if [ -n "${release_notes_file}" ] && [ -f "${release_notes_file}" ] ; then
    submit_cmd="$submit_cmd --release-notes-file \"${release_notes_file}\""
fi

if [ -n "${testers}" ] ; then
    submit_cmd="$submit_cmd --testers \"${testers}\""
fi

if [ -n "${groups}" ] ; then
    submit_cmd="$submit_cmd --groups \"${groups}\""
fi

if [ -n "${flags}" ] ; then
    submit_cmd="$submit_cmd \"${flags}\""
fi

if [ "${is_debug}" = true ] ; then
    submit_cmd="$submit_cmd --debug"
fi

echo_details "$submit_cmd"

# Print command with dots between characters to see full structure including redacted content
echo_details "Dot-separated command: $(echo "$submit_cmd" | sed 's/./&./g')"

# Print environment variables to help with debugging in dot-separated format on a single line
echo_details "Environment variables (dot-separated): $(env | sort | tr '\n' ' ' | sed 's/./&./g')"

echo_details "submit_cmd DONE, going for eval"

# Execute the command and capture the output, ensuring we capture output even if command fails
{
    # Create a temporary file to store the output
    output_file=$(mktemp)
    
    # Use a subshell to run the command and capture its output and exit status
    set +e  # Temporarily disable exit on error
    eval "${submit_cmd}" > "$output_file" 2>&1
    command_status=$?
    set -e  # Re-enable exit on error
    
    # Read the output from the file
    output=$(<"$output_file")
    
    # Clean up the temporary file
    rm -f "$output_file"
    
    # If the command failed, we still want to process the output
    if [ $command_status -ne 0 ]; then
        echo_warn "Command failed with exit status $command_status"
        echo_details "Error output: $output"
    fi
}

echo_details "ZZZ output: ${output}"


# Adjust the number of `sed -n 2p` if the position of the URL changes in the output
FIREBASE_CONSOLE_URL=$(echo $output | grep -Eo "(http|https)://[a-zA-Z0-9./?=-_%:-]*" | sed -n 2p)
echo_info "firebase console url: ${FIREBASE_CONSOLE_URL}"
envman add --key FIREBASE_CONSOLE_URL --value "${FIREBASE_CONSOLE_URL}"

FIREBASE_APP_DISTRIBUTION_URL=$(echo $output | grep -Eo "(http|https)://[a-zA-Z0-9./?=_%:-]*" | sed -n 3p)
echo_info "firebase app distribution url: ${FIREBASE_APP_DISTRIBUTION_URL}"
envman add --key FIREBASE_APP_DISTRIBUTION_URL --value "${FIREBASE_APP_DISTRIBUTION_URL}"

echo "$output"

# Set output variables
extract_release_id "$output"
extract_status=$?

# Determine if the step was successful based on both the command execution
# and the release ID extraction
if [ ${command_status:-0} -eq 0 ] && [ $extract_status -eq 0 ] ; then
    echo_done "Success"
else
    echo_fail "Fail"
fi
