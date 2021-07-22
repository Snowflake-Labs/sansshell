package unshelled.authz

default allow = false

allow {
	input.type = "HealthCheck.Ok"
}
allow {
	input.type = "LocalFile.ReadRequest"
	input.message.filename = "/etc/hosts"
}
