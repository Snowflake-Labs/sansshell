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

  if [ $STATUS != 0 ]; then
    {
      echo "FAIL ${FAIL}"
      echo "Logs in ${LOGS}"
    } >&2
    if [ "${LOG}" != "/dev/null" ]; then
      ls ${LOGS}
      print_logs ${LOG} ${FAIL}
    fi
    if [ -f ${LOGS}/proxy.log ]; then
      print_logs ${LOGS}/proxy.log proxy log
    fi
    if [ -f ${LOGS}/server.log ]; then
      print_logs ${LOGS}/server.log server log
    fi
    exit 1
  fi
}

function shutdown {
  echo "Shutting down"
  echo "Logs in ${LOGS}"
  if [ -n "${PROXY_PID}" ]; then
    kill -KILL ${PROXY_PID}
  fi
  # Skip if on github
  if [ -z "${ON_GITHUB}" ]; then
    aws s3 rm s3://${USER}-dev/hosts
  fi
  sudo killall sansshell-server >&/dev/null
}

# check_logs takes 3 args:
#
# Minimum number of lines log must contain
# The log suffix
# Text to display on error (all remaining args).
function check_logs {
  # The minimum number of lines a log file needs to have.
  # Not going to check exact content. If they are within
  # the same length and long enough and non-zero exit that's ok.
  LINE_MIN=$1
  shift
  SUFFIX=$1
  shift
  FAIL=$*

  wc -l ${LOGS}/1.${SUFFIX} ${LOGS}/2.${SUFFIX} | awk "{
    line1=\$1
    readline
    line2=\$1
    # They should be within an entry of each other and reasonable number of processes.
    if ((line1 != line2 && line1+1 != line2 && line1 != line2+2) || line1 < ${LINE_MIN}) {
      exit 1
    }
  }"
  if [ $? != 0 ]; then
    print_logs ${LOGS}/1.${SUFFIX} log 1
    print_logs ${LOGS}/2.${SUFFIX} log 2
    check_status 1 /dev/null logs mismatch or too short ${LOGS}/1.${SUFFIX} ${LOGS}/2.${SUFFIX} - ${FAIL}
  fi
}

function copy_logs {
  SUFFIX=$1
  shift
  PURPOSE=$1

  mv ${LOGS}/1.${SUFFIX} ${LOGS}/1.${SUFFIX}-${PURPOSE}
  if [ -f "${LOGS}/2.${SUFFIX}" ]; then
    mv ${LOGS}/2.${SUFFIX} ${LOGS}/2.${SUFFIX}-${PURPOSE}
  fi
}

function print_logs {
  LOG=$1
  shift
  PREFACE=$*

  printf "\n${PREFACE}:\n\n"
  cat ${LOG}
}

# run_a_test takes 4 args:
#
# A boolean (i.e. true or false string) indicating one run for 2 hosts can error.
# This can happen for ptrace related commands since 2 cannot act on the same process at once.
# Minimum number of lines log must contain
# The top level command to pass to the sanssh CLI.
# The subcommand to pass to the sanssh CLI.  NOTE: This is also used as the logfile suffix along with command.
# Args to pass to sanssh after the command (all remaining args)
function run_a_test {
  ONE_WILL_ERROR=$1
  shift
  LINE_MIN=$1
  shift
  CMD=$1
  shift
  SUBCMD=$1
  shift
  ARGS=$*

  echo "${CMD} ${SUBCMD} checks"

  CHECK="${CMD} ${SUBCMD} proxy to 2 hosts"
  echo ${CHECK}
  ${SANSSH_PROXY} ${MULTI_TARGETS} --outputs=${LOGS}/1.${CMD}-${SUBCMD},${LOGS}/2.${CMD}-${SUBCMD} ${CMD} ${SUBCMD} ${ARGS}
  STATUS=$?
  if [ "${ONE_WILL_ERROR}" = "false" ]; then
    if [ "${STATUS}" != 0 ]; then
      print_logs ${LOGS}/1.${CMD}-${SUBCMD} log 1
      print_logs ${LOGS}/2.${CMD}-${SUBCMD} log 2
      check_status ${STATUS} /dev/null ${CHECK}
    fi
  else
    # If one of these is expected to error then one log is larger than the other.
    # Take the larger file and copy it so log checking will pass.
    SIZE1=$(du -b ${LOGS}/1.${CMD}-${SUBCMD} | cut -f1)
    SIZE2=$(du -b ${LOGS}/2.${CMD}-${SUBCMD} | cut -f1)
    if ((${SIZE1} == ${SIZE2})); then
      echo "FAIL - logs identical $*"
      print_logs ${LOGS}/1.${CMD}-${SUBCMD} log 1
      print_logs ${LOGS}/2.${CMD}-${SUBCMD} log 2
      check_status 1 /dev/null logs identical
    fi
    if ((${SIZE1} > ${SIZE2})); then
      cp ${LOGS}/1.${CMD}-${SUBCMD} ${LOGS}/2.${CMD}-${SUBCMD}
    else
      cp ${LOGS}/2.${CMD}-${SUBCMD} ${LOGS}/1.${CMD}-${SUBCMD}
    fi
  fi
  copy_logs ${CMD}-${SUBCMD} proxy-2-hosts

  CHECK="${CMD} ${SUBCMD} proxy to 1 host"
  echo ${CHECK}
  ${SANSSH_PROXY} ${SINGLE_TARGET} --outputs=${LOGS}/1.${CMD}-${SUBCMD} ${CMD} ${SUBCMD} ${ARGS}
  check_status $? ${LOGS}/1.${CMD}-${SUBCMD} ${CHECK}
  # This way they pass the identical test
  cp ${LOGS}/1.${CMD}-${SUBCMD} ${LOGS}/2.${CMD}-${SUBCMD}
  check_logs ${LINE_MIN} ${CMD}-${SUBCMD} ${CHECK}
  copy_logs ${CMD}-${SUBCMD} proxy-1-host

  CHECK="${CMD} ${SUBCMD} with no proxy"
  echo ${CHECK}
  ${SANSSH_NOPROXY} ${SINGLE_TARGET} --outputs=${LOGS}/1.${CMD}-${SUBCMD} ${CMD} ${SUBCMD} ${ARGS}
  check_status $? ${LOGS}/1.${CMD}-${SUBCMD} ${CHECK}
  # This way they pass the identical test
  cp ${LOGS}/1.${CMD}-${SUBCMD} ${LOGS}/2.${CMD}-${SUBCMD}
  check_logs ${LINE_MIN} ${CMD}-${SUBCMD} ${CHECK}
  copy_logs ${CMD}-${SUBCMD} no-proxy

  echo "${CMD} ${SUBCMD} passed"
}

