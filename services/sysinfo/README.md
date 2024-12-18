# System Info
Service to retrieve system information from target system, such as kernel messages, uptime and journaling entries.

# Usage

### sanssh sysinfo uptime
Print the uptime of the system in below format: System_idx (ip:port) up for X days, X hours, X minutes, X seconds | total X seconds (X is a placeholder that will be replaced)

```bash
sanssh <sanssh-args> sysinfo uptime
```
Where:
- `<sanssh-args>` common sanssh arguments

Examples:
```bash
# Get data from a yml file
sanssh --targets=localhost sysinfo uptime
```

### sanssh sysinfo dmesg
Print the messages from kernel ring buffer.

```bash
sanssh <sanssh-args> sysinfo dmesg [--tail==<tail-n-lines>] [--grep=<grep-pattern>] [-i] [-v] [--timeout=<timeout-in-seconds>]
```

Where:
- `<sanssh-args>` common sanssh arguments
- `<tail-n-lines>` specify number of lines to `tail`
- `<grep-pattern>` grep regex pattern to filter out messages with
- `-i` - ignore grep case
- `-v` - inverted match grep
- `<timeout-in-seconds>` timeout collection of kernel messages in this number of seconds, default is 2 seconds

Examples:
```bash
### Default
sanssh --targets localhost sysinfo dmesg
### Get messages matching NVMe SSD pattern
sanssh --targets localhost sysinfo dmesg --grep "nvme1n1.*ssd.*"
### Get messages not related to BTRFS (ignoring case)
sanssh --targets localhost sysinfo dmesg -grep "btrfs" -i -v
### Collect messages for 10 seconds
sanssh --targets localhost sysinfo dmesg -grep "btrfs" -i -v --timeout=10
```

### sanssh sysinfo journalctl
Get the log entries stored in journald by systemd-journald.service

```bash
sanssh <sanssh-args> sysinfo djournalctl [--since|--S=<since>] [--until|-U=<until>] [-tail=<tail>] [-u|-unit=<unit>] [--json]
```

Where:
- `<sanssh-args>` common sanssh arguments
- `<since>` Sets the date (YYYY-MM-DD HH:MM:SS) we want to filter from
- `<until>` Sets the date (YYYY-MM-DD HH:MM:SS) we want to filter until
- `<tail>` - If positive, the latest n records to fetch. By default, fetch latest 100 records. The upper limit is 10000 for now
- `<unit>` - Sets systemd unit to filter messages
- `--json` - Print the journal entries in JSON format(can work with jq for better visualization)

Examples:
```bash
### Get 5 journalctl entries in json format between 18th and 19th of December 2024
sanssh --targets localhost sysinfo journalctl --json --tail 5 --since "2024-12-18 00:00:00" --until "2024-12-19 00:00:00"
### Read entries from default.target unit
sanssh --targets localhost sysinfo journalctl --unit default.target
```
