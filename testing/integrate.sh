#!/usr/bin/env bash
set -o nounset

# check_status takes 2 args:
#
# A code to compare against
# Any text to output on failure (all remaining args).
function check_status {
  STATUS=$1
  shift

  if [ $STATUS != 0 ]; then
    echo "FAIL $*"
    exit 1
  fi
}

function shutdown {
  echo "Shutting down"
  kill -KILL ${PROXY_PID}
  sudo killall sansshell-server
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
  wc -l ${LOGS}/1.${SUFFIX} ${LOGS}/2.${SUFFIX} | awk "{
    line1=\$1
    readline
    line2=\$1
    # They should be within an entry of each other and reasonable number of processes.
    if ((line1 != line2 && line1+1 != line2 && line1 != line2+2) || line1 < ${LINE_MIN}) {
      exit 1
    }
  }"
  check_status $? logs mismatch or too short ${LOGS}/1.${SUFFIX} ${LOGS}/2.${SUFFIX} - $*
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

# run_a_test takes 3 args:
#
# A boolean (i.e. true or false string) indicating one run for 2 hosts will error. 
# This can happen for ptrace related commands since 2 cannot act on the same process at once.
# Minimum number of lines log must contain
# The command to pass to the sanssh CLI. NOTE: This is also used as the logfile suffix.
# Args to pass to sanssh after the command (all remaining args)
function run_a_test {
    ONE_WILL_ERROR=$1
    shift
    LINE_MIN=$1
    shift
    CMD=$1
    shift
    ARGS=$*

    echo "${CMD} checks"

    CHECK="${CMD} proxy to 2 hosts"    
    echo ${CHECK}
    ${SANSSH_PROXY} ${MULTI_TARGETS} --outputs=${LOGS}/1.${CMD},${LOGS}/2.${CMD} ${CMD} ${ARGS}
    STATUS=$? 
    if [ "${ONE_WILL_ERROR}" = "false" ]; then
      check_status ${STATUS} ${CHECK}
    else
      # If one of these is expected to error then one log is larger than the other.
      # Take the larger file and copy it so log checking will pass.
      SIZE1=$(du -b ${LOGS}/1.${CMD} | cut -f1)
      SIZE2=$(du -b ${LOGS}/2.${CMD} | cut -f1)
      if (( ${SIZE1} == ${SIZE2} )); then
        echo "FAIL - logs identical $*"
        exit 1
      fi
      if (( ${SIZE1} > ${SIZE2} )); then
        cp ${LOGS}/1.${CMD} ${LOGS}/2.${CMD}
      else
        cp ${LOGS}/2.${CMD} ${LOGS}/1.${CMD}
      fi
    fi
    copy_logs ${CMD} proxy-2-hosts
        
    CHECK="${CMD} proxy to 1 host"
    echo ${CHECK}
    ${SANSSH_PROXY} ${SINGLE_TARGET} --outputs=${LOGS}/1.${CMD} ${CMD} ${ARGS} 
    check_status $? ${CHECK}
    # This way they pass the identical test
    cp ${LOGS}/1.${CMD} ${LOGS}/2.${CMD}
    check_logs ${LINE_MIN} ${CMD} ${CHECK}
    copy_logs ${CMD} proxy-1-host

    CHECK="${CMD} with no proxy"
    echo ${CHECK}
    ${SANSSH_NOPROXY} ${SINGLE_TARGET} --outputs=${LOGS}/1.${CMD} ${CMD} ${ARGS}
    check_status $? ${CHECK}
    # This way they pass the identical test
    cp ${LOGS}/1.${CMD} ${LOGS}/2.${CMD}
    check_logs ${LINE_MIN} ${CMD} ${CHECK}
    copy_logs ${CMD} no-proxy

    echo "${CMD} passed"
}

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
echo "package sansshell.authz" > ${LOGS}/policy
echo "default allow = true" >> ${LOGS}/policy

# Build everything (this won't rebuild the binaries)
go build -v ./...
cd cmd/proxy-server
go build -v
cd ../sanssh
go build -v
cd ../sansshell-server
go build -v 
cd ../..

check_status $? build

# Test everything
go test -v ./...
check_status $? test

