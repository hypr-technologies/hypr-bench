# HyprBench - Comprehensive System Benchmark Utility

## Introduction

HyprBench is a BASH script designed for comprehensive benchmarking of CPU, memory, disk I/O, network, and system stress on Linux systems. It places a particular focus on NVMe performance and aims for real-world relevance in its testing methodologies. HyprBench utilizes a suite of well-known benchmarking tools and includes `hyprbench-netblast.sh` as a component for advanced, multi-server network throughput testing.

## Features

*   **CPU Benchmarks:** Utilizes `sysbench` for single and multi-threaded CPU performance tests.
*   **Memory Benchmarks:** Employs `STREAM` (via Phoronix Test Suite) to measure memory bandwidth (Copy, Scale, Add, Triad).
*   **Disk I/O Benchmarks:** Leverages `fio` (Flexible I/O Tester) with a strong focus on NVMe drive performance, testing various random and sequential read/write scenarios. Supports custom target directories and test sizes.
*   **Network Benchmarks:**
    *   **Local Speed Test:** Uses `speedtest-cli` (Ookla) or `fast-cli` (Netflix) for internet bandwidth assessment.
    *   **iperf3 Tests:** Conducts download and upload tests against public `iperf3` servers.
    *   **Netblast:** Integrates `hyprbench-netblast.sh` for advanced, parallel network throughput testing against multiple iperf3 servers.
*   **System Stress Tests:** Uses `stress-ng` to perform CPU, matrix, and virtual memory stress tests, reporting bogo ops/s.
*   **System Information:** Gathers detailed information about CPU, RAM (including `dmidecode` specifics), motherboard, OS, storage devices (`lsblk`), and NVMe controllers (`lspci`).
*   **Public Reference Benchmarks (Optional):** Includes `UnixBench` (via Phoronix Test Suite) for a general system comparison score.
*   **Clear Reporting:** Outputs results to STDOUT and a timestamped log file.
*   **Dependency Checking:** Verifies the presence of required tools before execution.
*   **Customizable Execution:** Offers command-line arguments to skip specific test sections, override log/temp paths, and customize FIO parameters.

## Dependencies

The following external commands are required by `HyprBench.sh` and its sub-script `hyprbench-netblast.sh`.

Ensure all dependencies listed below are installed on your system before proceeding.

**Core Utilities (usually pre-installed):**
*   `bash`, `date`, `mktemp`, `mkdir`, `rm`, `id`, `echo`, `printf`, `grep`, `awk`, `sed`, `sort`, `head`, `tail`, `tr`, `dirname`, `readlink`, `cat`, `command`, `type`, `set`, `uname`

**Benchmarking & System Info Tools:**
*   `lscpu` (from `util-linux`)
*   `free` (from `procps` or `procps-ng`)
*   `dmidecode`
*   `lsb_release`
*   `lspci` (from `pciutils`)
*   `lsblk` (from `util-linux`)
*   `nproc` (from `coreutils`)
*   `sysbench`
*   `fio` (Flexible I/O Tester)
*   `jq` (for JSON parsing, e.g., FIO, iperf3, fast-cli results)
*   `git` (for Phoronix Test Suite installation)
*   `php-cli` (for Phoronix Test Suite)
*   `php-xml` (for Phoronix Test Suite)
*   `stress-ng`
*   `curl` (for fetching iperf3 server lists, etc.)
*   `iperf3`
*   `bc` (for calculations in `hyprbench-netblast.sh`)