# Takes 1 arg:
#
# Pid of sanssh process
function tail_execute {
  CLIENT_PID=$1
  shift
  sleep 2
  cat /etc/hosts >>${LOGS}/hosts
  sleep 2
  disown %%
  kill -KILL ${CLIENT_PID}
}

# Takes 1 arg:
#
# Suffix for copied log files
function tail_check {
  diff ${LOGS}/1.tail ${LOGS}/hosts >${LOGS}/tail.diff
  check_status $? ${LOGS}/tail.diff "tail: output differs from source"
  copy_logs tail $*
}

# Takes 1 arg:
#
# File to check
#
# Assumes EXPECTED_* are already set.
function check_perms_mode {
  FILE=$1
  shift

  read NEW_MODE NEW_UID NEW_GID < <(stat -c '0%a %u %g' $FILE)
  NEW_IMMUTABLE=$(lsattr ${FILE} | cut -c5)

  if [ "$NEW_MODE" != "$EXPECTED_NEW_MODE" ]; then
    check_status 1 /dev/null "modes for $FILE not as expected. Have $NEW_MODE but expected $EXPECTED_NEW_MODE"
  fi
  # These aren't quoted as parsed ones may have leading spaces
  if [ $NEW_UID != $EXPECTED_NEW_UID ]; then
    check_status 1 /dev/null "uid for $FILE not as expected. Have $NEW_UID but expected $EXPECTED_NEW_UID"
  fi
  if [ $NEW_GID != $EXPECTED_NEW_GID ]; then
    check_status 1 /dev/null "gid for $FILE not as expected. Have $NEW_GID but expected $EXPECTED_NEW_GID"
  fi
  if [ "$NEW_IMMUTABLE" != "$EXPECTED_NEW_IMMUTABLE" ]; then
    check_status 1 /dev/null "imutable for $FILE not as expected. Have $NEW_IMMUTABLE but expected $EXPECTED_NEW_IMMUTABLE"
  fi
}

PROXY_PID=""
ON_GITHUB=""

if [ -n "${GITHUB_ACTION:-}" ]; then
  ON_GITHUB="true"
  # So we can run pstack/gcore below
  sudo sysctl kernel.yama.ptrace_scope
  sudo sysctl -w kernel.yama.ptrace_scope=0
  sudo sysctl kernel.yama.ptrace_scope
  sudo cp testing/gdb-pstack /usr/bin/pstack
  sudo chmod +x /usr/bin/pstack

  # Tests expect a nobody group to exist
  sudo groupadd nobody
  sudo usermod -g nobody nobody
  cat /etc/passwd
fi

OS=$(uname -s)
if [ "${OS}" != "Linux" ]; then
  echo "Integration testing only works on linux"
  exit 1
fi

trap shutdown EXIT INT TERM HUP

LOGS=/tmp/test-logs-$$
mkdir -p ${LOGS}

# Make sure we're at the top level which is one below where this script lives
cd $(dirname $PWD/$BASH_SOURCE)/..

