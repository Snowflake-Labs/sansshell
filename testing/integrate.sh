#!/usr/bin/env bash

function check_status {
  if [ $? != 0 ]; then
    echo "FAIL $*"
    exit 1
  fi
}

function shutdown {
  echo "Shutting down"
  # Undo policy changes
  sed -i -e 's:default allow = true:default allow = false:g' ./cmd/proxy-server/default-policy.rego
  sed -i -e 's:default allow = true:default allow = false:g' ./cmd/sansshell-server/default-policy.rego
  kill ${PROXY_PID}
  sudo killall sansshell-server
}

# check_logs takes 3 args:
# 
# Minimum number of lines log must contain
# The log suffix
# Text to display on error (all remaining args)
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
  check_status logs mismatch or too short ${LOGS}/1.${SUFFIX} ${LOGS}/2.${SUFFIX} - $*
}

function copy_logs {
    SUFFIX=$1
    shift
    PURPOSE=$1

    mv ${LOGS}/1.${SUFFIX} ${LOGS}/1.${SUFFIX}-${PURPOSE}
    mv ${LOGS}/2.${SUFFIX} ${LOGS}/2.${SUFFIX}-${PURPOSE}
}

# run_a_test takes 3 args:
#
# A boolean (i.e. true or false string) indicating one run for 2 hosts will error. 
#  This can happen for ptrace related commands since 2 cannot act on the same process at once.
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
    if [ "${ONE_WILL_ERROR}" = "false" ]; then
      check_status ${CHECK}
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
    check_status ${CHECK}
    # This way they pass the identical test
    cp ${LOGS}/1.${CMD} ${LOGS}/2.${CMD}
    check_logs ${LINE_MIN} ${CMD} ${CHECK}
    copy_logs ${CMD} proxy-1-host

    CHECK="${CMD} with no proxy"
    echo ${CHECK}
    ${SANSSH_NOPROXY} ${SINGLE_TARGET} --outputs=${LOGS}/1.${CMD} ${CMD} ${ARGS}
    check_status ${CHECK}
    # This way they pass the identical test
    cp ${LOGS}/1.${CMD} ${LOGS}/2.${CMD}
    check_logs ${LINE_MIN} ${CMD} ${CHECK}
    copy_logs ${CMD} no-proxy

    echo "${CMD} passed"
}

trap shutdown 0
  
# Make sure we're at the top level which is one below where this script lives
cd $(dirname $PWD/$BASH_SOURCE)/..

# Open up policy for testing
sed -i -e 's:default allow = false:default allow = true:g' ./cmd/proxy-server/default-policy.rego
sed -i -e 's:default allow = false:default allow = true:g' ./cmd/sansshell-server/default-policy.rego

# Build everything (this won't rebuild the binaries)
go build -v ./...
cd cmd/proxy-server
go build -v
cd ../sanssh
go build -v
cd ../sansshell-server
go build -v 
cd ../..

check_status build

# Test everything
go test -v ./...
check_status test

LOGS=/tmp/test-logs-$$

mkdir -p ${LOGS}

# Remove zziplib so we can reinstall it
echo "Removing zziplib package so install can put it back"
sudo yum remove -y zziplib

echo
echo "Starting servers. Logs in ${LOGS}"
./cmd/proxy-server/proxy-server --hostport=localhost:50043 >& ${LOGS}/proxy.log &
PROXY_PID=$!

# The server needs to be root in order for package installation tests (and the nodes run this as root).
sudo -b ./cmd/sansshell-server/sansshell-server --hostport=localhost:50042 >& ${LOGS}/server.log

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
run_a_test false 1 stat /etc/hosts
run_a_test false 1 sum /etc/hosts
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
  check_status $i not a core file
done

# TODO(jchacon): Provide a java binary for tests
echo 
echo "All tests pass"