**Network Speed Test Tools (at least one required for local speed test):**
*   `speedtest-cli` (Ookla's official CLI - `speedtest` binary)
*   `fast-cli` (Netflix's `fast.com` CLI - `fast` binary)

**Example Installation Commands:**

*   **Debian/Ubuntu (APT):**
    ```bash
    sudo apt update
    sudo apt install -y util-linux procps dmidecode lsb-release pciutils coreutils \
                        sysbench fio jq git php-cli php-xml stress-ng curl iperf3 bc \
                        speedtest-cli # For Ookla's speedtest, or download from speedtest.net
    # For fast-cli (Node.js/npm required):
    # sudo apt install -y nodejs npm
    # sudo npm install --global fast-cli
    ```
    *Note: `speedtest-cli` might be an older version in some repos. For the official Ookla version, follow instructions at [Speedtest CLI](https://www.speedtest.net/apps/cli).*

*   **RHEL/CentOS/Fedora (YUM/DNF):**
    ```bash
    sudo dnf install -y util-linux procps-ng dmidecode lsb-release pciutils coreutils \
                       sysbench fio jq git php-cli php-xml stress-ng curl iperf3 bc
    # For Ookla's speedtest-cli (official method):
    # curl -s https://install.speedtest.net/app/cli/install.rpm.sh | sudo bash
    # sudo dnf install speedtest
    # For fast-cli (Node.js/npm required):
    # sudo dnf install -y nodejs npm # or use nodesource repository for newer Node versions
    # sudo npm install --global fast-cli
    ```

## Installation

Ensure all dependencies listed in the "Dependencies" section above are installed *before* attempting to run the scripts.

1.  **Clone the Repository:**
    ```bash
    # Replace with the actual URL when available
    git clone https://github.com/yourusername/hyprbench.git
    cd hyprbench
    ```

2.  **Make Scripts Executable:**
    Ensure the scripts have execute permissions.
    ```bash
    chmod +x HyprBench.sh hyprbench-netblast.sh
    ```

3.  **Run from Cloned Directory:**
    You can run HyprBench directly from the cloned directory using `sudo` (required for many benchmark operations):
    ```bash
    sudo ./HyprBench.sh [options]
    sudo ./hyprbench-netblast.sh
    ```
    *(See the Usage section below for available options.)*

4.  **(Optional) Install to a Directory in your PATH:**
    For convenience, you can copy the scripts to a directory included in your system's `$PATH`, such as `/usr/local/bin/`. This allows you to run them from any location without specifying the full path.
    ```bash
    sudo cp HyprBench.sh hyprbench-netblast.sh /usr/local/bin/
    ```
    After copying, you can execute them directly:
    ```bash
    sudo HyprBench.sh [options]
    sudo hyprbench-netblast.sh
    ```

---

## Usage

The script must be run with root privileges.

```bash
sudo ./HyprBench.sh [options]
```

**Command-Line Options:**

*   `--log-file <path>`: Override default log file path (`./logs/hyprbench-YYYYMMDD-HHMMSS.log`).
*   `--temp-dir <path>`: Override default temporary directory path (created in current directory if not absolute).
*   `--skip-cpu`: Skip CPU benchmarks (sysbench).
*   `--skip-memory`: Skip Memory benchmarks (STREAM via Phoronix Test Suite).
*   `--skip-disk`: Skip Disk I/O benchmarks (FIO).
*   `--skip-stress`: Skip `stress-ng` benchmarks.
*   `--skip-network`: Skip all network benchmarks (Speedtest, iperf3, Netblast).
*   `--skip-netblast`: Skip only `hyprbench-netblast.sh` (advanced network tests).
*   `--skip-public-ref`: Skip public reference benchmarks (UnixBench via Phoronix Test Suite).
*   `--fio-target-dir <path>`: Specify a single directory (mount point) for FIO tests, bypassing NVMe auto-detection. Example: `/mnt/test_disk`. Raw device paths are not currently supported.
*   `--fio-test-size <size>`: Override FIO test file size (e.g., `1G`, `4G`, `500M`). Default: `1G`.
*   `-h`, `--help`: Display the help message and exit.

## Output

*   **STDOUT:** Real-time progress, section headers, and summary results are printed to the console with color-coded log levels.
*   **Log File:** All output, including detailed results and debug information, is saved to a log file.
    *   Default location: `./logs/hyprbench-YYYYMMDD-HHMMSS.log`
    *   This path can be changed using the `--log-file` option.

## `hyprbench-netblast.sh`

This companion script is responsible for advanced, parallel network throughput testing against multiple `iperf3` servers. It is typically invoked by `HyprBench.sh` during the network benchmark phase.

It can also be run standalone for focused network blasting:
```bash
sudo ./hyprbench-netblast.sh
```
(Ensure `iperf3` and `bc` are installed.)

## Interpreting Results

*   **Throughput/IOPS/Scores (e.g., sysbench events/sec, STREAM MB/s, FIO IOPS/MB/s, stress-ng bogo ops/s, UnixBench score):** Generally, higher values are better.
*   **Latency (e.g., FIO avg latency, speedtest latency):** Generally, lower values are better.

Compare results against known baselines for your hardware or similar systems to gauge performance.

## Caveats

*   **Run as root:** The script requires root privileges for many operations, including `dmidecode`, `fio` on block devices (though current script uses mount points), and potentially other system-level information gathering or tool execution. The script will exit if not run as root.
*   **Resource Intensive:** Benchmarks are resource-intensive and can be time-consuming. They may significantly load your system's CPU, memory, disk, and network. Use responsibly, especially on production systems.
*   **Network Variability:** Network benchmark results (Speedtest, iperf3) can vary significantly based on current network conditions, server load on the test servers, and geographical location. Run multiple times or at different times for a more comprehensive view if network performance is critical.
*   **Phoronix Test Suite:** Memory (STREAM) and Public Reference (UnixBench) benchmarks rely on the Phoronix Test Suite. `HyprBench.sh` will attempt to clone and set it up in `./phoronix-test-suite` if not found. This process requires `git`, `php-cli`, and `php-xml`. The first run of a PTS test might take longer as it downloads test profiles.