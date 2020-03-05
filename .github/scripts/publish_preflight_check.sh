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
    echo_warn "PREFLIGHT FAILED"
    echo_warn "--------------------------------------------"
    exit 1
}


echo_info "Starting publish preflight check..."
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


echo_info ""
echo_info "--------------------------------------------"
echo_info "Checking release tag"
echo_info "--------------------------------------------"
echo_info ""

echo_info "---< git fetch --depth=1 origin +refs/tags/*:refs/tags/* >---"
git fetch --depth=1 origin +refs/tags/*:refs/tags/*
echo ""

readonly EXISTING_TAG=`git rev-parse -q --verify "refs/tags/v${RELEASE_VERSION}"` || true
if [[ -n "${EXISTING_TAG}" ]]; then
  echo_warn "Tag v${RELEASE_VERSION} already exists. Exiting."
  echo_warn "If the tag was created in a previous unsuccessful attempt, delete it and try again."
  echo_warn "  $ git tag -d v${RELEASE_VERSION}"
  echo_warn "  $ git push --delete origin v${RELEASE_VERSION}"

  readonly RELEASE_URL="https://github.com/firebase/firebase-admin-go/releases/tag/v${RELEASE_VERSION}"
  echo_warn "Delete any corresponding releases at ${RELEASE_URL}."
  terminate
fi

echo_info "Tag v${RELEASE_VERSION} does not exist."


echo ""
echo_info "--------------------------------------------"
echo_info "PREFLIGHT SUCCESSFUL"
echo_info "--------------------------------------------"
