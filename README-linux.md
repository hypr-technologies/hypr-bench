# HyprBench for Linux x86_64

This is the Linux x86_64 build of HyprBench, a comprehensive system benchmarking tool.

## Quick Start

1. Extract the archive:
   ```
   tar -xzf hyprbench-linux-amd64.tar.gz
   ```

2. Make the binary executable:
   ```
   chmod +x hyprbench
   ```

3. Run HyprBench:
   ```
   ./hyprbench
   ```

## Dependencies

HyprBench requires the following tools to be installed:

- `sysbench` - For CPU and memory benchmarks
- `fio` - For disk I/O benchmarks
- `stress-ng` - For system stress testing
- `speedtest` or `fast` - For network speed testing
- `iperf3` - For network benchmarks

You can install these dependencies manually, or use the `--auto-install-deps` flag:

```
sudo ./hyprbench --auto-install-deps
```

## Usage

```
./hyprbench [flags]
```

### Common Flags

- `--skip-cpu` - Skip CPU benchmarks
- `--skip-memory` - Skip memory benchmarks
- `--skip-disk` - Skip disk I/O benchmarks
- `--skip-network` - Skip network benchmarks
- `--skip-stress` - Skip stress tests
- `--skip-public-ref` - Skip public reference benchmarks

### FIO Options

- `--fio-target-dir string` - Specify a single directory/device for FIO tests
- `--fio-test-size string` - Override FIO test file size (default "1G")
- `--fio-profile string` - FIO test profile: 'standard', 'quick', 'thorough', 'iops', 'throughput', 'latency', or 'all' (default "standard")

### Export Options

- `--export-json string` - Export results to JSON file
- `--export-html string` - Export results to HTML file

### Web Interface

- `--web` - Start a web server to view results after benchmarks complete
- `--web-port int` - Port to use for the web server (default 8080)

### Other Options

- `--show-progress` - Show progress indicators for long-running benchmarks (default true)
- `--auto-install-deps` - Attempt to automatically install missing dependencies
- `-h, --help` - Show help
- `-v, --version` - Show version

## Examples

Run all benchmarks:
```
./hyprbench
```

Skip disk benchmarks:
```
./hyprbench --skip-disk
```

Run only CPU and memory benchmarks:
```
./hyprbench --skip-disk --skip-network --skip-stress --skip-public-ref
```

Export results to HTML and view in web interface:
```
./hyprbench --export-html results.html --web
```

## License

This software is provided as-is. All rights reserved.
