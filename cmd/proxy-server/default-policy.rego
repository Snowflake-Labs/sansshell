# This is the default policy for the Sansshell Proxy
package sansshell.authz

default allow = false

# Note: this single policy is used to enforce authorization
# for both the proxy itself, and methods called on target
# instances.

## Access control for the proxy. By default, anyone can
# communicate with the proxy itself.
allow {
	input.method = "/Proxy.Proxy/Proxy"
}

# Allow people to run reflection against the proxy
allow {
	input.method = "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo"
}

## Access control for targets

# Allow anyone to call healthcheck on any host
allow {
	input.method = "/HealthCheck.HealthCheck/Ok"
}

# Allow anyone to read /etc/hosts on any host
allow {
	input.method = "/LocalFile.LocalFile/Read"
	input.message.file.filename = "/etc/hosts"
}

# Allow anyone to stat /etc/hosts on any host
allow {
	input.method = "/LocalFile.LocalFile/Stat"
	input.message.filename = "/etc/hosts"
}

# More complex example: allow stat of any file in /etc/ for
# hosts in the 10.0.0.0/8 subnet, for callers in the 'admin'
# group.
#
# allow {
#  input.method = "/LocalFile.LocalFile/Stat"
#  startswith(input.message.filename, "/etc/")
#  net.cidr_contains("10.0.0.0/8", input.host.net.address)
#  some i
#  input.peer.principal.groups[i] = "admin"
# }