# Remove zziplib so we can reinstall it
echo "Removing zziplib package so install can put it back"
sudo yum remove -y zziplib

echo
echo "Starting servers. Logs in ${LOGS}"
./cmd/proxy-server/proxy-server --policy-file=${LOGS}/policy --hostport=localhost:50043 >& ${LOGS}/proxy.log &
PROXY_PID=$!
# Since we're controlling lifetime the shell can ignore this (avoids useless termination messages).
disown %%

# The server needs to be root in order for package installation tests (and the nodes run this as root).
sudo -b ./cmd/sansshell-server/sansshell-server --policy-file=${LOGS}/policy --hostport=localhost:50042 >& ${LOGS}/server.log

SANSSH_NOPROXY="./cmd/sanssh/sanssh --timeout=120s"
SANSSH_PROXY="${SANSSH_NOPROXY} --proxy=localhost:50043"
SINGLE_TARGET="--targets=localhost:50042"
MULTI_TARGETS="--targets=localhost:50042,localhost:50042"

# The first test is server health which also means we're validating
# things came up ok.
echo "Checking server health"
HEALTHY=false
for i in $(seq 1 100); do
  echo -n "${i}: "
  ${SANSSH_PROXY} ${MULTI_TARGETS} --outputs=-,- healthcheck
  if [ $? = 0 ]; then
    HEALTHY=true
    break
  fi
done
if [ "${HEALTHY}" != "true" ]; then
  echo "Servers never healthy"
  exit 1
fi
echo "Servers healthy"

run_a_test false 50 ansible --playbook=$PWD/services/ansible/server/testdata/test.yml --vars=path=/tmp,path2=/
run_a_test false 2 run /usr/bin/echo Hello World
run_a_test false 10 read /etc/hosts

# Tail is harder since we have to run the client in the background and then kill it
cp /etc/hosts ${LOGS}/hosts

# Takes 1 arg:
#
# Pid of sanssh process
function tail_execute {
  CLIENT_PID=$1
  shift
  sleep 1
  cat /etc/hosts >> ${LOGS}/hosts
  sleep 1
  disown %%
  kill -KILL ${CLIENT_PID}
}

# Takes 1 arg:
#
# Suffix for copied log files
function tail_check {
  diff -q ${LOGS}/1.tail ${LOGS}/hosts
  check_status $? "tail: output differs from source"
  copy_logs tail $*
}

echo "tail checks"

echo "tail proxy to 2 hosts"
${SANSSH_PROXY} ${MULTI_TARGETS} --outputs=${LOGS}/1.tail,${LOGS}/2.tail tail ${LOGS}/hosts &
tail_execute $!
diff -q ${LOGS}/1.tail ${LOGS}/2.tail
check_status $? "tail: output files differ"
tail_check proxy-2-hosts

echo "tail proxy to 1 hosts"
${SANSSH_PROXY} ${SINGLE_TARGET} --outputs=${LOGS}/1.tail tail ${LOGS}/hosts &
tail_execute $!
tail_check proxy-1-host

echo "tail with no proxy"
${SANSSH_NOPROXY} ${SINGLE_TARGET} --outputs=${LOGS}/1.tail tail ${LOGS}/hosts &
tail_execute $!
tail_check no-proxy

echo "tail passed"

# Validate immutable with stat
sudo chattr +i ${LOGS}/hosts

run_a_test false 1 stat ${LOGS}/hosts
sudo chattr -i ${LOGS}/hosts

IMMUTABLE_COUNT=$(egrep Immutable..true ${LOGS}/1.stat* | wc -l)
EXPECTED_IMMUTABLE=3
if [ "${IMMUTABLE_COUNT}" != "${EXPECTED_IMMUTABLE}" ]; then
  false
fi
check_status $? "Immutable count incorrect in ${LOGS}/1.stat* expected ${EXPECTED_IMMUTABLE} entries to be immutable"
echo "stat immutable state validated"

run_a_test false 1 sum /etc/hosts

# Record original state before we change it all with chown/etc
touch ${LOGS}/test-file
STATE=$(stat ${LOGS}/test-file | egrep Uid)
ORIG_MODE=$(echo ${STATE} | cut -c 10-13)
ORIG_UID=$(echo ${STATE} | cut -c 34- | awk -F/ '{print $1}')
ORIG_GID=$(echo ${STATE} | cut -c 55- | awk -F/ '{print $1}')
ORIG_IMMUTABLE=$(lsattr ${LOGS}/test-file | cut -c5)

