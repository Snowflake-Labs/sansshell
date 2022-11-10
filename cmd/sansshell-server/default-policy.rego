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
	input.type = "Exec.ExecRequest"
	input.message.command = "echo"
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
