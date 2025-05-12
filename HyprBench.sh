#!/usr/bin/env bash

# HyprBench.sh - A Comprehensive System Benchmarking Script
# Version: 0.1.0

# --- Strict Mode & Error Handling ---
# set -e: Exit immediately if a command exits with a non-zero status.
# set -o pipefail: The return value of a pipeline is the status of the last command to exit with a non-zero status,
#                  or zero if no command exited with a non-zero status.
# set -u: Treat unset variables as an error when substituting.
set -eo pipefail -u

# --- Constants & Variables ---
readonly SCRIPT_VERSION="0.1.0"
LOG_FILE="" # Will be set in main() e.g., hyprbench-YYYYMMDD-HHMMSS.log
TEMP_DIR="" # Will be created in main() e.g., $(mktemp -d)

# Argument Flags & Values (initialized with defaults)
SKIP_CPU=false
SKIP_MEMORY=false
SKIP_DISK=false
SKIP_STRESS=false
SKIP_NETWORK=false
SKIP_NETBLAST=false # Specific to netblast part of network tests
SKIP_PUBLIC_REF=false
ARG_LOG_FILE=""
ARG_TEMP_DIR=""
ARG_FIO_TARGET_DIR=""
ARG_FIO_TEST_SIZE=""

# Color Codes for Output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[0;33m'
readonly BLUE='\033[0;34m'
readonly MAGENTA='\033[0;35m'
readonly CYAN='\033[0;36m'
readonly NC='\033[0m' # No Color

# --- Logging Functions ---

# _log_base function to handle actual logging logic
# $1: Log level (e.g., INFO, WARN, ERROR)
# $2: Color code for the level
# $3: Message to log
# $4: Output stream (optional, defaults to &1 for STDOUT)
_log_base() {
    local level="$1"
    local color="$2"
    local message="$3"
    local output_stream="${4:-&1}" # Default to STDOUT
    local timestamp
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')

    # Print to the specified output stream (STDOUT or STDERR)
    printf "${color}[%s] [%s] %s${NC}\n" "$timestamp" "$level" "$message" >"${output_stream}"

    # Also print to LOG_FILE if it's set and not empty
    if [[ -n "${LOG_FILE}" ]]; then
        printf "[%s] [%s] %s\n" "$timestamp" "$level" "$message" >> "${LOG_FILE}"
    fi
}

log_info() {
    _log_base "INFO" "${GREEN}" "$1"
}

log_warn() {
    _log_base "WARN" "${YELLOW}" "$1"
}

log_error() {
    _log_base "ERROR" "${RED}" "$1" >&2 # Ensure ERROR messages go to STDERR
}

log_success() {
    _log_base "SUCCESS" "${BLUE}" "$1"
}

