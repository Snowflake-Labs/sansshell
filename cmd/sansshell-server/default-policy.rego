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
  input.type = "LocalFile.StatRequest"
}

allow {
  input.type = "LocalFile.SumRequest"
}

allow {
	input.type = "Exec.ExecRequest"
	input.message.command = "echo"
	input.message.args = ["hello", "world"]
}
