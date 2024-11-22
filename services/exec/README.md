# Exec
Executes arbitrary command remotely and returns the response.

## Usage

### sanssh exec run

For SoT of command line reference run `sanssh exec help run`.

```bash
sanssh <sanssh-args> exec run [--stream] [--user user] <command> [<args>...]
```

Run a command remotely and return the response.

Note: This is not optimized for large output or long running commands.  If
the output doesn't fit in memory in a single proto message or if it doesn't
complete within the timeout, you'll have a bad time.

Where:
- `<sanssh-args>` common sanssh arguments
- `<stream>` flag can be used to stream back command output as the command runs. It doesn't affect the timeout.
- `<user>` lag allows to specify a user for running command, equivalent of `sudo -u <user> <command> ...`