log_header() {
    local message="$1"
    local color="${2:-$CYAN}"
    local line
    line=$(printf '%0.s-' $(seq 1 $((${#message} + 4)) ) ) # Create a line of dashes
    _log_base "HEADER" "${color}" "\n${line}\n  ${message}\n${line}"
}


# --- Dependency Check Function ---
# Checks if a command is available in PATH.
# $1: command_name
# $2: package_name_if_different (optional)
check_dependency() {
    local cmd_name="$1"
    local pkg_name="${2:-$1}" # Use cmd_name as pkg_name if not provided

    if ! command -v "${cmd_name}" &> /dev/null; then
        log_error "Dependency missing: '${cmd_name}'. Please install '${pkg_name}'."
        log_info "Example: sudo apt install ${pkg_name} OR sudo yum install ${pkg_name}"
        exit 1
    else
        log_info "Dependency check passed: '${cmd_name}' is available."
    fi
}

# --- Test Section Runner Function (Placeholder) ---
# $1: section_name
# $2: function_to_run
run_test_section() {
    local section_name="$1"
    local function_to_run="$2"

    log_header "--- Running ${section_name} ---" "${MAGENTA}"
    if type -t "${function_to_run}" &> /dev/null && [[ $(type -t "${function_to_run}") == "function" ]]; then
        # Run the function and capture its exit code
        if ! "${function_to_run}"; then
            log_error "Section '${section_name}' (function '${function_to_run}') encountered an error."
            # Optionally, decide if you want to exit the script or continue
            # For now, we log and continue as per individual function error handling
        fi
    else
        log_warn "Test function '${function_to_run}' for section '${section_name}' not found or not a function."
    fi
    log_info "--- Finished ${section_name} ---"
    printf "\n" # Add a newline for better readability
}

# --- Cleanup Function ---
cleanup_temp_files() {
    log_info "Cleaning up temporary files..."
    if [[ -n "${TEMP_DIR}" && -d "${TEMP_DIR}" ]]; then
        rm -rf "${TEMP_DIR}"
        log_success "Temporary directory ${TEMP_DIR} removed."
    else
        log_info "No temporary directory to clean or TEMP_DIR not set."
    fi
}

# --- Trap for Cleanup ---
# Ensures cleanup_temp_files is called on script exit or interruption.
trap cleanup_temp_files EXIT SIGINT SIGTERM

# --- Help Function ---
display_help() {
    echo "HyprBench.sh - Comprehensive System Benchmark Utility"
    echo "Version: ${SCRIPT_VERSION}"
    echo ""
    echo "Usage: sudo ./HyprBench.sh [options]"
    echo ""
    echo "Options:"
    echo "  --log-file <path>       Override default log file path (./logs/hyprbench-YYYYMMDD-HHMMSS.log)"
    echo "  --temp-dir <path>       Override default temporary directory path (created in current dir if not absolute)"
    echo "  --skip-cpu              Skip CPU benchmarks (sysbench)"
    echo "  --skip-memory           Skip Memory benchmarks (STREAM via Phoronix Test Suite)"
    echo "  --skip-disk             Skip Disk I/O benchmarks (FIO)"
    echo "  --skip-stress           Skip stress-ng benchmarks"
    echo "  --skip-network          Skip all network benchmarks (Speedtest, iperf3, Netblast)"
    echo "  --skip-netblast         Skip only hyprbench-netblast.sh (advanced network tests)"
    echo "  --skip-public-ref       Skip public reference benchmarks (UnixBench via Phoronix Test Suite)"
    echo "  --fio-target-dir <path> Specify a single directory (mount point) for FIO tests,"
    echo "                          bypassing NVMe auto-detection. Example: /mnt/test_disk"
    echo "                          Raw device paths are NOT currently supported for safety."
    echo "  --fio-test-size <size>  Override FIO test file size (e.g., 1G, 4G, 500M). Default: 1G"
    echo "  -h, --help              Display this help message and exit"
    echo ""
    echo "Requires root privileges to run correctly."
}

# --- Benchmark Functions ---

cpu_benchmarks() {
    log_info "Starting CPU benchmarks..."

    # Check for nproc dependency
    if ! command -v nproc &> /dev/null; then
        log_error "'nproc' command not found. Cannot determine number of CPU threads. Skipping CPU benchmarks."
        return 1 # Indicate failure
    fi

    local num_threads
    num_threads=$(nproc)
    log_info "Number of CPU threads available: ${num_threads}"

    local prime_20k="20000"
    local prime_100k="100000"
    local time_60s="60"

    local result_single_thread_eps="N/A"
    local result_multi_thread_eps="N/A"
    local result_multi_thread_time_total_events="N/A"

    # Test 1: Single-thread test
    log_info "Running sysbench CPU (1-thread, prime=${prime_20k})..."
    local output_single
    if output_single=$(sysbench cpu --threads=1 --cpu-max-prime="${prime_20k}" run 2>&1); then
        result_single_thread_eps=$(echo "${output_single}" | grep "events per second:" | awk '{print $NF}')
        log_info "Single-thread test (prime=${prime_20k}) events per second: ${result_single_thread_eps}"
    else
        log_error "Sysbench single-thread CPU test failed. Output:"
        echo "${output_single}" | while IFS= read -r line; do log_error "  ${line}"; done
        # We'll keep result_single_thread_eps as "N/A"
    fi

    # Test 2: Multi-thread test (all threads)
    log_info "Running sysbench CPU (${num_threads}-threads, prime=${prime_20k})..."
    local output_multi
    if output_multi=$(sysbench cpu --threads="${num_threads}" --cpu-max-prime="${prime_20k}" run 2>&1); then
        result_multi_thread_eps=$(echo "${output_multi}" | grep "events per second:" | awk '{print $NF}')
        log_info "Multi-thread test (${num_threads}-threads, prime=${prime_20k}) events per second: ${result_multi_thread_eps}"
    else
        log_error "Sysbench multi-thread CPU test (prime=${prime_20k}) failed. Output:"
        echo "${output_multi}" | while IFS= read -r line; do log_error "  ${line}"; done
    fi

    # Test 3: Multi-thread test (fixed time)
    log_info "Running sysbench CPU (${num_threads}-threads, time=${time_60s}s, prime=${prime_100k})..."
    local output_multi_time
    if output_multi_time=$(sysbench cpu --threads="${num_threads}" --time="${time_60s}" --cpu-max-prime="${prime_100k}" run 2>&1); then
        result_multi_thread_time_total_events=$(echo "${output_multi_time}" | grep "total number of events:" | awk '{print $NF}')
        log_info "Multi-thread test (${num_threads}-threads, time=${time_60s}s) total events: ${result_multi_thread_time_total_events}"
    else
        log_error "Sysbench multi-thread CPU test (time=${time_60s}s) failed. Output:"
        echo "${output_multi_time}" | while IFS= read -r line; do log_error "  ${line}"; done
    fi

    # Reporting Results
    printf "\n" | while IFS= read -r line; do log_info "$line"; done # Using this to ensure log_info formatting
    log_info "${BLUE}CPU Benchmark Results:${NC}"
    log_info "${BLUE}----------------------------------------------------------------------${NC}"
    # Header
    printf "%-50s | %s\n" "Test" "Result" | while IFS= read -r line; do log_info "${CYAN}$line${NC}"; done
    log_info "${BLUE}----------------------------------------------------------------------${NC}"
    # Results
    printf "%-50s | %s events/sec\n" "Sysbench CPU (1-thread, prime=${prime_20k})" "${result_single_thread_eps}" | while IFS= read -r line; do log_info "$line"; done
    printf "%-50s | %s events/sec\n" "Sysbench CPU (${num_threads}-threads, prime=${prime_20k})" "${result_multi_thread_eps}" | while IFS= read -r line; do log_info "$line"; done
    printf "%-50s | %s total events\n" "Sysbench CPU (${num_threads}-threads, time=${time_60s}s, prime=${prime_100k})" "${result_multi_thread_time_total_events}" | while IFS= read -r line; do log_info "$line"; done
    log_info "${BLUE}----------------------------------------------------------------------${NC}"
    printf "\n" | while IFS= read -r line; do log_info "$line"; done

    # Check if any test reported N/A, which implies a failure occurred
    if [[ "${result_single_thread_eps}" == "N/A" || "${result_multi_thread_eps}" == "N/A" || "${result_multi_thread_time_total_events}" == "N/A" ]]; then
        log_warn "One or more CPU benchmark tests failed or did not produce a result."
        return 1 # Indicate partial or full failure of this section
    fi

    log_success "CPU benchmarks completed."
    return 0 # Indicate success
}

memory_benchmarks() {
    log_info "Starting Memory benchmarks (STREAM via Phoronix Test Suite)..."
    local PTS_DIR="./phoronix-test-suite"
    local pts_executable="${PTS_DIR}/phoronix-test-suite"

    # --- PTS Setup ---
    if [[ ! -x "${pts_executable}" ]]; then
        log_warn "Phoronix Test Suite not found at ${PTS_DIR}. Attempting installation."

        # Dependencies for PTS were already checked in main(), but good to be mindful.
        # We need git, php-cli, php-xml.

        if ! command -v git &>/dev/null || ! command -v php &>/dev/null; then
            log_error "Core dependencies (git, php) for Phoronix Test Suite are missing. Please install them."
            log_info "Memory benchmarks will be skipped."
            return 1
        fi
        # php-xml is harder to check directly without knowing the exact package name,
        # but php installation should ideally include it or PTS setup will fail later.

        log_info "Creating PTS directory parent: $(dirname "${PTS_DIR}")"
        if ! mkdir -p "$(dirname "${PTS_DIR}")"; then
            log_error "Failed to create parent directory for PTS at $(dirname "${PTS_DIR}")."
            log_info "Memory benchmarks will be skipped."
            return 1
        fi

        log_info "Cloning Phoronix Test Suite from GitHub..."
        if ! git clone https://github.com/phoronix-test-suite/phoronix-test-suite.git "${PTS_DIR}"; then
            log_error "Failed to clone Phoronix Test Suite."
            log_info "Memory benchmarks will be skipped."
            rm -rf "${PTS_DIR}" # Clean up partial clone
            return 1
        fi
        log_success "Phoronix Test Suite cloned to ${PTS_DIR}"

        log_info "Attempting Phoronix Test Suite enterprise setup (for non-interactive license acceptance)..."
        # This command might still require interaction or fail on some systems.
        # Using a timeout to prevent indefinite hanging.
        if timeout 60s "${pts_executable}" enterprise-setup; then
            log_success "Phoronix Test Suite enterprise setup completed."
        else
            local setup_exit_code=$?
            if [[ $setup_exit_code -eq 124 ]]; then # Timeout
                log_warn "Phoronix Test Suite 'enterprise-setup' timed out after 60 seconds."
                log_warn "This might indicate it's waiting for user input (e.g., license agreement)."
                log_warn "Please try running '${pts_executable} enterprise-setup' manually if issues persist."
                log_warn "Proceeding with benchmark, but it might fail if licenses are not accepted."
            else
                log_warn "Phoronix Test Suite 'enterprise-setup' failed with exit code ${setup_exit_code}."
                log_warn "Memory benchmarks might fail or require manual license agreement."
                log_warn "You can try running '${pts_executable} enterprise-setup' manually."
            fi
            # We'll proceed, as PTS might still work for some tests or if licenses were accepted previously.
        fi
    else
        log_info "Phoronix Test Suite found at ${PTS_DIR}."
    fi

    # --- Run STREAM Benchmark ---
    log_info "Running STREAM benchmark using Phoronix Test Suite..."
    log_info "This may take a few minutes, especially on the first run as PTS downloads test files."
    local stream_output
    local stream_copy="N/A"
    local stream_scale="N/A"
    local stream_add="N/A"
    local stream_triad="N/A"
    local benchmark_failed=0

    # Ensure ~/.phoronix-test-suite exists and is writable, PTS usually handles this.
    # However, creating it might avoid some first-run prompts if permissions are an issue.
    mkdir -p ~/.phoronix-test-suite

    if stream_output=$("${pts_executable}" batch-run pts/stream 2>&1); then
        log_info "STREAM benchmark command executed. Parsing results..."

        # Parse results from STDOUT (Primary Method)
        # Example lines:
        #   System Memory Performance (Copy) : 30069.83 MB/s
        #   System Memory Performance (Scale): 29045.12 MB/s
        #   System Memory Performance (Add)  : 30860.71 MB/s
        #   System Memory Performance (Triad): 31095.33 MB/s
        # Sometimes the output might be slightly different, e.g. "pts/stream: Test Result" then the lines.
        # We'll try to be flexible.

        stream_copy=$(echo "${stream_output}" | grep -i "Copy" | grep -i "MB/s" | awk -F': ' '{print $NF}' | sed 's/ MB\/s//' | tail -n 1)
        stream_scale=$(echo "${stream_output}" | grep -i "Scale" | grep -i "MB/s" | awk -F': ' '{print $NF}' | sed 's/ MB\/s//' | tail -n 1)
        stream_add=$(echo "${stream_output}" | grep -i "Add" | grep -i "MB/s" | awk -F': ' '{print $NF}' | sed 's/ MB\/s//' | tail -n 1)
        stream_triad=$(echo "${stream_output}" | grep -i "Triad" | grep -i "MB/s" | awk -F': ' '{print $NF}' | sed 's/ MB\/s//' | tail -n 1)

        # Validate parsed values (check if they are empty or look like numbers)
        if [[ -z "${stream_copy}" || -z "${stream_scale}" || -z "${stream_add}" || -z "${stream_triad}" ]]; then
            log_warn "Failed to parse some or all STREAM results from standard output."
            log_info "Full STREAM output:"
            echo "${stream_output}" | while IFS= read -r line; do log_info "  ${line}"; done
            benchmark_failed=1
        else
            # Check if values are numeric (simple check)
            if ! [[ "${stream_copy}" =~ ^[0-9.-]+$ && \
                    "${stream_scale}" =~ ^[0-9.-]+$ && \
                    "${stream_add}" =~ ^[0-9.-]+$ && \
                    "${stream_triad}" =~ ^[0-9.-]+$ ]]; then
                log_warn "Parsed STREAM results do not appear to be valid numbers."
                log_info "Copy: ${stream_copy}, Scale: ${stream_scale}, Add: ${stream_add}, Triad: ${stream_triad}"
                log_info "Full STREAM output:"
                echo "${stream_output}" | while IFS= read -r line; do log_info "  ${line}"; done
                benchmark_failed=1
                # Reset to N/A
                stream_copy="N/A"; stream_scale="N/A"; stream_add="N/A"; stream_triad="N/A"
            else
                 log_success "STREAM results parsed successfully."
            fi
        fi
    else
        log_error "Phoronix Test Suite 'batch-run pts/stream' command failed."
        log_info "Full command output:"
        echo "${stream_output}" | while IFS= read -r line; do log_error "  ${line}"; done
        benchmark_failed=1
    fi

    # --- Reporting Results ---
    printf "\n" | while IFS= read -r line; do log_info "$line"; done
    log_info "${BLUE}Memory Benchmark Results (STREAM via Phoronix Test Suite):${NC}"
    log_info "${BLUE}------------------------------------${NC}"
    printf "%-8s | %s\n" "Test" "Bandwidth (MB/s)" | while IFS= read -r line; do log_info "${CYAN}$line${NC}"; done
    log_info "${BLUE}------------------------------------${NC}"
    printf "%-8s | %s\n" "Copy" "${stream_copy}"   | while IFS= read -r line; do log_info "$line"; done
    printf "%-8s | %s\n" "Scale" "${stream_scale}" | while IFS= read -r line; do log_info "$line"; done
    printf "%-8s | %s\n" "Add" "${stream_add}"     | while IFS= read -r line; do log_info "$line"; done
    printf "%-8s | %s\n" "Triad" "${stream_triad}" | while IFS= read -r line; do log_info "$line"; done
    log_info "${BLUE}------------------------------------${NC}"
    printf "\n" | while IFS= read -r line; do log_info "$line"; done

    if [[ ${benchmark_failed} -eq 1 || "${stream_copy}" == "N/A" ]]; then
        log_warn "Memory benchmark (STREAM) results are unavailable or incomplete due to errors."
        return 1 # Indicate failure for this section
    fi

    log_success "Memory benchmarks (STREAM) completed."
    return 0
}

disk_benchmarks() {
    log_info "Starting Disk I/O benchmarks (FIO)..."
    local overall_status=0 # 0 for success, 1 for any failure/skip

    # --- Configuration ---
    # FIO_TEST_FILE_SIZE: Size of the test file FIO will use.
    local FIO_TEST_FILE_SIZE="1G" # Default
    if [[ -n "${ARG_FIO_TEST_SIZE}" ]]; then
        log_info "Overriding FIO test file size with user-provided value: ${ARG_FIO_TEST_SIZE}"
        FIO_TEST_FILE_SIZE="${ARG_FIO_TEST_SIZE}"
    else
        log_info "Using default FIO test file size: ${FIO_TEST_FILE_SIZE}"
    fi

    # Convert FIO_TEST_FILE_SIZE to bytes for space check
    local fio_size_bytes
    if [[ "${FIO_TEST_FILE_SIZE}" =~ ([0-9]+)G$ ]]; then
        fio_size_bytes=$(( ${BASH_REMATCH[1]} * 1024 * 1024 * 1024 ))
    elif [[ "${FIO_TEST_FILE_SIZE}" =~ ([0-9]+)M$ ]]; then
        fio_size_bytes=$(( ${BASH_REMATCH[1]} * 1024 * 1024 ))
    elif [[ "${FIO_TEST_FILE_SIZE}" =~ ([0-9]+)K$ ]]; then
        fio_size_bytes=$(( ${BASH_REMATCH[1]} * 1024 ))
    else
        log_error "Invalid FIO_TEST_FILE_SIZE format: ${FIO_TEST_FILE_SIZE}. Expected format like 1G, 512M, etc."
        return 1
    fi
    log_info "FIO test file size in bytes: ${fio_size_bytes}"

    local num_cpu_cores
    num_cpu_cores=$(nproc 2>/dev/null || echo 4) # Default to 4 if nproc fails

    # --- FIO Test Scenarios ---
    declare -a fio_scenarios=(
        "4K_RandRead_QD64;randread;4k;64;4"
        "4K_RandWrite_QD64;randwrite;4k;64;4"
        "1M_SeqRead_QD32;read;1m;32;2"
        "1M_SeqWrite_QD32;write;1m;32;2"
        "4K_Mixed_R70W30_QD64;randrw;4k;64;4;70" # Last field is rwmixread
    )

    local tests_performed_count=0
    declare -a fio_test_targets_info=() # Array of "mount_point;device_display_name"

    if [[ -n "${ARG_FIO_TARGET_DIR}" ]]; then
        log_info "User specified FIO target directory: ${ARG_FIO_TARGET_DIR}"
        # Ensure it's a directory and not a raw device path for now
        if [[ "${ARG_FIO_TARGET_DIR}" == /dev/* ]]; then
            log_error "Specified FIO target '${ARG_FIO_TARGET_DIR}' appears to be a raw device path."
            log_error "Direct raw device testing is not supported in this script version for safety."
            log_error "Please provide a path to a mounted directory for FIO tests."
            return 1
        fi
        if [[ ! -d "${ARG_FIO_TARGET_DIR}" ]]; then
            log_error "Specified FIO target directory '${ARG_FIO_TARGET_DIR}' does not exist or is not a directory."
            return 1
        fi
        # Check if it's root, and warn/skip if it is
        local actual_target_path
        actual_target_path=$(readlink -f "${ARG_FIO_TARGET_DIR}")
        if [[ "${actual_target_path}" == "/" ]]; then
             log_warn "Specified FIO target '${ARG_FIO_TARGET_DIR}' resolves to the root filesystem ('/')."
             log_warn "Testing directly on the root filesystem can be risky and affect system stability."
             log_warn "Skipping FIO tests on '/' for safety."
             return 1 # Critical enough to stop disk benchmarks
        fi
        fio_test_targets_info+=("${actual_target_path};${ARG_FIO_TARGET_DIR}")
    else
        # --- NVMe Drive Detection (Original Logic) ---
        log_info "Detecting NVMe drives for FIO testing..."
        local nvme_devices_json
        if ! nvme_devices_json=$(lsblk -djo KNAME,TYPE,ROTA,SIZE,VENDOR,MODEL 2>/dev/null); then
            log_error "Failed to list block devices using 'lsblk'. Skipping NVMe FIO tests."
            return 1
        fi

        local nvme_device_names
        mapfile -t nvme_device_names < <(echo "${nvme_devices_json}" | jq -r '.blockdevices[] | select(.type=="disk" and .rota=="0" and (.kname | startswith("nvme"))) | .kname')

        if [[ ${#nvme_device_names[@]} -eq 0 ]]; then
            log_warn "No NVMe drives automatically detected. If you have other storage, use --fio-target-dir."
            log_warn "Skipping FIO tests as no NVMe drives found and no target specified."
            return 0 # Not an error, just no NVMe to test by default
        fi

        log_info "Detected NVMe drives. Will attempt to find suitable mount points:"
        for dev_kname in "${nvme_device_names[@]}"; do
            local dev_path="/dev/${dev_kname}"
            log_info "Processing NVMe device: ${dev_path}"
            local current_mount_point=""
            # Check if the device itself is mounted
            current_mount_point=$(lsblk -no MOUNTPOINT "${dev_path}" 2>/dev/null | tr -d '[:space:]')

            if [[ -z "${current_mount_point}" || "${current_mount_point}" == "null" ]]; then
                log_info "${dev_path} is not directly mounted. Searching for largest mounted partition on it..."
                current_mount_point=$(lsblk -no MOUNTPOINT,SIZE -r "${dev_path}" 2>/dev/null | awk 'NF==2 && $1 ~ /^\// {print $2,$1}' | sort -hr | head -n1 | awk '{print $2}')
                if [[ -z "${current_mount_point}" ]]; then
                    log_warn "No mounted partitions found for ${dev_path}. Skipping FIO tests for this device."
                    overall_status=1 # Mark a skip occurred
                    continue
                fi
                log_info "Found largest mounted partition: ${current_mount_point} on ${dev_path}"
            else
                log_info "${dev_path} is directly mounted at ${current_mount_point}"
            fi

            if [[ "${current_mount_point}" == "/" ]]; then
                log_warn "Selected mount point for ${dev_path} is the root filesystem ('/')."
                log_warn "Skipping FIO tests on ${dev_path} at '/' for safety."
                overall_status=1
                continue
            fi
            log_info "Using mount point: ${current_mount_point} for device ${dev_path}"
            fio_test_targets_info+=("${current_mount_point};${dev_path}")
        done
    fi
    printf "\n"

    if [[ ${#fio_test_targets_info[@]} -eq 0 ]]; then
        log_warn "No suitable targets found or configured for FIO benchmarks after filtering."
        return ${overall_status} # Return current status (might be 0 if only specified target was root)
    fi

    # --- Loop Through Each FIO Target ---
    for target_info_str in "${fio_test_targets_info[@]}"; do
        IFS=';' read -r mount_point device_display_name <<< "${target_info_str}"

        log_header "Starting FIO Benchmarks for ${device_display_name} (on mount point: ${mount_point})" "${BLUE}"

        # 2. Test File Management
        # Make fio_test_file_path unique using a sanitized version of device_display_name or mount_point
        local unique_suffix
        unique_suffix=$(echo "${device_display_name}" | tr -cd '[:alnum:]_-') # Sanitize for filename
        [[ -z "${unique_suffix}" ]] && unique_suffix=$(echo "${mount_point}" | tr -cd '[:alnum:]_-') # Fallback if device_display_name is odd
        unique_suffix=$(echo "${unique_suffix}" | tr '/' '_') # Replace slashes if any remain from mount_point
        
        local fio_test_file_path="${mount_point}/fio_test_file_hyprbench_${unique_suffix}"
        log_info "FIO test file will be: ${fio_test_file_path}"

        # Check available disk space
        local available_space_bytes
        available_space_bytes=$(df --output=avail -B1 "${mount_point}" | tail -n1 | tr -d '[:space:]')
        if ! [[ "${available_space_bytes}" =~ ^[0-9]+$ ]]; then
            log_error "Could not determine available space on ${mount_point} for ${device_path}. df output: $(df --output=avail -B1 "${mount_point}" | tail -n1)"
            log_error "Skipping FIO tests for this device."
            overall_status=1
            continue
        fi
        log_info "Available space on ${mount_point}: ${available_space_bytes} bytes."

        if (( available_space_bytes < fio_size_bytes )); then
            log_error "Not enough space on ${mount_point} for FIO test file (${FIO_TEST_FILE_SIZE})."
            log_error "Available: ${available_space_bytes} bytes, Required: ${fio_size_bytes} bytes."
            log_error "Skipping FIO tests for ${device_path} or consider reducing FIO_TEST_FILE_SIZE."
            overall_status=1
            continue
        fi

        # --- Reporting Header for this device ---
        printf "\n" | while IFS= read -r line; do log_info "$line"; done
        log_info "${CYAN}FIO Benchmark Results for ${device_path} on ${mount_point}:${NC}"
        log_info "${CYAN}Test File: ${fio_test_file_path}, Size: ${FIO_TEST_FILE_SIZE}${NC}"
        log_info "${CYAN}---------------------------------------------------------------------------------${NC}"
        printf "%-32s | %-10s | %-18s | %-18s\n" "Test Type" "IOPS" "Bandwidth (MB/s)" "Avg Latency (us)" | while IFS= read -r line; do log_info "${YELLOW}$line${NC}"; done
        log_info "${CYAN}---------------------------------------------------------------------------------${NC}"

        local device_test_failed=0
        tests_performed_count=$((tests_performed_count + 1))

        # 3. FIO Test Execution Loop
        for scenario_params_str in "${fio_scenarios[@]}"; do
            IFS=';' read -r test_name rw bs iodepth numjobs rwmixread <<< "${scenario_params_str}"

            log_info "Running FIO test: ${test_name} (rw=${rw}, bs=${bs}, iodepth=${iodepth}, numjobs=${numjobs}${rwmixread:+, rwmixread=${rwmixread}})..."

            local fio_cmd
            fio_cmd=(fio
                "--name=${test_name}"
                "--filename=${fio_test_file_path}"
                "--ioengine=libaio" # Common for Linux, consider alternatives if needed
                "--direct=1"        # Bypass OS cache
                "--rw=${rw}"
                "--bs=${bs}"
                "--iodepth=${iodepth}"
                "--numjobs=${numjobs}"
                "--size=${FIO_TEST_FILE_SIZE}"
                "--runtime=60"      # Duration of the test in seconds
                "--group_reporting" # Report aggregate results for all jobs
                "--output-format=json"
            )
            if [[ -n "${rwmixread}" ]]; then
                fio_cmd+=("--rwmixread=${rwmixread}")
            fi

            # log_info "Executing FIO command: ${fio_cmd[*]}" # For debugging
            local fio_output_json
            local fio_exit_code=0
            if ! fio_output_json=$("${fio_cmd[@]}" 2>&1); then
                fio_exit_code=$?
                log_error "FIO command for test '${test_name}' failed with exit code ${fio_exit_code}."
                log_error "FIO output/error:"
                echo "${fio_output_json}" | while IFS= read -r line; do log_error "  ${line}"; done
                printf "%-32s | %-10s | %-18s | %-18s\n" "${test_name}" "FAIL" "FAIL" "FAIL" | while IFS= read -r line; do log_info "$line"; done
                device_test_failed=1
                overall_status=1
                continue
            fi

            # 4. Parse FIO JSON Output
            local iops="N/A"
            local bw_mbs="N/A"
            local lat_us="N/A"

            # IOPS: Try read, then write, then mix (if applicable), then total.
            # FIO JSON structure: .jobs[0].read.iops, .jobs[0].write.iops
            # For mixed, .jobs[0].mix.iops or sum .jobs[0].read.iops and .jobs[0].write.iops or .jobs[0].total_iops
            iops=$(echo "${fio_output_json}" | jq -r '(.jobs[0].read.iops // .jobs[0].write.iops // .jobs[0].mix.iops // .jobs[0].total_iops // 0) | tonumber | if . == 0 then "N/A" else . end')
            if [[ "${iops}" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then # Check if it's a number before formatting
                iops=$(printf "%.0f" "${iops}")
            fi


            # Bandwidth (MB/s): Try read, then write, then mix. Use bw_bytes if available (Bytes/s), convert to MB/s.
            # Else, use .bw (KiB/s) and convert to MB/s.
            local bw_field_bytes_val
            bw_field_bytes_val=$(echo "${fio_output_json}" | jq -r '(.jobs[0].read.bw_bytes // .jobs[0].write.bw_bytes // .jobs[0].mix.bw_bytes // 0) | tonumber')

            if [[ "${bw_field_bytes_val}" != "0" && -n "${bw_field_bytes_val}" ]]; then
                bw_mbs=$(awk -v bw_bytes="${bw_field_bytes_val}" 'BEGIN { printf "%.2f", bw_bytes / (1024*1024) }')
            else
                # Fallback to .bw field (KiB/s)
                local bw_field_kib_val
                bw_field_kib_val=$(echo "${fio_output_json}" | jq -r '(.jobs[0].read.bw // .jobs[0].write.bw // .jobs[0].bw_mean // 0) | tonumber') # bw_mean often for mixed
                if [[ "${bw_field_kib_val}" != "0" && -n "${bw_field_kib_val}" ]]; then
                    bw_mbs=$(awk -v bw_kib="${bw_field_kib_val}" 'BEGIN { printf "%.2f", bw_kib / 1024 }')
                else
                    bw_mbs="N/A"
                fi
            fi


            # Latency (us): Use clat_ns.mean (completion latency in nanoseconds), convert to microseconds.
            # Try read, then write, then an aggregate if available (total_lat_ns or similar, though clat is usually per R/W)
            local lat_ns_mean
            lat_ns_mean=$(echo "${fio_output_json}" | jq -r '(.jobs[0].read.clat_ns.mean // .jobs[0].write.clat_ns.mean // .jobs[0].latency_ns.mean // 0) | tonumber') # Added latency_ns.mean as a fallback
            if [[ "${lat_ns_mean}" != "0" && -n "${lat_ns_mean}" ]]; then
                lat_us=$(awk -v lat_ns="${lat_ns_mean}" 'BEGIN { printf "%.1f us", lat_ns / 1000 }')
            else
                # Try msec fields if ns not found (less common for clat)
                local lat_ms_mean
                lat_ms_mean=$(echo "${fio_output_json}" | jq -r '(.jobs[0].read.clat_ms.mean // .jobs[0].write.clat_ms.mean // .jobs[0].latency_ms.mean // 0) | tonumber')
                if [[ "${lat_ms_mean}" != "0" && -n "${lat_ms_mean}" ]]; then
                     lat_us=$(printf "%.1f ms" "${lat_ms_mean}") # Already in ms
                else
                    lat_us="N/A"
                fi
            fi

            # Handle cases where jq might return null or empty string and it wasn't caught by `// 0 | tonumber`
            [[ "${iops}" == "null" || -z "${iops}" ]] && iops="N/A"
            [[ "${bw_mbs}" == "null" || -z "${bw_mbs}" ]] && bw_mbs="N/A"
            [[ "${lat_us}" == "null" || -z "${lat_us}" ]] && lat_us="N/A"

            printf "%-32s | %-10s | %-18s | %-18s\n" "${test_name}" "${iops}" "${bw_mbs}" "${lat_us}" | while IFS= read -r line; do log_info "$line"; done

        done # End of FIO scenarios loop for a device

        log_info "${CYAN}---------------------------------------------------------------------------------${NC}"

        # 5. Cleanup Test File
        if [[ -f "${fio_test_file_path}" ]]; then
            log_info "Cleaning up FIO test file: ${fio_test_file_path}..."
            if rm -f "${fio_test_file_path}"; then
                log_success "Successfully removed FIO test file: ${fio_test_file_path}"
            else
                log_error "Failed to remove FIO test file: ${fio_test_file_path}. Please remove it manually."
                overall_status=1
            fi
        else
            log_info "FIO test file ${fio_test_file_path} not found or already removed."
        fi

        if [[ ${device_test_failed} -eq 1 ]]; then
            log_warn "One or more FIO tests failed for ${device_path}."
        else
            log_success "FIO benchmarks completed for ${device_path} on ${mount_point}."
        fi
        printf "\n"

    done # End of NVMe devices loop

    if [[ ${tests_performed_count} -eq 0 && ${#nvme_device_names[@]} -gt 0 ]]; then
        log_warn "NVMe drives were detected, but no FIO tests were performed (e.g., due to mount point issues, space, or root fs safety)."
        overall_status=1
    elif [[ ${tests_performed_count} -gt 0 && ${overall_status} -eq 0 ]]; then
        log_success "All Disk I/O benchmarks (FIO) completed successfully for tested NVMe devices."
    elif [[ ${tests_performed_count} -gt 0 && ${overall_status} -ne 0 ]]; then
        log_warn "Disk I/O benchmarks (FIO) completed with some errors or skips for NVMe devices."
    fi

    # Comment: Placeholder for future Boot Disk Test
    # log_info "Future: Consider adding specific tests for the boot disk if not covered by NVMe tests."
    # This would involve identifying the boot disk (e.g., `lsblk` output where MOUNTPOINT is /)
    # and running a similar set of FIO tests, perhaps with different parameters or safety checks.

    return ${overall_status}
}

stress_ng_benchmarks() {
    log_info "Starting Threads & System Stress benchmarks (stress-ng)..."
    local overall_status=0 # 0 for success, 1 for any failure/skip

    # nproc dependency is checked in main()
    local num_threads
    num_threads=$(nproc)
    log_info "Number of CPU threads available for stress-ng: ${num_threads}"

    local cpu_bogo_ops_s="N/A"
    local matrix_bogo_ops_s="N/A"
    local vm_bogo_ops_s="N/A"

    # Test 1: CPU Stress
    log_info "Running stress-ng CPU Stress (--cpu \"${num_threads}\" --cpu-method all -t 60s)..."
    local stress_ng_cpu_output
    if stress_ng_cpu_output=$(stress-ng --cpu "${num_threads}" --cpu-method all -t 60s --metrics-brief 2>&amp;1); then
        cpu_bogo_ops_s=$(echo "${stress_ng_cpu_output}" | grep "^cpu" | awk '{print $4}')
        if [[ -z "${cpu_bogo_ops_s}" || ! "${cpu_bogo_ops_s}" =~ ^[0-9.]+$ ]]; then
            log_warn "Failed to parse CPU Stress bogo ops/s. Output:"
            echo "${stress_ng_cpu_output}" | while IFS= read -r line; do log_warn "  ${line}"; done
            cpu_bogo_ops_s="N/A"
            overall_status=1
        else
            log_info "CPU Stress bogo ops/s: ${cpu_bogo_ops_s}"
        fi
    else
        log_error "stress-ng CPU Stress command failed. Output:"
        echo "${stress_ng_cpu_output}" | while IFS= read -r line; do log_error "  ${line}"; done
        overall_status=1
    fi

    # Test 2: Matrix Stress
    log_info "Running stress-ng Matrix Stress (--matrix \"${num_threads}\" -t 60s)..."
    local stress_ng_matrix_output
    if stress_ng_matrix_output=$(stress-ng --matrix "${num_threads}" -t 60s --metrics-brief 2>&amp;1); then
        matrix_bogo_ops_s=$(echo "${stress_ng_matrix_output}" | grep "^matrix" | awk '{print $4}')
        if [[ -z "${matrix_bogo_ops_s}" || ! "${matrix_bogo_ops_s}" =~ ^[0-9.]+$ ]]; then
            log_warn "Failed to parse Matrix Stress bogo ops/s. Output:"
            echo "${stress_ng_matrix_output}" | while IFS= read -r line; do log_warn "  ${line}"; done
            matrix_bogo_ops_s="N/A"
            overall_status=1
        else
            log_info "Matrix Stress bogo ops/s: ${matrix_bogo_ops_s}"
        fi
    else
        log_error "stress-ng Matrix Stress command failed. Output:"
        echo "${stress_ng_matrix_output}" | while IFS= read -r line; do log_error "  ${line}"; done
        overall_status=1
    fi

    # Test 3: Memory Stress (VM)
    log_info "Running stress-ng VM Stress (--vm 2 --vm-bytes 50% -t 30s)..."
    local stress_ng_vm_output
    if stress_ng_vm_output=$(stress-ng --vm 2 --vm-bytes 50% -t 30s --metrics-brief 2>&amp;1); then
        vm_bogo_ops_s=$(echo "${stress_ng_vm_output}" | grep "^vm" | awk '{print $4}')
        if [[ -z "${vm_bogo_ops_s}" || ! "${vm_bogo_ops_s}" =~ ^[0-9.]+$ ]]; then
            log_warn "Failed to parse VM Stress bogo ops/s. Output:"
            echo "${stress_ng_vm_output}" | while IFS= read -r line; do log_warn "  ${line}"; done
            vm_bogo_ops_s="N/A"
            overall_status=1
        else
            log_info "VM Stress bogo ops/s: ${vm_bogo_ops_s}"
        fi
    else
        log_error "stress-ng VM Stress command failed. Output:"
        echo "${stress_ng_vm_output}" | while IFS= read -r line; do log_error "  ${line}"; done
        overall_status=1
    fi

    # Reporting Results
    printf "\n" | while IFS= read -r line; do log_info "$line"; done
    log_info "${BLUE}Threads & System Stress Results (stress-ng):${NC}"
    log_info "${BLUE}--------------------------------------------------${NC}"
    printf "%-30s | %s\n" "Test" "Bogo Ops/s (Higher is better)" | while IFS= read -r line; do log_info "${CYAN}$line${NC}"; done
    log_info "${BLUE}--------------------------------------------------${NC}"
    printf "%-30s | %s\n" "CPU Stress (all methods)" "${cpu_bogo_ops_s}" | while IFS= read -r line; do log_info "$line"; done
    printf "%-30s | %s\n" "Matrix Stress" "${matrix_bogo_ops_s}" | while IFS= read -r line; do log_info "$line"; done
    printf "%-30s | %s\n" "VM Stress (50% mem)" "${vm_bogo_ops_s}" | while IFS= read -r line; do log_info "$line"; done
    log_info "${BLUE}--------------------------------------------------${NC}"
    printf "\n" | while IFS= read -r line; do log_info "$line"; done

    if [[ ${overall_status} -ne 0 ]]; then
        log_warn "One or more stress-ng tests failed or did not produce a valid result."
    else
        log_success "Threads & System Stress benchmarks (stress-ng) completed."
    fi
    return ${overall_status}
}
network_benchmarks() {
    log_info "Starting Network benchmarks..."
    local overall_status=0 # 0 for success, 1 for any failure/skip

    # --- Local Speed Test ---
    log_header "Local Speed Test" "${BLUE}"
    local speedtest_tool_found=""
    local speedtest_download="N/A"
    local speedtest_upload="N/A"
    local speedtest_latency="N/A"

    # Check for speedtest-cli
    if command -v speedtest-cli &> /dev/null; then
        speedtest_tool_found="speedtest-cli"
        log_info "Using 'speedtest-cli' for local speed test."
        local speedtest_output
        if speedtest_output=$(speedtest-cli --secure --simple 2>&1); then
            log_info "speedtest-cli output:"
            echo "${speedtest_output}" | while IFS= read -r line; do log_info "  ${line}"; done
            speedtest_ping=$(echo "${speedtest_output}" | grep -oP 'Ping: \K[0-9.]+')
            speedtest_download=$(echo "${speedtest_output}" | grep -oP 'Download: \K[0-9.]+')
            speedtest_upload=$(echo "${speedtest_output}" | grep -oP 'Upload: \K[0-9.]+')
            [[ -n "${speedtest_ping}" ]] && speedtest_latency="${speedtest_ping}"
            if [[ -z "${speedtest_download}" || -z "${speedtest_upload}" ]]; then
                log_warn "Could not parse Download/Upload speeds from speedtest-cli."
                speedtest_download="N/A"; speedtest_upload="N/A"; speedtest_latency="N/A"
                overall_status=1
            fi
        else
            log_error "'speedtest-cli' command failed. Output:"
            echo "${speedtest_output}" | while IFS= read -r line; do log_error "  ${line}"; done
            overall_status=1
        fi
    elif command -v fast &> /dev/null; then
        speedtest_tool_found="fast-cli"
        log_info "Using 'fast' (fast-cli) for local speed test."
        if ! command -v jq &> /dev/null; then
            log_error "'jq' is required for parsing 'fast-cli' output but not found. Skipping fast-cli test."
            overall_status=1
        else
            local fast_output
            # fast-cli might sometimes hang or take very long, add a timeout
            if fast_output=$(timeout 120s fast --upload --json 2>&1); then
                log_info "fast-cli output: ${fast_output}" # Log raw JSON for debugging
                # Validate JSON structure before parsing
                if ! echo "${fast_output}" | jq empty 2>/dev/null; then
                    log_error "fast-cli output is not valid JSON. Output: ${fast_output}"
                    overall_status=1
                else
                    speedtest_download=$(echo "${fast_output}" | jq -r '.downloadSpeed // "N/A"')
                    speedtest_upload=$(echo "${fast_output}" | jq -r '.uploadSpeed // "N/A"')
                    # fast-cli JSON does not typically include latency.
                    speedtest_latency="N/A (fast-cli)"

                    if [[ "${speedtest_download}" == "N/A" || "${speedtest_upload}" == "N/A" ]]; then
                        log_warn "Could not parse Download/Upload speeds from fast-cli JSON."
                        overall_status=1
                    fi
                fi
            else
                local fast_exit_code=$?
                if [[ $fast_exit_code -eq 124 ]]; then # Timeout
                    log_error "'fast --upload --json' timed out after 120 seconds."
                else
                    log_error "'fast --upload --json' command failed with exit code ${fast_exit_code}. Output:"
                    echo "${fast_output}" | while IFS= read -r line; do log_error "  ${line}"; done
                fi
                overall_status=1
            fi
        fi
    else
        log_warn "Neither 'speedtest-cli' nor 'fast' (fast-cli) found. Skipping local speed test."
        log_info "Consider installing: 'sudo apt install speedtest-cli' or 'npm install --global fast-cli'"
        overall_status=1 # Mark as skip/failure for this sub-test
    fi

    log_info "${BLUE}Local Speed Test Results:${NC}"
    log_info "${BLUE}--------------------------------------------------${NC}"
    printf "%-20s | %s\n" "Tool Used" "${speedtest_tool_found:-Not Found}" | while IFS= read -r line; do log_info "$line"; done
    printf "%-20s | %s Mbps\n" "Download Speed" "${speedtest_download}" | while IFS= read -r line; do log_info "$line"; done
    printf "%-20s | %s Mbps\n" "Upload Speed" "${speedtest_upload}" | while IFS= read -r line; do log_info "$line"; done
    printf "%-20s | %s ms\n" "Latency" "${speedtest_latency}" | while IFS= read -r line; do log_info "$line"; done
    log_info "${BLUE}--------------------------------------------------${NC}"
    printf "\n"

    # --- iperf3 Single Server Tests ---
    log_header "iperf3 Single Server Tests" "${BLUE}"
    local iperf3_available=0
    if ! command -v iperf3 &> /dev/null; then
        log_warn "'iperf3' command not found. Skipping iperf3 tests."
        log_info "Consider installing: 'sudo apt install iperf3'"
        overall_status=1
    else
        iperf3_available=1
        if ! command -v curl &> /dev/null || ! command -v jq &> /dev/null; then
            log_error "'curl' and 'jq' are required for iperf3 server list fetching/parsing but not found. Skipping iperf3 tests."
            iperf3_available=0
            overall_status=1
        fi
    fi

    if [[ ${iperf3_available} -eq 1 ]]; then
        local public_iperf_servers_json
        log_info "Fetching public iperf3 servers from iperf.fr API..."
        # Fetch top 3-5 servers. Using 3 for brevity in this example.
        if public_iperf_servers_json=$(curl -s --connect-timeout 10 "https://iperf.fr/api/best-servers?topN=3" 2>&1); then
            if ! echo "${public_iperf_servers_json}" | jq empty 2>/dev/null; then
                log_error "Failed to fetch or parse iperf3 server list from API. Output was not valid JSON:"
                echo "${public_iperf_servers_json}" | while IFS= read -r line; do log_error "  ${line}"; done
                public_iperf_servers_json="" # Clear to use fallback
            else
                 log_info "Successfully fetched server list."
                 # log_info "Server list JSON: ${public_iperf_servers_json}" # For debugging
            fi
        else
            log_error "curl command failed to fetch iperf3 server list. Error:"
            echo "${public_iperf_servers_json}" | while IFS= read -r line; do log_error "  ${line}"; done
            public_iperf_servers_json="" # Clear to use fallback
        fi

        declare -a iperf_servers_to_test=()
        if [[ -n "${public_iperf_servers_json}" ]]; then
            # Map JSON to an array of "host:port Description"
            # Assuming structure like: [{"host": "...", "port": ..., "city": "...", "country": "..."}]
            local server_count
            server_count=$(echo "${public_iperf_servers_json}" | jq '. | length')
            if [[ "${server_count}" -gt 0 ]]; then
                for i in $(seq 0 $((server_count - 1))); do
                    local host city country port
                    host=$(echo "${public_iperf_servers_json}" | jq -r ".[${i}].host")
                    port=$(echo "${public_iperf_servers_json}" | jq -r ".[${i}].port")
                    city=$(echo "${public_iperf_servers_json}" | jq -r ".[${i}].city")
                    country=$(echo "${public_iperf_servers_json}" | jq -r ".[${i}].country")
                    if [[ "${host}" != "null" && "${port}" != "null" ]]; then
                        iperf_servers_to_test+=("${host}:${port} ${city}, ${country}")
                    fi
                done
            fi
        fi

        # Fallback hardcoded list if API fails or returns no servers
        if [[ ${#iperf_servers_to_test[@]} -eq 0 ]]; then
            log_warn "API fetch for iperf3 servers failed or returned no servers. Using hardcoded fallback list."
            iperf_servers_to_test=(
                "bouygues.iperf.fr:5201 Bouygues Telecom, Paris, FR"
                "iperf.he.net:5201 Hurricane Electric, Fremont, CA, US"
                "speedtest.serverius.net:5201 Serverius, Netherlands, NL"
                "ping.online.net:5201 Online.net, France" # Scaleway/Online.net
                "iperf.scottlinux.com:5201 ScottLinux, Dallas, TX, US"
            )
        fi

        log_info "iperf3 servers to test:"
        for server_desc in "${iperf_servers_to_test[@]}"; do
            log_info "  - ${server_desc}"
        done
        printf "\n"

        log_info "${BLUE}iperf3 Single Server Test Results:${NC}"
        log_info "${BLUE}--------------------------------------------------------------------------${NC}"
        printf "%-35s | %-5s | %-15s | %-15s\n" "Server Location/Host" "Port" "Download (Mbps)" "Upload (Mbps)" | while IFS= read -r line; do log_info "${CYAN}$line${NC}"; done
        log_info "${BLUE}--------------------------------------------------------------------------${NC}"

        for server_entry in "${iperf_servers_to_test[@]}"; do
            local server_host_port="${server_entry%% *}" # e.g., bouygues.iperf.fr:5201
            local server_desc="${server_entry#* }"     # e.g., Bouygues Telecom, Paris, FR
            local server_host="${server_host_port%:*}"
            local server_port="${server_host_port#*:}"

            local iperf_dl_speed="N/A"
            local iperf_ul_speed="N/A"
            local test_failed_for_server=0

            log_info "Testing iperf3 against: ${server_desc} (${server_host}:${server_port})"

            # Download Test
            log_info "  Running iperf3 Download test..."
            local iperf_dl_output
            # Timeout for iperf3 command itself, e.g., 30s for a 10s test + connection setup
            if iperf_dl_output=$(timeout 30s iperf3 -c "${server_host}" -p "${server_port}" -P 8 -t 10 -J 2>&1); then
                if ! echo "${iperf_dl_output}" | jq empty 2>/dev/null; then
                    log_warn "  iperf3 Download JSON output for ${server_host} is invalid."
                    echo "${iperf_dl_output}" | while IFS= read -r line; do log_warn "    ${line}"; done
                    test_failed_for_server=1
                else
                    local dl_bits_per_second
                    dl_bits_per_second=$(echo "${iperf_dl_output}" | jq -r '.end.sum_received.bits_per_second // 0')
                    if [[ "${dl_bits_per_second}" == "0" || -z "${dl_bits_per_second}" ]]; then
                        log_warn "  Could not parse download bits_per_second from iperf3 JSON for ${server_host}."
                        # Check for error field in JSON
                        local iperf_error
                        iperf_error=$(echo "${iperf_dl_output}" | jq -r '.error // ""')
                        if [[ -n "${iperf_error}" ]]; then
                            log_warn "  iperf3 reported error: ${iperf_error}"
                        fi
                        test_failed_for_server=1
                    else
                        iperf_dl_speed=$(awk -v bps="${dl_bits_per_second}" 'BEGIN { printf "%.2f", bps / 1000000 }')
                    fi
                fi
            else
                local iperf_exit_code=$?
                if [[ $iperf_exit_code -eq 124 ]]; then # Timeout
                    log_error "  iperf3 Download test for ${server_host} timed out."
                else
                    log_error "  iperf3 Download test for ${server_host} failed (exit code ${iperf_exit_code}). Output:"
                    echo "${iperf_dl_output}" | while IFS= read -r line; do log_error "    ${line}"; done
                fi
                test_failed_for_server=1
            fi

            # Upload Test
            log_info "  Running iperf3 Upload test..."
            local iperf_ul_output
            if iperf_ul_output=$(timeout 30s iperf3 -c "${server_host}" -p "${server_port}" -P 8 -t 10 -R -J 2>&1); then
                if ! echo "${iperf_ul_output}" | jq empty 2>/dev/null; then
                    log_warn "  iperf3 Upload JSON output for ${server_host} is invalid."
                    echo "${iperf_ul_output}" | while IFS= read -r line; do log_warn "    ${line}"; done
                    test_failed_for_server=1
                else
                    local ul_bits_per_second
                    ul_bits_per_second=$(echo "${iperf_ul_output}" | jq -r '.end.sum_sent.bits_per_second // 0')
                     if [[ "${ul_bits_per_second}" == "0" || -z "${ul_bits_per_second}" ]]; then
                        log_warn "  Could not parse upload bits_per_second from iperf3 JSON for ${server_host}."
                        local iperf_error
                        iperf_error=$(echo "${iperf_ul_output}" | jq -r '.error // ""')
                        if [[ -n "${iperf_error}" ]]; then
                            log_warn "  iperf3 reported error: ${iperf_error}"
                        fi
                        test_failed_for_server=1
                    else
                        iperf_ul_speed=$(awk -v bps="${ul_bits_per_second}" 'BEGIN { printf "%.2f", bps / 1000000 }')
                    fi
                fi
            else
                local iperf_exit_code=$?
                 if [[ $iperf_exit_code -eq 124 ]]; then # Timeout
                    log_error "  iperf3 Upload test for ${server_host} timed out."
                else
                    log_error "  iperf3 Upload test for ${server_host} failed (exit code ${iperf_exit_code}). Output:"
                    echo "${iperf_ul_output}" | while IFS= read -r line; do log_error "    ${line}"; done
                fi
                test_failed_for_server=1
            fi

            printf "%-35s | %-5s | %-15s | %-15s\n" "${server_desc:0:33}" "${server_port}" "${iperf_dl_speed}" "${iperf_ul_speed}" | while IFS= read -r line; do log_info "$line"; done
            if [[ ${test_failed_for_server} -ne 0 ]]; then
                overall_status=1 # Mark that at least one iperf3 test had issues
            fi
        done
        log_info "${BLUE}--------------------------------------------------------------------------${NC}"
        printf "\n"
    fi # end iperf3_available check

    if ! ${SKIP_NETBLAST}; then
        log_info "--- Running hyprbench-netblast: Advanced Multi-Server Network Tests ---"
        if [ -x "./hyprbench-netblast.sh" ]; then
            # Capture output, handling potential errors from netblast itself
            # Since script runs as root, netblast will also run as root if it uses sudo internally.
            # Or, ensure netblast.sh also has its sudo calls removed if it's part of this suite.
            # For now, assuming netblast.sh handles its own permissions or doesn't need special perms beyond HyprBench's root.
            if netblast_output=$(./hyprbench-netblast.sh 2>&1); then
                log_info "hyprbench-netblast Results:"
                # Log each line of the output to preserve formatting
                while IFS= read -r line; do
                    log_info "$line" # This will use HyprBench's log_info
                done <<< "$netblast_output"
            else
                log_error "hyprbench-netblast.sh encountered an error during execution."
                log_info "Captured output from hyprbench-netblast.sh:"
                while IFS= read -r line; do
                    log_info "$line"
                done <<< "$netblast_output" # Log output even on error
                overall_status=1 # Mark that this sub-test had issues
            fi
        else
            log_warn "./hyprbench-netblast.sh not found or not executable. Skipping advanced network tests."
            overall_status=1 # Mark that this sub-test was skipped/failed
        fi
    else
        log_warn "Skipping hyprbench-netblast.sh as per --skip-netblast option."
        # No change to overall_status here, as skipping is intentional for this part.
        # The main SKIP_NETWORK flag would handle the broader skip.
    fi
    printf "\n" # Add a newline for better readability after this section
    if [[ ${overall_status} -ne 0 ]]; then
        log_warn "One or more network benchmark tests failed, timed out, or were skipped."
    else
        log_success "Network benchmarks completed."
    fi
    return ${overall_status}
}

public_reference_benchmarks() {
    log_info "Starting Public Reference Benchmarks (UnixBench via Phoronix Test Suite)..."
    local PTS_DIR="./phoronix-test-suite"
    local pts_executable="${PTS_DIR}/phoronix-test-suite"
    local overall_status=0 # 0 for success, 1 for warning/skip, 2 for error

    local unixbench_index_score="N/A"
    local unixbench_dhrystone_lps="N/A"
    local unixbench_whetstone_mwips="N/A"

    # --- PTS Check ---
    if [[ ! -x "${pts_executable}" ]]; then
        log_warn "Phoronix Test Suite not found at ${PTS_DIR} or not executable."
        log_warn "UnixBench (Optional) will be skipped. This is not an error if you haven't set up PTS."
        log_warn "If you intend to run memory or other PTS-based benchmarks, ensure PTS is installed (e.g., by running memory_benchmarks)."
        overall_status=1 # Mark as skipped
    else
        log_info "Phoronix Test Suite found at ${PTS_DIR}."
        log_info "Running UnixBench via Phoronix Test Suite (this may take a long time, 30-60+ minutes)..."

        local unixbench_output
        local benchmark_failed=0

        # Ensure ~/.phoronix-test-suite exists and is writable, PTS usually handles this.
        mkdir -p ~/.phoronix-test-suite

        if unixbench_output=$("${pts_executable}" batch-run pts/unixbench 2>&1); then
            log_info "UnixBench 'batch-run pts/unixbench' command executed. Parsing results..."

            # Parse results from STDOUT
            # Example lines:
            #   pts/unixbench:
            #       System Benchmarks Index Score        : XXXX.X
            #       Dhrystone 2 using register variables : YYYYYYYY.Y lps
            #       Double-Precision Whetstone           : ZZZZ.Z MWIPS

            # System Benchmarks Index Score
            local raw_index
            raw_index=$(echo "${unixbench_output}" | grep "System Benchmarks Index Score" | awk -F': ' '{print $NF}' | tr -d '[:space:]')
            if [[ -n "${raw_index}" && "${raw_index}" != "N/A" ]]; then
                unixbench_index_score="${raw_index}"
            fi

            # Dhrystone 2 using register variables
            local raw_dhrystone
            raw_dhrystone=$(echo "${unixbench_output}" | grep "Dhrystone 2 using register variables" | awk -F': ' '{print $NF}' | sed 's/ lps//' | tr -d '[:space:]')
             if [[ -n "${raw_dhrystone}" && "${raw_dhrystone}" != "N/A" ]]; then
                unixbench_dhrystone_lps="${raw_dhrystone}"
            fi

            # Double-Precision Whetstone
            local raw_whetstone
            raw_whetstone=$(echo "${unixbench_output}" | grep "Double-Precision Whetstone" | awk -F': ' '{print $NF}' | sed 's/ MWIPS//' | tr -d '[:space:]')
            if [[ -n "${raw_whetstone}" && "${raw_whetstone}" != "N/A" ]]; then
                unixbench_whetstone_mwips="${raw_whetstone}"
            fi

            if [[ "${unixbench_index_score}" == "N/A" && "${unixbench_dhrystone_lps}" == "N/A" && "${unixbench_whetstone_mwips}" == "N/A" ]]; then
                log_warn "Failed to parse any UnixBench results from standard output."
                log_info "Full UnixBench output:"
                echo "${unixbench_output}" | while IFS= read -r line; do log_info "  ${line}"; done
                benchmark_failed=1
                overall_status=2 # Mark as error in parsing
            elif [[ "${unixbench_index_score}" == "N/A" ]]; then
                 log_warn "Could not parse the main 'System Benchmarks Index Score' for UnixBench."
                 # We might still have component scores, so don't mark as total failure yet unless others also fail.
            else
                log_success "UnixBench results parsed."
            fi
        else
            log_error "Phoronix Test Suite 'batch-run pts/unixbench' command failed."
            log_info "Full command output:"
            echo "${unixbench_output}" | while IFS= read -r line; do log_error "  ${line}"; done
            benchmark_failed=1
            overall_status=2 # Mark as error in execution
        fi

        if [[ ${benchmark_failed} -eq 1 ]]; then
            log_warn "UnixBench run failed or results could not be parsed."
        fi
    fi

    # --- Reporting Results ---
    printf "\n" | while IFS= read -r line; do log_info "$line"; done
    log_info "${BLUE}UnixBench Results (via Phoronix Test Suite - Optional):${NC}"
    log_info "${BLUE}----------------------------------------------------------${NC}"
    printf "%-35s | %s\n" "Test Component" "Score" | while IFS= read -r line; do log_info "${CYAN}$line${NC}"; done
    log_info "${BLUE}----------------------------------------------------------${NC}"

    if [[ ${overall_status} -eq 1 ]]; then # PTS not found
        printf "%-35s | %s\n" "Status" "PTS Not Found, Skipped" | while IFS= read -r line; do log_info "$line"; done
    elif [[ ${overall_status} -eq 2 && "${unixbench_index_score}" == "N/A" ]]; then # Execution or major parsing error
        printf "%-35s | %s\n" "Status" "Run/Parse Failed" | while IFS= read -r line; do log_info "$line"; done
    else # Results (even if partial or N/A for some components)
        printf "%-35s | %s\n" "System Benchmarks Index Score" "${unixbench_index_score}" | while IFS= read -r line; do log_info "$line"; done
        printf "%-35s | %s lps\n" "Dhrystone 2" "${unixbench_dhrystone_lps}" | while IFS= read -r line; do log_info "$line"; done
        printf "%-35s | %s MWIPS\n" "Double-Precision Whetstone" "${unixbench_whetstone_mwips}" | while IFS= read -r line; do log_info "$line"; done
    fi
    log_info "${BLUE}----------------------------------------------------------${NC}"
    printf "\n" | while IFS= read -r line; do log_info "$line"; done

    if [[ ${overall_status} -eq 2 ]]; then
        log_warn "UnixBench benchmarks encountered errors or results are unavailable."
        return 1 # Indicate failure for this section if actual errors occurred
    elif [[ ${overall_status} -eq 1 ]]; then
        log_info "UnixBench benchmarks were skipped as PTS was not available."
        return 0 # Skipped is not an error for an optional test
    fi

    log_success "Public Reference Benchmarks (UnixBench) section finished."
    return 0
}


# --- System Information Gathering Function ---
gather_system_info() {
    log_info "Gathering CPU Information..."
    # CPU Information
    log_info "CPU Model: $(lscpu | grep "Model name:" | sed 's/Model name:[ \t]*//' || echo "N/A")"
    log_info "CPU Architecture: $(lscpu | grep "Architecture:" | sed 's/Architecture:[ \t]*//' || echo "N/A")"
    log_info "CPU(s): $(lscpu | grep "^CPU(s):" | sed 's/CPU(s):[ \t]*//' || echo "N/A")"
    log_info "Core(s) per socket: $(lscpu | grep "Core(s) per socket:" | sed 's/Core(s) per socket:[ \t]*//' || echo "N/A")"
    log_info "Thread(s) per core: $(lscpu | grep "Thread(s) per core:" | sed 's/Thread(s) per core:[ \t]*//' || echo "N/A")"
    log_info "Vendor ID: $(lscpu | grep "Vendor ID:" | sed 's/Vendor ID:[ \t]*//' || echo "N/A")"
    log_info "CPU max MHz: $(lscpu | grep "CPU max MHz:" | sed 's/CPU max MHz:[ \t]*//' || echo "N/A")"
    log_info "CPU min MHz: $(lscpu | grep "CPU min MHz:" | sed 's/CPU min MHz:[ \t]*//' || echo "N/A")"
    log_info "L1d cache: $(lscpu | grep "L1d cache:" | sed 's/L1d cache:[ \t]*//;s/  .*//' || echo "N/A")"
    log_info "L1i cache: $(lscpu | grep "L1i cache:" | sed 's/L1i cache:[ \t]*//;s/  .*//' || echo "N/A")"
    log_info "L2 cache: $(lscpu | grep "L2 cache:" | sed 's/L2 cache:[ \t]*//;s/  .*//' || echo "N/A")"
    log_info "L3 cache: $(lscpu | grep "L3 cache:" | sed 's/L3 cache:[ \t]*//;s/  .*//' || echo "N/A")"
    printf "\n"

    log_info "Gathering RAM Information..."
    # RAM Information (free)
    log_info "Memory Usage (from free -h):"
    free -h | grep -E "^Mem:|^Swap:" | while IFS= read -r line; do log_info "  $line"; done
    printf "\n"

    # RAM Information (dmidecode)
    # Script now requires root, so direct calls without sudo and no internal root check
    if command -v dmidecode &> /dev/null; then
        log_info "Memory Device Details (from dmidecode -t memory):"
        local mem_info
        mem_info=$(dmidecode -t memory 2>&1) # Capture stderr too
        if [[ $? -eq 0 && -n "$mem_info" && ! "$mem_info" =~ "No SMBIOS nor DMI entry point found" ]]; then
            local found_modules_count=0
            local awk_output
            awk_output=$(echo "$mem_info" | awk '
            BEGIN { RS="Memory Device"; FS="\n"; OFS=" | "; header_printed=0; idx_counter=0; } # Initialize idx_counter
            /Size: No Module Installed/ { next }
            /Size: Disabled/ { next }
            /Size: Not Populated/ { next }
            /Size: .*/ {
                size="N/A"; type="N/A"; speed="N/A"; manu="N/A"; part="N/A";
                for (i=1; i<=NF; i++) {
                    if ($i ~ /^\s*Size: /) { gsub(/^\s*Size: /, "", $i); size=$i }
                    if ($i ~ /^\s*Type: /) { gsub(/^\s*Type: /, "", $i); type=$i }
                    if ($i ~ /^\s*Speed: /) { gsub(/^\s*Speed: /, "", $i); speed=$i }
                    if ($i ~ /^\s*Manufacturer: /) { gsub(/^\s*Manufacturer: /, "", $i); manu=$i }
                    if ($i ~ /^\s*Part Number: /) { gsub(/^\s*Part Number: /, "", $i); part=$i }
                }
                gsub(/^[ \t]+|[ \t]+$/, "", size);
                gsub(/^[ \t]+|[ \t]+$/, "", type);
                gsub(/^[ \t]+|[ \t]+$/, "", speed);
                gsub(/^[ \t]+|[ \t]+$/, "", manu);
                gsub(/^[ \t]+|[ \t]+$/, "", part);

                if (size != "N/A" && size != "0 B" && size != "Unknown") { # Process only if size is meaningful
                    if (!header_printed) {
                        print "  Idx | Size     | Type             | Speed          | Manufacturer     | Part Number";
                        print "  ----|----------|------------------|----------------|------------------|-----------------------";
                        header_printed=1;
                    }
                    idx_counter++;
                    printf "  %-3s | %-8s | %-16s | %-14s | %-16s | %s\n", idx_counter, size, type, speed, manu, part;
                    print "AWK_FOUND_MODULE";
                }
            }')

            echo "$awk_output" | grep -v "AWK_FOUND_MODULE" | while IFS= read -r line; do log_info "$line"; done
            found_modules_count=$(echo "$awk_output" | grep -c "AWK_FOUND_MODULE")

            if [[ "$found_modules_count" -eq 0 ]]; then
                log_info "  No populated memory module details found or parsed from dmidecode."
            fi
        else
            log_warn "  Could not retrieve detailed memory info using dmidecode. Output:"
            echo "$mem_info" | while IFS= read -r line; do log_warn "    $line"; done
        fi
    else
        log_warn "  dmidecode command not found, skipping detailed memory info."
    fi
    printf "\n"

    log_info "Gathering Motherboard/System Information..."
    # Motherboard/System Information
    if command -v dmidecode &> /dev/null; then
        log_info "System Manufacturer: $(dmidecode -s system-manufacturer 2>/dev/null || echo "N/A")"
        log_info "System Product Name: $(dmidecode -s system-product-name 2>/dev/null || echo "N/A")"
        log_info "System Version: $(dmidecode -s system-version 2>/dev/null || echo "N/A")"
        log_info "System Serial Number: $(dmidecode -s system-serial-number 2>/dev/null || echo "N/A")"
    else
        log_warn "  dmidecode command not found, skipping system details."
    fi
    printf "\n"

    log_info "Gathering OS Information..."
    # OS Information
    log_info "Kernel Version: $(uname -r || echo "N/A")"
    if [ -f /etc/os-release ]; then
        # Source os-release to get variables like PRETTY_NAME
        # shellcheck disable=SC1091
        . /etc/os-release
        log_info "OS Name: ${NAME:-N/A}"
        log_info "OS Version: ${VERSION_ID:-N/A}"
        log_info "OS Pretty Name: ${PRETTY_NAME:-N/A}"
    elif command -v lsb_release &> /dev/null; then
        log_info "OS Description: $(lsb_release -ds 2>/dev/null || echo "N/A")"
        log_info "OS Release: $(lsb_release -rs 2>/dev/null || echo "N/A")"
        log_info "OS Codename: $(lsb_release -cs 2>/dev/null || echo "N/A")"
    else
        log_warn "  Could not determine OS release information (/etc/os-release and lsb_release not found)."
    fi
    printf "\n"

    log_info "Gathering Storage Overview..."
    # Storage Overview
    if command -v lsblk &> /dev/null; then
        log_info "Storage Devices (lsblk -bno NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE,ROTA):"
        # Header for the table
        printf "%s\n" "  Name         Size (Bytes)  Type      Mountpoint     FSType    ROTA (0=SSD,1=HDD)" | while IFS= read -r line; do log_info "$line"; done
        printf "%s\n" "  ------------ ------------- --------- -------------- --------- -----------------" | while IFS= read -r line; do log_info "$line"; done
        
        # Process lsblk output
        lsblk -bno NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE,ROTA 2>/dev/null | while IFS= read -r line || [[ -n "$line" ]]; do # Process last line even if no newline
            # shellcheck disable=SC2086 # We want word splitting here
            set -- $line # Split line into positional parameters
            local name="${1:--}" size="${2:--}" type="${3:--}" mountpoint="${4:--}" fstype="${5:--}" rota="${6:--}"
            
            # Ensure rota is displayed correctly if missing or invalid
            local rota_display="N/A"
            if [[ "$rota" == "0" ]]; then
                rota_display="0 (SSD/NVMe)"
            elif [[ "$rota" == "1" ]]; then
                rota_display="1 (HDD)"
            elif [[ "$rota" != "-" ]]; then # If rota has some other value
                rota_display="$rota (Unknown)"
            fi

            printf "  %-12s %-13s %-9s %-14s %-9s %s\n" "$name" "$size" "$type" "$mountpoint" "$fstype" "$rota_display" | while IFS= read -r log_line; do log_info "$log_line"; done
        done
        if ! lsblk -bno NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE,ROTA 2>/dev/null | grep -q '.'; then # Check if output has any character
            log_info "  No storage devices found by lsblk or output was empty."
        fi
    else
        log_warn "  lsblk command not found, skipping storage overview."
    fi
    printf "\n"

    log_info "Gathering NVMe Specifics..."
    # NVMe Specifics
    if command -v lspci &> /dev/null; then
        log_info "NVMe Controllers (lspci | grep -i nvme):"
        local lspci_nvme_output
        lspci_nvme_output=$(lspci | grep -i 'nvme' 2>/dev/null)
        if [[ -n "$lspci_nvme_output" ]]; then
            echo "$lspci_nvme_output" | while IFS= read -r line; do log_info "  $line"; done
        else
            log_info "  No NVMe controllers found via lspci."
        fi
    else
        log_warn "  lspci command not found, skipping NVMe controller check."
    fi

    if command -v lsblk &> /dev/null; then
        log_info "Detected NVMe Block Devices (lsblk -dno NAME | grep nvme):"
        local lsblk_nvme_output
        lsblk_nvme_output=$(lsblk -dno NAME 2>/dev/null | grep 'nvme')
        if [[ -n "$lsblk_nvme_output" ]]; then
            echo "$lsblk_nvme_output" | while IFS= read -r line; do log_info "  /dev/$line"; done
        else
            log_info "  No NVMe block devices found via lsblk."
        fi
    else
        log_warn "  lsblk command not found, skipping NVMe block device check."
    fi
    printf "\n"
}

# --- Main Script Body ---
main() {
    # Argument parsing
    while [[ $# -gt 0 ]]; do
        key="$1"
        case $key in
            -h|--help)
            display_help
            exit 0
            ;;
            --log-file)
            ARG_LOG_FILE="$2"
            shift # past argument
            shift # past value
            ;;
            --temp-dir)
            ARG_TEMP_DIR="$2"
            shift # past argument
            shift # past value
            ;;
            --skip-cpu)
            SKIP_CPU=true
            shift # past argument
            ;;
            --skip-memory)
            SKIP_MEMORY=true
            shift # past argument
            ;;
            --skip-disk)
            SKIP_DISK=true
            shift # past argument
            ;;
            --skip-stress)
            SKIP_STRESS=true
            shift # past argument
            ;;
            --skip-network)
            SKIP_NETWORK=true
            shift # past argument
            ;;
            --skip-netblast)
            SKIP_NETBLAST=true
            shift # past argument
            ;;
            --skip-public-ref)
            SKIP_PUBLIC_REF=true
            shift # past argument
            ;;
            --fio-target-dir)
            ARG_FIO_TARGET_DIR="$2"
            shift # past argument
            shift # past value
            ;;
            --fio-test-size)
            ARG_FIO_TEST_SIZE="$2"
            shift # past argument
            shift # past value
            ;;
            *)    # unknown option
            # Not using log_error here as LOG_FILE might not be set yet
            echo -e "${RED}[ERROR] Unknown option: $1${NC}" >&2
            display_help
            exit 1
            ;;
        esac
    done

    # Initialize LOG_FILE (must happen AFTER argument parsing)
    if [[ -n "${ARG_LOG_FILE}" ]]; then
        LOG_FILE="${ARG_LOG_FILE}"
        local log_dir
        log_dir=$(dirname "${LOG_FILE}")
        # Check if log_dir is not "." or empty, then try to create
        if [[ -n "${log_dir}" && "${log_dir}" != "." ]]; then
             if ! mkdir -p "${log_dir}"; then
                echo -e "${RED}[ERROR] Failed to create custom log directory: ${log_dir}${NC}" >&2
                exit 1;
             fi
        fi
    else
        if ! mkdir -p ./logs; then
            echo -e "${RED}[ERROR] Failed to create default log directory: ./logs${NC}" >&2
            exit 1;
        fi
        LOG_FILE="./logs/hyprbench-$(date +%Y%m%d-%H%M%S).log"
    fi
    # Now that LOG_FILE is set, we can use proper logging for TEMP_DIR errors.

    # Initialize TEMP_DIR (must happen AFTER argument parsing)
    if [[ -n "${ARG_TEMP_DIR}" ]]; then
        TEMP_DIR="${ARG_TEMP_DIR}"
        if ! mkdir -p "${TEMP_DIR}"; then
            log_error "Failed to create custom temp directory: ${TEMP_DIR}" # log_error now works
            exit 1;
        fi
    else
        # Create temp dir in current directory to avoid /tmp issues on some systems / permissions
        TEMP_DIR=$(mktemp -d "$(pwd)/hyprbench.XXXXXX")
        if [[ ! -d "${TEMP_DIR}" ]]; then # Check if mktemp failed
            log_error "Failed to create temporary directory using mktemp."
            exit 1
        fi
    fi

    local start_script_time
    start_script_time=$(date +%s)

    # --- Main Report Header ---
    # (LOG_FILE is now defined, so header logging will work)
    log_info "======================================================================"
    log_info " HyprBench v${SCRIPT_VERSION} - Comprehensive System Benchmark"
    log_info "======================================================================"
    log_info "Date: $(date +"%Y-%m-%d %H:%M:%S %Z")"
    log_info "Hostname: $(hostname -f || hostname)"
    log_info "Log file: ${LOG_FILE}"
    log_info "Temp dir: ${TEMP_DIR}"
    log_info "======================================================================"
    log_info "" # Empty line for spacing

    # Strict Root check (must be after LOG_FILE is set for log_error to work)
    if [ "$(id -u)" -ne 0 ]; then
        log_error "This script must be run as root. Please use sudo."
        exit 1
    fi
    # No printf "\n" needed here, log_error adds newline.

    # Check core dependencies
    log_header "Checking Core Dependencies" "${CYAN}"
    check_dependency "lscpu"
    check_dependency "free"
    check_dependency "dmidecode" "dmidecode" # package name might be the same
    check_dependency "lsb_release" "lsb-release" # package name often lsb-release or similar
    check_dependency "lspci" "pciutils"
    check_dependency "lsblk" "util-linux" # often part of util-linux
    check_dependency "nproc" "coreutils" # nproc is usually in coreutils
    check_dependency "sysbench"
    check_dependency "fio"
    check_dependency "jq" # Added for FIO JSON parsing
    check_dependency "git"
    check_dependency "php" "php-cli" # php-cli is often the package for the PHP CLI
    check_dependency "php-xml" "php-xml" # Or phpX.Y-xml depending on the system
    check_dependency "stress-ng" "stress-ng"
    check_dependency "curl" "curl" # For iperf.fr API and potentially other network tasks
    # For network_benchmarks
    check_dependency "iperf3" "iperf3"
    # speedtest-cli and fast are checked within network_benchmarks, but we ensure they *could* be installed
    # We don't want the script to exit if only one of them is missing, as the other might be present.
    # So, we'll use a softer check or rely on the internal check in network_benchmarks.
    # For now, let's list them so user is aware. The script won't exit if one is missing if the other is found by the function.
    # A more robust check_dependency could handle "OR" cases or optional dependencies.
    # Simple check for awareness:
    if ! command -v speedtest-cli &>/dev/null && ! command -v speedtest &>/dev/null; then
        log_warn "Optional: 'speedtest-cli' (Ookla) not found. Install with 'sudo apt install speedtest-cli' or from official site."
    else
        # Check which one it is if we want to pass the exact command name to check_dependency
        # For now, this warning is sufficient.
        log_info "Dependency check: 'speedtest-cli' or 'speedtest' seems available."
    fi
    if ! command -v fast &>/dev/null; then
        log_warn "Optional: 'fast' (fast-cli) not found. Install with 'npm install --global fast-cli'."
    else
        log_info "Dependency check: 'fast' (fast-cli) is available."
    fi
    # Add more critical dependencies as needed
    log_info "-----------------------------------------------------"
    printf "\n"

    # Run benchmark sections
    # System info always runs, not skippable by flag
    run_test_section "System Information" "gather_system_info"

    if ! ${SKIP_CPU}; then
        run_test_section "CPU Benchmarks" "cpu_benchmarks"
    else
        log_warn "Skipping CPU benchmarks as per --skip-cpu option."
        printf "\n" # Add spacing like run_test_section would
    fi

    if ! ${SKIP_MEMORY}; then
        run_test_section "Memory Benchmarks" "memory_benchmarks"
    else
        log_warn "Skipping Memory benchmarks as per --skip-memory option."
        printf "\n"
    fi

    if ! ${SKIP_DISK}; then
        run_test_section "Disk I/O Benchmarks (FIO)" "disk_benchmarks"
    else
        log_warn "Skipping Disk I/O benchmarks as per --skip-disk option."
        printf "\n"
    fi

    if ! ${SKIP_STRESS}; then
        run_test_section "Threads & System Stress (stress-ng)" "stress_ng_benchmarks"
    else
        log_warn "Skipping Threads & System Stress benchmarks as per --skip-stress option."
        printf "\n"
    fi

    if ! ${SKIP_NETWORK}; then
        run_test_section "Network Benchmarks" "network_benchmarks"
    else
        log_warn "Skipping all Network benchmarks as per --skip-network option."
        printf "\n"
    fi

    # Note: SKIP_NETBLAST is handled inside network_benchmarks function

    if ! ${SKIP_PUBLIC_REF}; then
        run_test_section "Public Reference Benchmarks (Optional)" "public_reference_benchmarks"
    else
        log_warn "Skipping Public Reference benchmarks as per --skip-public-ref option."
        printf "\n"
    fi

    # --- Main Report Footer ---
    local end_time
    end_time=$(date +%s)
    local duration=$((end_time - start_script_time))

    log_info "" # Empty line
    log_info "======================================================================"
    log_info " HyprBench Test Suite Finished"
    log_info "----------------------------------------------------------------------"
    # log_info "Total Test Duration: ${duration} seconds"
    log_info "Total Test Duration: $(printf '%02d:%02d:%02d' $((duration/3600)) $(( (duration/60)%60)) $((duration%60)) ) (H:M:S)"
    log_info "Full log available at: $LOG_FILE"
    log_info "======================================================================"

    log_success "HyprBench Script v${SCRIPT_VERSION} processing complete."
}

# --- Script Entry Point ---
# Call main function if the script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi