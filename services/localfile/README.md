# Local file
Service to manipulate file on remote machines

# Usage

### sanssh file get-data
Get data from a file of specific format on a remote host by specified data key.

```bash
sanssh <sanssh-args> file get-data [--format <file-format>] <file-path> <data-key>
```
Where:
- `<sanssh-args>` common sanssh arguments
- `<file-path>` is the path to the file on remote machine. If --format is not provided, format would be detected from file extension.
- `<data-key>` is the key to read from the file. For different file formats it would require keys in different format
    - for `yml`, key should be valid [YAMLPath](https://github.com/goccy/go-yaml/tree/master?tab=readme-ov-file#5-use-yamlpath) string
    - for `dotenv`, key should be a name of variable
- `<file-format>` is the format of the file, if specified it would override the format detected from file extension. Supported formats are:
    - `yml`
    - `dotenv`

Examples:
```bash
# Get data from a yml file
sanssh --targets $TARGET file get-data /etc/config.yml "$.databases[0].host"
# Get data from a dotenv with explicitly specified format
sanssh --targets file get-data --format dotenv /etc/some-config "HOST"
```

### sanssh file set-data
Set data to a file of specific format on a remote host by specified data key.

```bash
sanssh <sanssh-args> file set-data [--format <file-format>] [--value-type <value-type>] <file-path> <data-key> <value>
```
Where:
- `<sanssh-args>` common sanssh arguments
- `<file-path>` is the path to the file on remote machine. If --format is not provided, format would be detected from file extension.
- `<data-key>` is the key to set value in the file. For different file formats it would require keys in different format
  - for `yml`, key should be valid [YAMLPath](https://github.com/goccy/go-yaml/tree/master?tab=readme-ov-file#5-use-yamlpath) string
  - for `dotenv`, key should be a name of variable
- `<value>` is the value to set in the file
- `<file-format>` is the format of the file, if specified it would override the format detected from file extension. Supported formats are:
  - `yml`
  - `dotenv`
- `<value-type>` is the type of value to set in the file. By default, `string`. Supported types are:
  - `string`
  - `int`
  - `float`
  - `bool`

Examples:
```bash
# Set data to a yml file
sanssh --targets $TARGET file set-data /etc/config.yml "database.host" "localhost"
# Set data to a dotenv with explicitly specified format
sanssh --targets file set-data --format dotenv /etc/some-config "HOST" "localhost"
# Set data specified type
sanssh --targets file set-data --value-type int /etc/config.yml "database.port" 8080
```

### sanssh file cp
Copy the source file (which can be local or a URL such as --bucket=s3://bucket <source> or --bucket=file://directory <source>) to the target(s)
placing it into the remote destination.

NOTE: Using file:// means the file must be in that location on each remote target in turn as no data is transferred in that case. Also make
sure to use a fully formed directory. i.e. copying /etc/hosts would be --bucket=file:///etc hosts <destination>

```bash
sanssh <sanssh-args> file cp --uid=X|username=Y --gid=X|group=Y --mode=X [--bucket=XXX] [--overwrite] [--immutable] <source> <remote destination>
```
Where:
- `<sanssh-args>` common sanssh arguments
- `<source>` path to a file to copy from (can be local path or a URL)
- `<remote destination>` path to a file to copy to on a remote machine
- `--uid` The uid the remote file will be set via chown.
- `--username` The remote file will be set to this username via chown.
- `--gid` The gid the remote file will be set via chown.
- `--group` The remote file will be set to this group via chown.
- `--mode` The mode the remote file will be set via chmod. Must be an octal number (e.g. 644, 755, 0777).
- `--bucket` If set to a valid prefix will copy from this bucket with the key being the source provided
- `--overwrite` If true will overwrite the remote file. Otherwise the file pre-existing is an error.
- `--immutable` If true sets the remote file to immutable after being written.

Examples:
```bash
# Copies a local file `local.txt` and stores it on the remote machine as `/tmp/remote.txt`
sanssh --target $TARGET file cp --username=joe --group=staff --mode=644 local.txt /tmp/remote.txt
# Copies a file from an S3 bucket and stores it on the remote machine as `/tmp/remote.txt`
sanssh --target $TARGET file cp --username=joe --group=staff --mode=644 --bucket=s3://my-bucket local.txt /tmp/remote.txt
```

### sanssh file mkdir
Create a directory at the specified path.

Note: Creating intermediate directories is not supported. In order to create `/AAA/BBB/test`,
   both `AAA` and `BBB` must exist.

```bash
sanssh <sanssh-args> file mkdir --uid=X|username=Y --gid=X|group=Y --mode=X <path>
```
Where:
- `<sanssh-args>` common sanssh arguments
- `<path>` path of the new directory
- `--uid` The uid the remote file will be set via chown.
- `--username` The remote file will be set to this username via chown.
- `--gid` The gid the remote file will be set via chown.
- `--group` The remote file will be set to this group via chown.
- `--mode` The mode the remote file will be set via chmod. Must be an octal number (e.g. 644, 755, 0777).

Examples:
```bash
# Creates a new `hello` directory in `/opt`
sanssh --target $TARGET file mkdir --username=joe --group=staff --mode=644 /opt/hello
```
