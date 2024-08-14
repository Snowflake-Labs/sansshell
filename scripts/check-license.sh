#!/usr/bin/env bash

# Copyright (c) 2019 Snowflake Inc. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

set -o nounset

function create_tempfile() {
    tempprefix=$(basename "$0")
    mktemp /tmp/${tempprefix}.XXXXXX
}


# check_status takes 3 args:
#
# A code to compare against
# Logfile to print on error
# Any text to output on failure (all remaining args).
function check_status {
  STATUS=$1
  shift
  LOG=$1
  shift
  FAIL=$*

  if [ "${STATUS}" != 0 ]; then
    {
      echo "FAIL ${FAIL}"
    } >&2
    if [ "${LOG}" != "/dev/null" ]; then
      ls "${LOGS}"
      print_logs "${LOG}" "${FAIL}"
    fi
    exit 1
  fi
}

license_go_files=$(create_tempfile)
license_proto_files=$(create_tempfile)

# Check licensing
# For Go we can ignore generate protobuf files.
find . -type f -name \*.go ! -name \*.pb.go >"${license_go_files}"
find . -type f -name \*.proto >"${license_proto_files}"

cat "${license_go_files}" "${license_proto_files}" | (
  broke=""
  while read -r i; do
    if ! grep -q "Licensed under the Apache License" ${i}; then
      echo "${i} is missing required license."
      broke=true
    fi
  done

  if [ "${broke}" == "true" ]; then
    exit 1
  fi
)
check_status $? /dev/null Files missing license
