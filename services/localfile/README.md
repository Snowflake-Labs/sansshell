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
