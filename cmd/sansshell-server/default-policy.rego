package sansshell.authz

default allow = false

allow {
	input.method = "/HealthCheck.HealthCheck/Ok"
}

# Allow people to run reflection against the server
allow {
	input.method = "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo"
}

allow {
	input.method = "/Dns.Lookup/Lookup"
}

allow {
	input.method = "/HTTPOverRPC.HTTPOverRPC/Host"
}

allow {
	input.type = "LocalFile.ReadActionRequest"
	input.message.file.filename = "/etc/hosts"
}

allow {
	input.type = "LocalFile.StatRequest"
	input.message.filename = "/etc/hosts"
}

allow {
	input.type = "LocalFile.SumRequest"
	input.message.filename = "/etc/hosts"
}

allow {
	input.method = "/Network.PacketCapture/ListInterfaces"
}

allow {
	input.method = "/Network.PacketCapture/RawStream"
}

allow {
	input.type = "Exec.ExecRequest"
	input.message.command = "/bin/echo"
	input.message.args = ["hello", "world"]
}

allow {
	input.type = "Process.ListRequest"
}

allow {
	input.type = "Process.GetStacksRequest"
}

allow {
	input.type = "Packages.ListInstalledRequest"
}

allow {
	input.type = "Packages.SearchRequest"
}

allow {
	input.type = "Packages.RepoListRequest"
}

allow {
	input.type = "Process.GetJavaStacksRequest"
}

allow {
	input.type = "Service.ListRequest"
}

allow {
	input.type = "Service.StatusRequest"
}

denial_hints[msg] {
	input.message.file.filename != "/etc/hosts"
	msg := "we only allow /etc/hosts"
}

allow {
	input.method = "/SysInfo.SysInfo/Uptime"
}

allow {
	input.method = "/SysInfo.SysInfo/Dmesg"
}

# Allow anything from a proxy
allow {
	input.peer.principal.id = "proxy"
}

# Allow anything with MPA
allow {
	input.peer.principal.id = "sanssh"
	input.approvers[_].id = "approver"
}

# Allow MPA listing commands
allow {
	input.method = ["/Mpa.Mpa/Get", "/Mpa.Mpa/List", "/Mpa.Mpa/WaitForApproval"][_]
}

# Allow MPA setting when not sending a proxied identity. The proxy is allowed above.
allow {
	not input.metadata["proxied-sansshell-identity"]
	input.method = ["/Mpa.Mpa/Store", "/Mpa.Mpa/Approve"][_]
}
