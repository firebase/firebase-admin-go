
#!/bin/bash

# Copyright 2020 Google Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


###################################### Outputs #####################################

# 1. version: The version of this release including the 'v' prefix (e.g. v1.2.3).
# 2. changelog: Formatted changelog text for this release.

####################################################################################

set -e
set -u

function echo_info() {
    local MESSAGE=$1
    echo "[INFO] ${MESSAGE}"
}

function echo_warn() {
    local MESSAGE=$1
    echo "[WARN] ${MESSAGE}"
}

function terminate() {
    echo ""
    echo_warn "--------------------------------------------"
    echo_warn "POST CHECK FAILED"
    echo_warn "--------------------------------------------"
    exit 1
}


echo_info "Starting publish post check..."
echo_info "Git revision          : ${GITHUB_SHA}"
echo_info "Git ref               : ${GITHUB_REF}"
echo_info "Workflow triggered by : ${GITHUB_ACTOR}"
echo_info "GitHub event          : ${GITHUB_EVENT_NAME}"


echo_info ""
echo_info "--------------------------------------------"
echo_info "Extracting release version"
echo_info "--------------------------------------------"
echo_info ""

echo_info "Loading version from: firebase.go"

readonly RELEASE_VERSION=`grep "const Version" firebase.go | awk '{print $4}' | tr -d \"` || true
if [[ -z "${RELEASE_VERSION}" ]]; then
  echo_warn "Failed to extract release version from: firebase.go"
  terminate
fi

if [[ ! "${RELEASE_VERSION}" =~ ^([0-9]*)\.([0-9]*)\.([0-9]*)$ ]]; then
  echo_warn "Malformed release version string: ${RELEASE_VERSION}. Exiting."
  terminate
fi

echo_info "Extracted release version: ${RELEASE_VERSION}"
echo "::set-output name=version::v${RELEASE_VERSION}"


echo_info ""
echo_info "--------------------------------------------"
echo_info "Generating changelog"
echo_info "--------------------------------------------"
echo_info ""

echo_info "---< git fetch origin master --prune --unshallow >---"
git fetch origin master --prune --unshallow
echo ""

echo_info "Generating changelog from history..."
readonly CURRENT_DIR=$(dirname "$0")
readonly CHANGELOG=`${CURRENT_DIR}/generate_changelog.sh`
echo "$CHANGELOG"

# Parse and preformat the text to handle multi-line output.
# See https://github.community/t5/GitHub-Actions/set-output-Truncates-Multiline-Strings/td-p/37870
FILTERED_CHANGELOG=`echo "$CHANGELOG" | grep -v "\\[INFO\\]"`
FILTERED_CHANGELOG="${FILTERED_CHANGELOG//'%'/'%25'}"
FILTERED_CHANGELOG="${FILTERED_CHANGELOG//$'\n'/'%0A'}"
FILTERED_CHANGELOG="${FILTERED_CHANGELOG//$'\r'/'%0D'}"
echo "::set-output name=changelog::${FILTERED_CHANGELOG}"


echo ""
echo_info "--------------------------------------------"
echo_info "POST CHECK SUCCESSFUL"
echo_info "--------------------------------------------"
