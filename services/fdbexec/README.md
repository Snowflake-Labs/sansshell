# FdbExec

The FdbExec service provides a way to execute commands on remote hosts.

## Usage

### sanssh fdbexec run
```
sanssh <sanssh-args> fdbexec run [--stream] [--user user] <command> [<args>...]
```

## Notes

The FdbExec service is identical to the Exec service in functionality, but is specifically intended for use by the FDB team. The rego policy can be configured to allow only FDB team members to use this command.

The `--stream` flag streams command output as it is generated.
The `--user user` flag executes the command as the specified user (requires privileges). 