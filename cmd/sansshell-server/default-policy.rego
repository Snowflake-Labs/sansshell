package sansshell.authz

default allow = false

allow {
	input.type = "HealthCheck.Empty"
}

allow {
	input.type = "LocalFile.ReadRequest"
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
	input.type = "Ansible.RunRequest"
}
