# This is the default policy for the Sansshell Proxy
package sansshell.authz

default allow = false

# Allow anyone to call healthcheck on any host
allow {
  input.method = "/HealthCheck.HealthCheck/Ok"
}

# Allow anyone to read /etc/hosts on any host
allow {
  input.method = "/LocalFile.LocalFile/Read"
  input.message.filename = "/etc/hosts"
}

# Allow anyone to calculate sums on /etc/hosts on any host
allow {
  input.method = "/LocalFile.LocalFile/Stat"
  input.message.filename = "/etc/hosts"
}
