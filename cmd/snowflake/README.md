# Snowflake CLI

Command-line tool for generating and working with Snowflake IDs.

## Installation

```bash
# From the project root
go install ./cmd/snowflake

# Or build directly
cd cmd/snowflake
go build -o snowflake
```

## Usage

### Generate IDs

```bash
# Generate a single ID
snowflake generate --worker 42

# Generate multiple IDs
snowflake generate --count 10 --worker 42

# Generate in different formats
snowflake generate --format base62 --worker 42
snowflake generate --format hex --worker 5

# Generate in JSON format with full details
snowflake generate --json --worker 42

# Use batch generation for better performance
snowflake generate --count 1000 --batch --worker 42
```

### Parse and Inspect IDs

```bash
# Parse a decimal ID
snowflake parse 1234567890123456789

# Parse Base62 encoded ID
snowflake parse 7n42dgm5tflk

# Output shows:
# - All encoding formats
# - Timestamp, Worker ID, Sequence
# - Age and validation status
```

### Convert Between Formats

```bash
# Convert to Base62
snowflake encode 1234567890123456789 base62

# Convert to Hex
snowflake encode 1234567890123456789 hex

# Works with any input format
snowflake encode hxSLg9p4e4 decimal
```

### Validate IDs

```bash
# Check if an ID is structurally valid
snowflake validate 1234567890123456789

# Shows validation errors if invalid
snowflake validate 12345
```

### Run Benchmarks

```bash
# Quick benchmark (3 seconds)
snowflake bench --worker 42

# Longer benchmark
snowflake bench --duration 10s --worker 42

# Custom batch size
snowflake bench --batch 500 --duration 5s
```

## Supported Formats

| Format | Description | Example |
|--------|-------------|---------|
| `decimal` | Decimal string (default) | `1234567890123456789` |
| `base62` | URL-safe alphanumeric | `7n42dgm5tflk` |
| `base58` | Bitcoin-style (no 0OIl) | `BukQL2gPvMW` |
| `base32` | z-base-32 encoding | `ybndrfg8ejkmc` |
| `hex` | Hexadecimal | `112210f47de98115` |
| `binary` | Binary string | `1000100100...` |

## Command Aliases

For faster typing, most commands have short aliases:

```bash
snowflake g --worker 42          # generate
snowflake p 1234567890123456789  # parse
snowflake e 1234567890123456789 base62  # encode
snowflake v 1234567890123456789  # validate
snowflake b --duration 5s        # bench
```

## Examples

### Generate 1000 IDs and save to file

```bash
snowflake generate --count 1000 --format base62 --worker 42 > ids.txt
```

### Parse IDs from file

```bash
cat ids.txt | while read id; do snowflake parse "$id"; done
```

### Batch generation with performance stats

```bash
snowflake generate --count 10000 --batch --worker 42
# Shows: Generated 10000 IDs in 2.5ms (4,000,000 IDs/sec)
```

### JSON output for scripts

```bash
snowflake generate --count 5 --json --worker 42 | jq '.ids[].base62'
```

## Performance

Typical performance on modern hardware:

- **Single generation**: 3-4 million IDs/sec (~300ns/op)
- **Batch generation**: 4-5 million IDs/sec (~250ns/op)
- **Parsing**: ~10ns/op
- **Encoding**: 30-40ns/op depending on format

## Use Cases

- **Development**: Quickly generate test IDs
- **Debugging**: Parse and inspect IDs from logs
- **Scripts**: Generate IDs in shell scripts
- **Testing**: Benchmark ID generation performance
- **Data Migration**: Convert between ID formats