# Open up policy for testing
echo "package sansshell.authz" >${LOGS}/policy
echo "default allow = true" >>${LOGS}/policy

# Check licensing
cat >${LOGS}/required-license <<EOF
/* Copyright (c) 2019 Snowflake Inc. All rights reserved.

   Licensed under the Apache License, Version 2.0 (the
   "License"); you may not use this file except in compliance
   with the License.  You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing,
   software distributed under the License is distributed on an
   "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
   KIND, either express or implied.  See the License for the
   specific language governing permissions and limitations
   under the License.
*/

EOF
cat >${LOGS}/required-license.sh <<EOF
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

EOF

# For Go we can ignore generate protobuf files.
find . -type f -name \*.go ! -name \*.pb.go >${LOGS}/license-go
find . -type f -name \*.sh >${LOGS}/license-sh
find . -type f -name \*.proto >${LOGS}/license-proto

broke=""
for i in $(cat ${LOGS}/license-go ${LOGS}/license-proto); do
  LIC="${LOGS}/required-license"
  SUF=${i##*.}
  if [ "${SUF}" == "go" -o "${SUF}" == "proto" ]; then
    head -16 $i >${LOGS}/diff

    # If this has a go directive it has to be the first line
    # so skip forward to where the block should be right after that.
    one=$(head -1 ${LOGS}/diff | cut -c1-5)
    if [ "${one}" == "//go:" ]; then
      printf "4,19p\n" | ed -s $i >${LOGS}/diff 2>/dev/null
    fi
  fi
  if [ "${SUF}" == "sh" ]; then
    printf "3,17p\n" | ed -s $i >${LOGS}/diff 2>/dev/null
    LIC="${LOGS}/required-license.sh"
  fi
  diff ${LIC} ${LOGS}/diff >/dev/null
  if [ $? != 0 ]; then
    echo $i is missing required license.
    broke=true
  fi
done

if [ "${broke}" == "true" ]; then
  check_status 1 /dev/null Files missing license
fi

echo
echo "Checking formatting"
echo
find . -type f -name \*.go | xargs gofmt -l >${LOGS}/gofmt
check_status $? /dev/null gofmt
if [ -s ${LOGS}/gofmt ]; then
  cat ${LOGS}/gofmt
  echo
  check_status 1 /dev/null "Files listed need gofmt run on them"
fi

echo
echo "Checking with go vet"
echo
go vet ./...
check_status $? /dev/null go vet

# Build everything (this won't rebuild the binaries but the generate will)
echo
echo "Running builds"
echo
go build -v ./...
check_status $? /dev/null build
go generate build.go
check_status $? /dev/null build binaries

# Test everything
echo
echo "Running tests (with tsan)"
echo
go test -count=10 -race -timeout 5m -v ./... >&${LOGS}/test.log
check_status $? ${LOGS}/test.log test

echo
echo "Checking coverage - logs in ${LOGS}/cover.log"
echo
go test -timeout 30s -v -coverprofile=/tmp/go-code-cover ./... >&${LOGS}/cover.log
check_status $? ${LOGS}/cover.log coverage

# Print out coverage stats
egrep "^ok.*coverage:.*of.statements|no test files" ${LOGS}/cover.log >${LOGS}/cover-filtered.log
echo
echo
echo
egrep "coverage:.*of.statements" ${LOGS}/cover-filtered.log

# There are a bunch of directories where having no tests is fine.
# They are either binaries, top level package directories or
# testing code.
for i in \
  github.com/Snowflake-Labs/sansshell/auth/mtls/flags \
  github.com/Snowflake-Labs/sansshell/client \
  github.com/Snowflake-Labs/sansshell/cmd \
  github.com/Snowflake-Labs/sansshell/cmd/proxy-server \
  github.com/Snowflake-Labs/sansshell/cmd/proxy-server/server \
  github.com/Snowflake-Labs/sansshell/cmd/sanssh \
  github.com/Snowflake-Labs/sansshell/cmd/sanssh/client \
  github.com/Snowflake-Labs/sansshell/cmd/sansshell-server \
  github.com/Snowflake-Labs/sansshell/cmd/sansshell-server/server \
  github.com/Snowflake-Labs/sansshell/cmd/util \
  github.com/Snowflake-Labs/sansshell/proxy \
  github.com/Snowflake-Labs/sansshell/proxy/protoc-gen-go-grpcproxy \
  github.com/Snowflake-Labs/sansshell/proxy/testutil \
  github.com/Snowflake-Labs/sansshell/services \
  github.com/Snowflake-Labs/sansshell/services/ansible \
  github.com/Snowflake-Labs/sansshell/services/ansible/client \
  github.com/Snowflake-Labs/sansshell/services/exec \
  github.com/Snowflake-Labs/sansshell/services/exec/client \
  github.com/Snowflake-Labs/sansshell/services/healthcheck \
  github.com/Snowflake-Labs/sansshell/services/healthcheck/client \
  github.com/Snowflake-Labs/sansshell/services/localfile \
  github.com/Snowflake-Labs/sansshell/services/localfile/client \
  github.com/Snowflake-Labs/sansshell/services/packages \
  github.com/Snowflake-Labs/sansshell/services/packages/client \
  github.com/Snowflake-Labs/sansshell/services/process \
  github.com/Snowflake-Labs/sansshell/services/process/client \
  github.com/Snowflake-Labs/sansshell/services/sansshell \
  github.com/Snowflake-Labs/sansshell/services/sansshell/client \
  github.com/Snowflake-Labs/sansshell/services/service \
  github.com/Snowflake-Labs/sansshell/services/service/client \
  github.com/Snowflake-Labs/sansshell/testing/testutil; do

  egrep -E -v "$i[[:space:]]+.no test file" ${LOGS}/cover-filtered.log >${LOGS}/cover-filtered2.log
  cp ${LOGS}/cover-filtered2.log ${LOGS}/cover-filtered.log
done
echo
egrep "no test files" ${LOGS}/cover-filtered.log
echo

# Abort if either required packaged have no tests or coverage is below 85%
# for any package.
abort_tests=""
egrep 'no test files' ${LOGS}/cover-filtered.log
if [ $? == 0 ]; then
  abort_tests="Packages with no tests"
fi
echo

bad=false
oIFS=${IFS}
IFS="
"
for i in $(egrep % ${LOGS}/cover-filtered.log); do
  percent=$(echo $i | sed -e "s,.*\(coverage:.*\),\1," | awk '{print $2}' | sed -e 's:\%::')
  # Have to use bc as expr can't handle floats...
  if [ $(printf "$percent < 90.0\n" | bc) == "1" ]; then
    echo "Coverage not high enough"
    echo $i
    echo
    bad=true
  fi
done
IFS=${oIFS}

if [ "$bad" == "true" ]; then
  abort_tests="${abort_tests}  Packages with coverage too low"
fi
if [ -n "${abort_tests}" ]; then
  check_status 1 /dev/null "${abort_tests}"
fi

# Skip if on github
if [ -z "${ON_GITHUB}" ]; then
  # Remove zziplib so we can reinstall it
  echo "Removing zziplib package so install can put it back"
  sudo yum remove -y zziplib
else
  echo "Skipping package setup on Github"
fi

echo
echo "Testing policy validation for proxy"
./bin/proxy-server --policy-file=${LOGS}/policy --validate
check_status $? /dev/null policy check failed for proxy
echo
echo "Testing policy validation for server"
./bin/sansshell-server --policy-file=${LOGS}/policy --validate
check_status $? /dev/null policy check failed for server

echo
echo "Starting servers. Logs in ${LOGS}"
./bin/proxy-server --justification --root-ca=./auth/mtls/testdata/root.pem --server-cert=./auth/mtls/testdata/leaf.pem --server-key=./auth/mtls/testdata/leaf.key --client-cert=./auth/mtls/testdata/client.pem --client-key=./auth/mtls/testdata/client.key --policy-file=${LOGS}/policy >&${LOGS}/proxy.log &
PROXY_PID=$!
# Since we're controlling lifetime the shell can ignore this (avoids useless termination messages).
disown %%

# The server needs to be root in order for package installation tests (and the nodes run this as root).
sudo --preserve-env=AWS_ACCESS_KEY_ID,AWS_SECRET_ACCESS_KEY -b ./bin/sansshell-server --justification --root-ca=./auth/mtls/testdata/root.pem --server-cert=./auth/mtls/testdata/leaf.pem --server-key=./auth/mtls/testdata/leaf.key --policy-file=${LOGS}/policy >&${LOGS}/server.log

# Skip if on github
if [ -z "${ON_GITHUB}" ]; then
  # Make sure remote cloud works.
  # TODO(jchacon): Plumb a test account in via a flag instead of assuming this works.

  # Check if the bucket exists and is in the right region. If it doesn't exist we'll
  # make it but otherwise abort.
  aws s3api get-bucket-location --bucket=${USER}-dev >${LOGS}/s3.data 2>/dev/null
  RC=$?
  if [ "$RC" = 0 ]; then
    if [ "$(egrep LocationConstraint ${LOGS}/s3.data | awk '{print $2}')" != '"us-west-2"' ]; then
      check_status 1 /dev/null Bucket ${USER}-dev not in us-west-2. Fix manually and run again.
    fi
  else
    aws s3 mb s3://${USER}-dev --region us-west-2
    check_status $? /dev/null Making s3 bucket
  fi
else
  echo "Skipping remote cloud setup on Github"
fi

SANSSH_NOPROXY_NO_JUSTIFY="./bin/sanssh --root-ca=./auth/mtls/testdata/root.pem --client-cert=./auth/mtls/testdata/client.pem --client-key=./auth/mtls/testdata/client.key --timeout=120s"
SANSSH_NOPROXY="${SANSSH_NOPROXY_NO_JUSTIFY} --justification=yes"
SANSSH_PROXY_NOPORT="${SANSSH_NOPROXY} --proxy=localhost"
SANSSH_PROXY="${SANSSH_PROXY_NOPORT}:50043"
SINGLE_TARGET_NOPORT="--targets=localhost"
SINGLE_TARGET="${SINGLE_TARGET_NOPORT}:50042"
MULTI_TARGETS="--targets=localhost,localhost:50042"

# The first test is server health which also means we're validating
# things came up ok.
echo "Checking server health"
HEALTHY=false
for i in $(seq 1 100); do
  echo "${i}:"
  ${SANSSH_PROXY} ${MULTI_TARGETS} healthcheck validate
  if [ $? = 0 ]; then
    HEALTHY=true
    break
  fi
done
if [ "${HEALTHY}" != "true" ]; then
  echo "Servers never healthy"
  printf "\nServer:\n\n"
  cat ${LOGS}/server.log
  printf "\nProxy:\n\n"
  cat ${LOGS}/proxy.log
  exit 1
fi
echo "Servers healthy"

# Now a simple test to validate justification requirements are working
# and are passing along from proxy->clients
echo "Expect a failure here about a lack of justification"
${SANSSH_NOPROXY_NO_JUSTIFY} --proxy=localhost:50043 ${MULTI_TARGETS} healthcheck validate
if [ $? != 1 ]; then
  check_status 1 /dev/null missing justification failed
fi

cat >${LOGS}/client-policy.rego <<EOF
package sansshell.authz

default allow = false

allow {
  some i
  input.peer.cert.subject.organization[i] = "foo"
}
EOF
${SANSSH_PROXY} ${SINGLE_TARGET} --v=1 --client-policy-file=${LOGS}/client-policy.rego healthcheck validate
if [ $? != 1 ]; then
  check_status 1 /dev/null policy check did not fail
fi

# Now use the real test cert org
cat >${LOGS}/client-policy.rego <<EOF
package sansshell.authz

default allow = false

allow {
  some i
  input.peer.cert.subject.Organization[i] = "Acme Co"
}
EOF
${SANSSH_PROXY} ${SINGLE_TARGET} --v=1 --client-policy-file=${LOGS}/client-policy.rego healthcheck validate
check_status $? policy check should succeed

${SANSSH_PROXY_NOPORT} ${SINGLE_TARGET_NOPORT} healthcheck validate
check_status $? default ports have been appended

# Now set logging to v=1 and validate we saw that in the logs
echo "Setting logging level higher"
${SANSSH_PROXY} ${MULTI_TARGETS} sansshell set-verbosity --verbosity=1
egrep -q -e '"msg"="set-verbosity".*"new level"=1 "old level"=0' ${LOGS}/server.log
check_status $? /dev/null cant find log entry for changing levels

echo "Setting proxy logging level higher"
${SANSSH_PROXY} sansshell set-proxy-verbosity --verbosity=1
egrep -q -e '"msg"="set-verbosity".*"new level"=1 "old level"=0' ${LOGS}/proxy.log
check_status $? /dev/null cant find log entry in proxy for changing levels

${SANSSH_PROXY} sansshell get-proxy-verbosity
check_status $? /dev/null cant get proxy verbosity

run_a_test false 1 sansshell get-verbosity

run_a_test false 50 ansible playbook --playbook=$PWD/services/ansible/server/testdata/test.yml --vars=path=/tmp,path2=/

run_a_test false 1 exec run /usr/bin/echo Hello World

# Test that outputs/errors go where expected and --output-dir works.
mkdir -p ${LOGS}/exec-testdir
${SANSSH_PROXY} ${MULTI_TARGETS} --output-dir=${LOGS}/exec-testdir exec run /bin/sh -c "echo foo >&2"
check_status $? /dev/null exec failed emitting to stderr
if [ -s ${LOGS}/exec-testdir/0 ] || [ -s ${LOGS}/exec-testdir/1 ]; then
  check_status 1 /dev/null exec output appeared in normal output files in ${LOGS}/exec-testdir
fi
if [ ! -s ${LOGS}/exec-testdir/0.error ] || [ ! -s ${LOGS}/exec-testdir/1.error ]; then
  check_status 1 /dev/null exec error output did not appear in ${LOGS}/exec-testdir error files
fi

run_a_test false 5 file read /etc/hosts

# Tail is harder since we have to run the client in the background and then kill it
cp /etc/hosts ${LOGS}/hosts

echo "tail checks"

echo "tail proxy to 2 hosts"
${SANSSH_PROXY} ${MULTI_TARGETS} --outputs=${LOGS}/1.tail,${LOGS}/2.tail file tail ${LOGS}/hosts &
tail_execute $!
diff ${LOGS}/1.tail ${LOGS}/2.tail >${LOGS}/tail.diff
check_status $? ${LOGS}/tail.diff "tail: output files differ"
tail_check proxy-2-hosts

echo "tail proxy to 1 hosts"
${SANSSH_PROXY} ${SINGLE_TARGET} --outputs=${LOGS}/1.tail file tail ${LOGS}/hosts &
tail_execute $!
tail_check proxy-1-host

echo "tail with no proxy"
${SANSSH_NOPROXY} ${SINGLE_TARGET} --outputs=${LOGS}/1.tail file tail ${LOGS}/hosts &
tail_execute $!
tail_check no-proxy

echo "tail passed"

# Validate immutable with stat
sudo chattr +i ${LOGS}/hosts

run_a_test false 1 file stat ${LOGS}/hosts
sudo chattr -i ${LOGS}/hosts

IMMUTABLE_COUNT=$(egrep Immutable..true ${LOGS}/1.file-stat* | wc -l)
EXPECTED_IMMUTABLE=3
if [ "${IMMUTABLE_COUNT}" != "${EXPECTED_IMMUTABLE}" ]; then
  false
fi
check_status $? /dev/null "Immutable count incorrect in ${LOGS}/1.stat* expected ${EXPECTED_IMMUTABLE} entries to be immutable"
echo "stat immutable state validated"

run_a_test false 1 file sum /etc/hosts

# Record original state before we change it all with chown/etc
touch ${LOGS}/test-file
read ORIG_MODE ORIG_UID ORIG_GID < <(stat -c '0%a %u %g' ${LOGS}/test-file)
ORIG_IMMUTABLE=$(lsattr ${LOGS}/test-file | cut -c5)

EXPECTED_NEW_UID=$(($ORIG_UID + 1))
EXPECTED_NEW_GID=$(($ORIG_GID + 1))
EXPECTED_NEW_IMMUTABLE="i"

# Determine the new mode but since it's in octal need help
# adding this in the shell as we want to pass it in octal
# to the chmod below.
CUR=$(printf "%d\n" ${ORIG_MODE})
NEW=$(expr ${CUR} + 1)
EXPECTED_NEW_MODE=$(printf "0%o" ${NEW})

run_a_test false 0 file chown --uid=${EXPECTED_NEW_UID} ${LOGS}/test-file
run_a_test false 0 file chgrp --gid=${EXPECTED_NEW_GID} ${LOGS}/test-file
run_a_test false 0 file chmod --mode=${EXPECTED_NEW_MODE} ${LOGS}/test-file
run_a_test false 0 file immutable --state=true ${LOGS}/test-file

check_perms_mode ${LOGS}/test-file

# Now do it with username/group args
NOBODY_UID=$(id nobody | awk '{print $1}' | sed -e 's:uid=\([0-9][0-9]*\).*:\1:')
NOBODY_GID=$(id nobody | awk '{print $2}' | sed -e 's:gid=\([0-9][0-9]*\).*:\1:')

EXPECTED_NEW_UID=${NOBODY_UID}
EXPECTED_NEW_GID=${NOBODY_GID}

# Need to make this non-immutable again or we can't change the owner.
run_a_test false 0 file immutable --state=false ${LOGS}/test-file

run_a_test false 0 file chown --username=nobody ${LOGS}/test-file
run_a_test false 0 file chgrp --group=nobody ${LOGS}/test-file
run_a_test false 0 file immutable --state=true ${LOGS}/test-file
check_perms_mode ${LOGS}/test-file

# Now validate we can clear immutable too
${SANSSH_NOPROXY} ${SINGLE_TARGET} file immutable --state=false ${LOGS}/test-file
check_status $? /dev/null "setting immutable to false"
EXPECTED_NEW_IMMUTABLE="-"
NEW_IMMUTABLE=$(lsattr ${LOGS}/test-file | cut -c5)
if [ "$NEW_IMMUTABLE" != "$EXPECTED_NEW_IMMUTABLE" ]; then
  check_status 1 /dev/null "immutable not as expected. Have $NEW_IMMUTABLE but expected $EXPECTED_NEW_IMMUTABLE"
fi

echo "uid, etc checks passed"

EXPECTED_NEW_UID=$(($ORIG_UID + 1))
EXPECTED_NEW_GID=$(($ORIG_GID + 1))
run_a_test false 0 file cp --overwrite --uid=${EXPECTED_NEW_UID} --gid=${EXPECTED_NEW_GID} --mode=${EXPECTED_NEW_MODE} ${LOGS}/hosts ${LOGS}/cp-hosts
check_perms_mode ${LOGS}/cp-hosts

# Now do it with username/group
EXPECTED_NEW_UID=${NOBODY_UID}
EXPECTED_NEW_GID=${NOBODY_GID}
run_a_test false 0 file cp --overwrite --username=nobody --group=nobody --mode=${EXPECTED_NEW_MODE} ${LOGS}/hosts ${LOGS}/cp-hosts
check_perms_mode ${LOGS}/cp-hosts

# Skip if on github
if [ -z "${ON_GITHUB}" ]; then
  echo cp test with bucket syntax
  aws s3 cp ${LOGS}/hosts s3://${USER}-dev/hosts
  run_a_test false 0 file cp --overwrite --uid=${EXPECTED_NEW_UID} --gid=${EXPECTED_NEW_GID} --mode=${EXPECTED_NEW_MODE} --bucket=s3://${USER}-dev?region=us-west-2 hosts ${LOGS}/cp-hosts
  check_perms_mode ${LOGS}/cp-hosts
  aws s3 rm s3://${USER}-dev/hosts
else
  echo "Skipping cp with s3 on Github"
fi

# Trying without --overwrite to validate
echo "This can emit an error message about overwrite"
${SANSSH_NOPROXY} ${SINGLE_TARGET} --outputs=${LOGS}/1.cp-no-overwrite file cp --uid=${EXPECTED_NEW_UID} --gid=${EXPECTED_NEW_GID} --mode=${EXPECTED_NEW_MODE} ${LOGS}/hosts ${LOGS}/cp-hosts
if [ $? == 0 ]; then
  check_status 1 ${LOGS}/1.cp-no-overwrite Expected failure for no --overwrite from cp for existing file.
fi

# Make sure we can set immutable
echo "cp test immutable/overwrite"
${SANSSH_NOPROXY} ${SINGLE_TARGET} --outputs=${LOGS}/1.cp-overwrite-immutable file cp --overwrite --immutable --uid=${EXPECTED_NEW_UID} --gid=${EXPECTED_NEW_GID} --mode=${EXPECTED_NEW_MODE} ${LOGS}/hosts ${LOGS}/cp-hosts
check_status $? ${LOGS}/1.cp-overwrite-immutable cp run with --overwrite and --immutable
EXPECTED_NEW_IMMUTABLE="i"
NEW_IMMUTABLE=$(lsattr ${LOGS}/cp-hosts | cut -c5)
if [ "$NEW_IMMUTABLE" != "$EXPECTED_NEW_IMMUTABLE" ]; then
  check_status 1 /dev/null "immutable not as expected. Have $NEW_IMMUTABLE but expected $EXPECTED_NEW_IMMUTABLE"
fi
# Cleanup the file so we aren't leaving immutable files laying around
sudo chattr -i ${LOGS}/cp-hosts

mkdir ${LOGS}/test
touch ${LOGS}/test/file
echo "${LOGS}/test/file" >>${LOGS}/ls.expected

run_a_test false 1 file ls ${LOGS}/test
diff -u ${LOGS}/1.file-ls-proxy-1-host ${LOGS}/ls.expected >${LOGS}/ls.diff
check_status $? ${LOGS}/ls.diff diff check ls wrong. See ${LOGS}/1.file-ls-proxy-1-host and ${LOGS}/ls.expected

run_a_test false 1 file ls --long ${LOGS}/test
tail -1 ${LOGS}/1.file-ls-proxy-1-host | egrep -q -e '^-rw-'
check_status $? /dev/null long list file wrong. 2nd line should start with -rw- - See ${LOGS}/1.file-ls-proxy-1-host

run_a_test false 1 file ls --long --directory ${LOGS}/test
# Too painful to validate this content so we'll just check some basics
head -1 ${LOGS}/1.file-ls-proxy-1-host | egrep -q -e "^drwx.*${LOGS}/test"
check_status $? /dev/null long list dir wrong. 1st line should start with drwx - See ${LOGS}/1.file-ls-proxy-1-host

# One last one...Check we can 2 multiple directories in a query
run_a_test false 2 file ls --long --directory ${LOGS}/test ${LOGS}/test
head -1 ${LOGS}/1.file-ls-proxy-1-host | egrep -q -e "^drwx.*${LOGS}/test"
check_status $? /dev/null long list dir wrong. 1st line should start with drwx - See ${LOGS}/1.file-ls-proxy-1-host
tail -1 ${LOGS}/1.file-ls-proxy-1-host | egrep -q -e "^drwx.*${LOGS}/test"
check_status $? /dev/null long list dir wrong. 2nd line should start with drwx - See ${LOGS}/1.file-ls-proxy-1-host

# Skip if on github (we assume yum, no apt support yet)
if [ -z "${ON_GITHUB}" ]; then
  run_a_test false 10 packages install --name=zziplib --version=0:0.13.62-12.el7.x86_64
  run_a_test false 10 packages update --name=ansible --old_version=0:2.9.27-1.el7.noarch --new_version=0:2.9.27-1.el7.noarch
  run_a_test false 50 packages list
  run_a_test false 50 packages repolist --verbose
else
  echo "Skipping package tests on Github"
fi

run_a_test false 50 process ps
run_a_test false 0 process kill --pid=$$ --signal=0

# Skip if on github (pstack randomly fails)
if [ -z "${ON_GITHUB}" ]; then
  run_a_test true 20 process pstack --pid=${PROXY_PID}
else
  echo "Skipping pstack tests on Github due to transient 'could not find _DYNAMIC symbol' errors"
fi

echo
echo "Expect an error about ptrace failing when we do 2 hosts"
run_a_test true 20 process dump --pid=$$ --dump-type=GCORE
# Cores get their own additional checks.
for i in ${LOGS}/?.process-dump*; do
  # Skip the .error files
  if [ "$(basename $i)" != "$(basename $i .error)" ]; then
    continue
  fi
  file $i | egrep -q "LSB core file.*from 'bash'"
  check_status $? /dev/null $i not a core file
done

# Skip if on github
if [ -z "${ON_GITHUB}" ]; then
  echo "Dumping core to s3 bucket"
  ${SANSSH_PROXY} ${SINGLE_TARGET} process dump --output=s3://${USER}-dev?region=us-west-2 --pid=$$ --dump-type=GCORE
  check_status $? /dev/null Remote dump to s3
  echo Dumping bucket
  aws s3 ls s3://${USER}-dev >${LOGS}/s3-ls
  cat ${LOGS}/s3-ls
  egrep -q "127.0.0.1:[0-9]+-core.$$" ${LOGS}/s3-ls
  check_status $? ${LOGS}/s3-ls cant find core in bucket
  CORE=$(egrep "127.0.0.1:[0-9]+-core.$$" ${LOGS}/s3-ls | awk '{print $4}')
  aws s3 cp s3://${USER}-dev/${CORE} ${LOGS}
  file ${LOGS}/${CORE} | egrep -q "LSB core file.*from 'bash'"
  check_status $? /dev/null ${LOGS}/${CORE} is not a core file

  aws s3 rm s3://${USER}-dev/${CORE}
else
  echo "Skipping s3 dump tests on Github"
fi

echo "Querying systemd for service status"
run_a_test false 10 service list --system-type systemd
run_a_test false 1 service status --system-type systemd systemd-journald

# Have to do these one by one since the parallel rm/rmdir will fail
# since it's the same host and that's ok.
function setup_rmdir {
  mkdir -p ${LOGS}/testdir
}

function setup_rm {
  setup_rmdir
  touch ${LOGS}/testdir/file
}

function check_rm {
  if [ -f ${LOGS}/testdir/file ]; then
    check_status 1 /dev/null ${LOGS}/testdir/file not removed as expected
  fi
}

function check_rmdir {
  if [ -d ${LOGS}/testdir ]; then
    check_status 1 /dev/null ${LOGS}/testdir not removed as expected
  fi
}

echo "rm checks"
echo "rm proxy to 2 hosts (one will fail)"
setup_rm
${SANSSH_PROXY} ${MULTI_TARGETS} file rm ${LOGS}/testdir/file
if [ $? != 1 ]; then
  check_status 1 /dev/null "rm didn't get error when expected"
fi
check_rm

echo "rm proxy to 1 host"
setup_rm
${SANSSH_PROXY} ${SINGLE_TARGET} file rm ${LOGS}/testdir/file
check_status $? /dev/null rm failed
check_rm

echo "rm with no proxy"
setup_rm
${SANSSH_NOPROXY} ${SINGLE_TARGET} file rm ${LOGS}/testdir/file
check_status $? /dev/null rm failed
check_rm

echo "rmdir checks"
echo "rmdir proxy to 2 hosts (one will fail)"
setup_rmdir
${SANSSH_PROXY} ${MULTI_TARGETS} file rmdir ${LOGS}/testdir
if [ $? != 1 ]; then
  check_status 1 /dev/null "rmdir didn't get error when expected"
fi
check_rmdir

echo "rmdir proxy to 1 host"
setup_rmdir
${SANSSH_PROXY} ${SINGLE_TARGET} file rmdir ${LOGS}/testdir
check_status $? /dev/null rmdir failed
check_rmdir

echo "rmdir with no proxy"
setup_rmdir
${SANSSH_NOPROXY} ${SINGLE_TARGET} file rmdir ${LOGS}/testdir
check_status $? /dev/null rmdir failed
check_rmdir

echo "rmdir with files in directory (should fail)"
setup_rm
${SANSSH_NOPROXY} ${SINGLE_TARGET} file rmdir ${LOGS}/testdir
if [ $? != 1 ]; then
  check_status 1 /dev/null "rmdir didn't get error when expected"
fi

# TODO(jchacon): Provide a java binary for test{s
echo
echo "All tests pass."
echo