EXPECTED_NEW_UID=$(($ORIG_UID + 1))
EXPECTED_NEW_GID=$(($ORIG_GID + 1))
EXPECTED_NEW_IMMUTABLE="i"

# Determine the new mode but since it's in octal need help
# adding this in the shell as we want to pass it in octal
# to the chmod below.
EXPECTED_NEW_MODE=0$(printf "8\ni\n8\no\n${ORIG_MODE}\n1\n+\np\n" | dc)

run_a_test false 0 chown --uid=$EXPECTED_NEW_UID ${LOGS}/test-file
run_a_test false 0 chgrp --gid=$EXPECTED_NEW_GID ${LOGS}/test-file
run_a_test false 0 chmod --mode=${EXPECTED_NEW_MODE} ${LOGS}/test-file
run_a_test false 0 immutable --state=true ${LOGS}/test-file
NEW_STATE=$(stat ${LOGS}/test-file | egrep Uid)
NEW_MODE=$(echo ${NEW_STATE} | cut -c 10-13)
NEW_UID=$(echo ${NEW_STATE} | cut -c 34- | awk -F/ '{print $1}')
NEW_GID=$(echo ${NEW_STATE} | cut -c 55- | awk -F/ '{print $1}')
NEW_IMMUTABLE=$(lsattr ${LOGS}/test-file | cut -c5)

if [ "$NEW_MODE" != "$EXPECTED_NEW_MODE" ]; then
  check_status 1 "modes not as expected. Started with $ORIG_MODE and now have $NEW_MODE but expected $EXPECTED_NEW_MODE"
fi
# These aren't quoted as parsed ones may have leading spaces
if [ $NEW_UID != $EXPECTED_NEW_UID ]; then
  check_status 1 "uid not as expected. Started with $ORIG_UID and now have $NEW_UID but expected $EXPECTED_NEW_UID"
fi
if [ $NEW_GID != $EXPECTED_NEW_GID ]; then
  check_status 1 "gid not as expected. Started with $ORIG_GID and now have $NEW_GID but expected $EXPECTED_NEW_GID"
fi
if [ "$NEW_IMMUTABLE" != "$EXPECTED_NEW_IMMUTABLE" ]; then
  check_status 1 "imutable not as expected. Started with $ORIG_IMMUTABLE and now have $NEW_IMMUTABLE but expected $EXPECTED_NEW_IMMUTABLE"
fi

# Now validage we can clear immutable too
${SANSSH_NOPROXY} ${SINGLE_TARGET} --outputs=- immutable --state=false ${LOGS}/test-file
check_status $? "setting immutable to false"
ORIG_IMMUTABLE=$NEW_IMMUTABLE
EXPECTED_NEW_IMMUTABLE="-"
NEW_IMMUTABLE=$(lsattr ${LOGS}/test-file | cut -c5)
if [ "$NEW_IMMUTABLE" != "$EXPECTED_NEW_IMMUTABLE" ]; then
  check_status 1 "imutable not as expected. Started with $ORIG_IMMUTABLE and now have $NEW_IMMUTABLE but expected $EXPECTED_NEW_IMMUTABLE"
fi

echo "Uid, etc checks passed"

run_a_test false 10 install --name=zziplib --version=0:0.13.62-12.el7.x86_64
run_a_test false 10 update --name=ansible --old_version=0:2.9.25-1.el7.noarch --new_version=0:2.9.25-1.el7.noarch
run_a_test false 50 list
run_a_test false 50 repolist --verbose
run_a_test false 50 ps
run_a_test true 20 pstack --pid=$$

echo
echo "Expect an error about ptrace failing when we do 2 hosts"
run_a_test true 20 dump --pid=$$ --dump-type=GCORE
# Cores get their own additional checks.
for i in ${LOGS}/?.dump*; do
  file $i | egrep -q "LSB core file.*from 'bash'"
  check_status $? $i not a core file
done


# TODO(jchacon): Provide a java binary for tests
echo 
echo "All tests pass"