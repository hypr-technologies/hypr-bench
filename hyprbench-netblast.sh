#!/usr/bin/env bash
set -eo pipefail -u

# === CONFIGURATION ===
IPERF_DURATION=10
IPERF_PARALLEL_STREAMS=8
MAX_SERVERS_PER_REGION=2
MAX_DISTANCE_KM=2000
MIN_BANDWIDTH_MBPS=100

# === COLORS ===
COLOR_RESET="\033[0m"
COLOR_RED="\033[0;31m"
COLOR_GREEN="\033[0;32m"
COLOR_YELLOW="\033[0;33m"
COLOR_BLUE="\033[0;34m"
COLOR_PURPLE="\033[0;35m"
COLOR_CYAN="\033[0;36m"
COLOR_WHITE="\033[0;37m"
COLOR_BOLD="\033[1m"

# === LOGGING ===
_log() {
    local color_prefix="$1"
    local level="$2"
    local message="$3"
    echo -e "${color_prefix}${COLOR_BOLD}[$(date '+%Y-%m-%d %H:%M:%S')] [${level}]${COLOR_RESET} ${message}"
}

log_info() {
    _log "${COLOR_GREEN}" "INFO" "$1"
}

log_warn() {
    _log "${COLOR_YELLOW}" "WARN" "$1"
}

log_error() {
    _log "${COLOR_RED}" "ERROR" "$1"
}

log_debug() {
    # Add a DEBUG flag check if you want to enable/disable debug logs
    # if [[ "${DEBUG:-0}" -eq 1 ]]; then
    _log "${COLOR_PURPLE}" "DEBUG" "$1"
    # fi
}

# === HELPER FUNCTIONS ===
get_user_location() {
    log_info "Fetching user's public IP and location from ipinfo.io..."
    local ip_info
    if ! ip_info=$(curl -s --connect-timeout 5 "https://ipinfo.io"); then
        log_error "Failed to fetch location data from ipinfo.io. Check internet connection."
        return 1
    fi

    if [[ -z "$ip_info" ]]; then
        log_error "Received empty response from ipinfo.io."
        return 1
    fi

    USER_LAT=$(echo "$ip_info" | jq -r '.loc | split(",")[0] // empty')
    USER_LON=$(echo "$ip_info" | jq -r '.loc | split(",")[1] // empty')
    USER_COUNTRY=$(echo "$ip_info" | jq -r '.country // empty')
    USER_CITY=$(echo "$ip_info" | jq -r '.city // empty') # Added city for better logging
    USER_IP=$(echo "$ip_info" | jq -r '.ip // empty')


    if [[ -z "$USER_LAT" || "$USER_LAT" == "null" || -z "$USER_LON" || "$USER_LON" == "null" || -z "$USER_COUNTRY" || "$USER_COUNTRY" == "null" ]]; then
        log_error "Could not parse latitude, longitude, or country from ipinfo.io response."
        log_debug "ipinfo.io response: $ip_info"
        return 1
    fi

    log_info "User IP: ${USER_IP}"
    log_info "User Location: ${USER_CITY}, ${USER_COUNTRY} (Lat: ${USER_LAT}, Lon: ${USER_LON})"
    return 0
}

fetch_iperf_servers() {
    log_info "Fetching iperf3 server list from iperf3serverlist.net..."
    local server_list_json
    if ! server_list_json=$(curl -s --connect-timeout 10 "https://iperf3serverlist.net/json/all_servers-export.json"); then
        log_error "Failed to fetch iperf3 server list. Check internet connection or source URL."
        return 1
    fi

    if [[ -z "$server_list_json" ]]; then
        log_error "Received empty response from iperf3 server list provider."
        return 1
    fi

    # Basic JSON validation
    if ! echo "$server_list_json" | jq -e . > /dev/null 2>&1; then
        log_error "Fetched iperf3 server list is not valid JSON."
        log_debug "Raw server list response: $server_list_json"
        return 1
    fi
    
    # Store the raw JSON in a global or pass it back. For now, let's make it available to the caller.
    IPERF_SERVER_LIST_JSON="$server_list_json"
    log_info "Successfully fetched and validated iperf3 server list."
    return 0
}

# Haversine distance calculation
# Usage: haversine_distance lat1 lon1 lat2 lon2
# Returns distance in KM
haversine_distance() {
    local lat1_rad lon1_rad lat2_rad lon2_rad delta_lat delta_lon a c R
    R=6371 # Earth radius in KM

    # Convert degrees to radians
    lat1_rad=$(echo "$1 * 0.017453292519943295" | bc -l) # PI / 180
    lon1_rad=$(echo "$2 * 0.017453292519943295" | bc -l)
    lat2_rad=$(echo "$3 * 0.017453292519943295" | bc -l)
    lon2_rad=$(echo "$4 * 0.017453292519943295" | bc -l)

    delta_lat=$(echo "$lat2_rad - $lat1_rad" | bc -l)
    delta_lon=$(echo "$lon2_rad - $lon1_rad" | bc -l)

    # Haversine formula
    # a = sin²(Δlat/2) + cos(lat1) ⋅ cos(lat2) ⋅ sin²(Δlon/2)
    # c = 2 ⋅ atan2( √a, √(1−a) )
    # d = R ⋅ c
    # bc doesn't have asin or atan2 directly, using alternative for atan2(y,x) = atan(y/x) with quadrant checks,
    # but for Haversine, simpler: 2 * a(s(sqrt(a))) where a is angle, s is sine, sqrt is square root
    # Using a simpler form for `a` and then `c = 2 * atan2(sqrt(a), sqrt(1-a))`
    # For bc: `a(x)` is arctan(x), `s(x)` is sin(x), `c(x)` is cos(x), `l(x)` is ln(x), `e(x)` is exp(x)
    # atan2(y,x) can be derived: if x > 0, atan(y/x); if x < 0 and y >= 0, atan(y/x) + pi; if x < 0 and y < 0, atan(y/x) - pi;
    # if x = 0 and y > 0, pi/2; if x = 0 and y < 0, -pi/2.
    # However, for Haversine, `2 * a(sqrt(A)/sqrt(1-A))` is `2 * atan(sqrt(A)/sqrt(1-A))`.
    # `A = s(delta_lat/2)^2 + c(lat1_rad)*c(lat2_rad)*s(delta_lon/2)^2`

    local s_delta_lat_half s_delta_lon_half
    s_delta_lat_half=$(echo "s($delta_lat / 2)" | bc -l)
    s_delta_lon_half=$(echo "s($delta_lon / 2)" | bc -l)
    
    local cos_lat1 cos_lat2
    cos_lat1=$(echo "c($lat1_rad)" | bc -l)
    cos_lat2=$(echo "c($lat2_rad)" | bc -l)

    a=$(echo "($s_delta_lat_half * $s_delta_lat_half) + ($cos_lat1 * $cos_lat2 * $s_delta_lon_half * $s_delta_lon_half)" | bc -l)

    # Ensure 'a' is not > 1 due to precision, which would break sqrt(1-a)
    if (( $(echo "$a > 1" | bc -l) )); then a=1; fi

    # c = 2 * atan2(sqrt(a), sqrt(1-a))
    # For bc, atan2(y,x) is a bit complex. Using `2 * a(sqrt(a)/sqrt(1-a))` for `c`
    # Handle case where a=1 (distance is 180 degrees), 1-a = 0.
    if (( $(echo "$a == 1" | bc -l) )); then
        c=$(echo "scale=10; 4*a(1)" | bc -l) # pi
    elif (( $(echo "$a == 0" | bc -l) )); then
        c=0
    else
        c=$(echo "scale=10; 2 * a(sqrt($a)/sqrt(1-$a))" | bc -l)
    fi
    
    local distance
    distance=$(echo "$R * $c" | bc -l)
    printf "%.0f" "$distance" # Return integer KM
}


filter_and_select_servers() {
    log_info "Filtering and selecting iperf3 servers..."
    
    # Declare an array to hold the JSON string of each selected server
    SELECTED_SERVERS_JSON_ARRAY=()
    
    # Temporary associative array to count servers per region (country_code)
    declare -A servers_per_country_count

    # Read the server list using jq
    # Filter, sort by distance (primary) and bandwidth (secondary), then select per region
    # This jq command is complex. Let's break it down:
    # 1. Iterate over each server (.[]).
    # 2. Add user lat/lon to each server object for context (not strictly needed by haversine func but good for debug).
    # 3. Calculate distance (this is tricky to do purely in jq with external bash func).
    #    Alternative: iterate in bash, call haversine, then process with jq.
    #    Let's iterate in bash to call the haversine_distance function.

    local temp_server_list=() # Array to hold servers that pass initial filters (distance, bandwidth)

    log_debug "User location for filtering: Lat=${USER_LAT}, Lon=${USER_LON}"
    log_debug "Filtering criteria: Max Distance=${MAX_DISTANCE_KM}km, Min Bandwidth=${MIN_BANDWIDTH_MBPS}Mbps"

    # Use jq to iterate through the server list array
    # The input JSON is a single object with a key (like "1715500015") whose value is the array of servers.
    # First, extract the actual array of servers.
    local actual_server_array_json
    actual_server_array_json=$(echo "$IPERF_SERVER_LIST_JSON" | jq -c '.[keys[0]]')

    if [[ -z "$actual_server_array_json" || "$actual_server_array_json" == "null" ]]; then
        log_error "Could not extract server array from the fetched JSON."
        log_debug "IPERF_SERVER_LIST_JSON was: $IPERF_SERVER_LIST_JSON"
        return 1
    fi
    
    # Now iterate over this array
    local server_json
    echo "$actual_server_array_json" | jq -c '.[]' | while IFS= read -r server_json; do
        local host port latitude longitude country_code city advertised_bandwidth_bits advertised_bandwidth_mbps distance

        # Defensive parsing with defaults for numeric values if jq -r yields "null" or empty
        latitude=$(echo "$server_json" | jq -r '.latitude // "0"')
        longitude=$(echo "$server_json" | jq -r '.longitude // "0"')
        advertised_bandwidth_bits=$(echo "$server_json" | jq -r '.advertised_bandwidth // "0"')

        # Skip if essential geo-data is missing (after trying to default)
        if [[ "$latitude" == "0" && "$longitude" == "0" ]]; then # Crude check, assuming 0,0 is unlikely for a real server
            log_debug "Skipping server with missing/invalid geo-coordinates: $(echo "$server_json" | jq -r .host)"
            continue
        fi
        
        distance=$(haversine_distance "$USER_LAT" "$USER_LON" "$latitude" "$longitude")
        
        # Filter by distance
        if (( distance > MAX_DISTANCE_KM )); then
            log_debug "Server $(echo "$server_json" | jq -r .host) too far: ${distance}km > ${MAX_DISTANCE_KM}km"
            continue
        fi

        advertised_bandwidth_mbps=$(echo "scale=2; $advertised_bandwidth_bits / 1000000" | bc -l)
        
        # Filter by advertised bandwidth (ensure it's a valid number for comparison)
        if ! [[ "$advertised_bandwidth_mbps" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
             log_debug "Server $(echo "$server_json" | jq -r .host) has invalid advertised bandwidth: $advertised_bandwidth_mbps"
             continue
        fi
        if (( $(echo "$advertised_bandwidth_mbps < $MIN_BANDWIDTH_MBPS" | bc -l) )); then
            log_debug "Server $(echo "$server_json" | jq -r .host) below min bandwidth: ${advertised_bandwidth_mbps}Mbps < ${MIN_BANDWIDTH_MBPS}Mbps"
            continue
        fi

        # Server passed initial filters, add distance to its JSON and store
        # server_json_with_distance=$(echo "$server_json" | jq --argjson dist "$distance" '. + {calculated_distance: $dist, calculated_adv_bw_mbps: '"$advertised_bandwidth_mbps"'}')
        # A bit complex to pass float $advertised_bandwidth_mbps to jq --argjson if it's not int.
        # Let's use --arg for string and convert in jq, or just add distance.
        server_json_with_distance=$(echo "$server_json" | jq --argjson dist "$distance" --arg adv_bw_mbps "$advertised_bandwidth_mbps" \
            '. + {calculated_distance: $dist, calculated_adv_bw_mbps: ($adv_bw_mbps | tonumber) }')

        temp_server_list+=("$server_json_with_distance")
        log_debug "Provisionally selected server: $(echo "$server_json_with_distance" | jq -c '{host, city, country_code, calculated_distance, calculated_adv_bw_mbps}')"
    done
    
    log_info "Found ${#temp_server_list[@]} servers after distance and bandwidth filtering."

    if [[ ${#temp_server_list[@]} -eq 0 ]]; then
        log_warn "No servers found matching distance and bandwidth criteria."
        return 0 # Not an error, just no servers
    fi

    # Now, group by country_code and select MAX_SERVERS_PER_REGION from each
    # This is easier done by first creating a structure that jq can sort and group easily
    # Convert bash array of JSON strings to a single JSON array string for jq processing
    local combined_json_array="["
    for i in "${!temp_server_list[@]}"; do
        combined_json_array+="${temp_server_list[$i]}"
        if [[ $i -lt $((${#temp_server_list[@]} - 1)) ]]; then
            combined_json_array+=","
        fi
    done
    combined_json_array+="]"

    # Use jq to group by country_code, sort within groups, and take top N
    # Sorting: by distance (asc), then by calculated_adv_bw_mbps (desc)
    # .country_code can be null, handle this by assigning a default group e.g., "UNKNOWN"
    local final_selected_servers_json
    final_selected_servers_json=$(echo "$combined_json_array" | jq -c \
        --argjson max_per_region "$MAX_SERVERS_PER_REGION" \
        'group_by(.country_code // "UNKNOWN") | map(
            sort_by(.calculated_distance, .calculated_adv_bw_mbps_desc // (.calculated_adv_bw_mbps * -1)) # Sort by distance asc, bandwidth desc
            | .[:$max_per_region] # Take top N
        ) | flatten') # Flatten the array of arrays

    # Populate SELECTED_SERVERS_JSON_ARRAY from the jq output
    if [[ "$(echo "$final_selected_servers_json" | jq 'length')" -gt 0 ]]; then
        while IFS= read -r line; do
            SELECTED_SERVERS_JSON_ARRAY+=("$line")
        done < <(echo "$final_selected_servers_json" | jq -c '.[]')
    fi

    log_info "Selected ${#SELECTED_SERVERS_JSON_ARRAY[@]} servers after regional filtering:"
    for s_json in "${SELECTED_SERVERS_JSON_ARRAY[@]}"; do
        log_info "  - $(echo "$s_json" | jq -r '.host') in $(echo "$s_json" | jq -r '.city // "N/A"'), $(echo "$s_json" | jq -r '.country_code // "N/A"') (Dist: $(echo "$s_json" | jq -r .calculated_distance)km, BW: $(echo "$s_json" | jq -r .calculated_adv_bw_mbps)Mbps)"
    done
    
    if [[ ${#SELECTED_SERVERS_JSON_ARRAY[@]} -eq 0 ]]; then
        log_warn "No servers selected after all filtering stages."
    fi
    return 0
}

# Function to run a single iperf3 test and extract results
# Usage: run_single_iperf_test <host> <port> <direction_flag ("-R" for upload, "" for download)>
# Outputs: "throughput_mbps error_message" (error_message is empty on success)
# Returns: 0 on success, 1 on iperf3 error, 2 on JSON parsing error
run_single_iperf_test() {
    local host="$1"
    local port="$2"
    local direction_flag="$3" # -R for reverse (upload), empty for normal (download)
    local test_type_log

    if [[ -n "$direction_flag" ]]; then
        test_type_log="upload"
    else
        test_type_log="download"
    fi

    log_info "Starting ${test_type_log} test with ${host}:${port} (P=${IPERF_PARALLEL_STREAMS}, t=${IPERF_DURATION}s)..."

    local iperf_output iperf_exit_code
    # Run iperf3 and capture stdout, stderr, and exit code
    # Redirect stderr to stdout to capture iperf3 error messages if JSON is not produced.
    iperf_output=$(iperf3 -c "$host" -p "$port" -P "$IPERF_PARALLEL_STREAMS" -t "$IPERF_DURATION" $direction_flag -J 2>&1)
    iperf_exit_code=$?

    local throughput_mbps="N/A" # Default to N/A
    local error_msg=""

    if [[ $iperf_exit_code -ne 0 ]]; then
        error_msg="iperf3 command failed with exit code $iperf_exit_code."
        # Try to extract a more specific error from iperf3 output if it's not JSON
        if ! echo "$iperf_output" | jq -e . > /dev/null 2>&1; then
            local iperf_error
            iperf_error=$(echo "$iperf_output" | head -n 1) # Take first line as a summary
            error_msg="$error_msg Iperf3 error: $iperf_error"
        fi
        log_warn "iperf3 test (${test_type_log}) against ${host}:${port} failed. ${error_msg}"
        echo "N/A|$error_msg" # Pipe as delimiter for multi-value return
        return 1
    fi

    # Attempt to parse JSON output
    local sum_key
    if [[ -n "$direction_flag" ]]; then # Upload test, sum_sent
        sum_key=".end.sum_sent.bits_per_second"
    else # Download test, sum_received
        sum_key=".end.sum_received.bits_per_second"
    fi

    local bits_per_second
    bits_per_second=$(echo "$iperf_output" | jq -r "$sum_key // \"null\"")

    if [[ "$bits_per_second" == "null" || -z "$bits_per_second" ]]; then
        error_msg="Could not parse throughput from iperf3 JSON output."
        # Check for specific error object in JSON
        local json_error
        json_error=$(echo "$iperf_output" | jq -r '.error // ""')
        if [[ -n "$json_error" ]]; then
            error_msg="$error_msg Iperf3 JSON error: $json_error"
        fi
        log_warn "Failed to parse iperf3 JSON (${test_type_log}) for ${host}:${port}. ${error_msg}"
        log_debug "iperf3 output for ${host}:${port} (${test_type_log}): $iperf_output"
        echo "N/A|$error_msg"
        return 2
    fi

    throughput_mbps=$(echo "scale=2; $bits_per_second / 1000000" | bc -l)
    if ! [[ "$throughput_mbps" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then # Validate bc output
        error_msg="Calculated throughput is not a valid number: $throughput_mbps"
        log_warn "$error_msg for ${host}:${port} (${test_type_log})."
        echo "N/A|$error_msg"
        return 2 # Treat as a parsing/calculation issue
    fi


    log_info "Test ${test_type_log} with ${host}:${port}: ${throughput_mbps} Mbps"
    echo "${throughput_mbps}|" # Empty error message on success
    return 0
}

report_results() {
    if [[ ${#TEST_RESULTS_ARRAY[@]} -eq 0 ]]; then
        log_warn "No test results to report."
        return
    fi

    log_info "Generating performance report..."
    echo # Newline for cleaner output separation

    printf "${COLOR_BOLD}${COLOR_CYAN}"
    printf "+----------------------------------------------------------------------------------------------------+\n"
    printf "| %-70s |\n" "hyprbench-netblast :: Multi-Server Network Performance"
    printf "+------+--------------------------+-------------------------+-------+-----------------+-----------------+\n"
    printf "| Rank | Server Location          | Host                    | Port  | Download (Mbps) | Upload (Mbps)   |\n"
    printf "+------+--------------------------+-------------------------+-------+-----------------+-----------------+\n"
    printf "${COLOR_RESET}"

    # Sort results:
    # Lines are "host|port|location_str|dl_mbps|ul_mbps"
    # We need to sort by dl_mbps (4th field) numerically, descending. "N/A" should be last.
    # Use awk to prepare for sort: replace N/A with -1 (or a very low number) for numeric sort.
    # Then sort, then re-format for display.
    
    local sorted_results=()
    local original_ifs="$IFS"
    IFS=$'\n' # Handle lines with spaces in location correctly
    
    # Create a temporary array of lines where N/A in download speed is replaced by -1 for sorting
    local sortable_lines=()
    for item in "${TEST_RESULTS_ARRAY[@]}"; do
        local host port location dl ul
        IFS='|' read -r host port location dl ul <<< "$item"
        local dl_sort_val="$dl"
        [[ "$dl" == "N/A" ]] && dl_sort_val="-1" # Treat N/A as lowest for sorting
        # Store original values along with sort key
        sortable_lines+=("$(printf "%.2f" "$dl_sort_val")|$item")
    done
    
    # Sort based on the prepended sort key (download speed)
    # -k1,1nr means sort by the first field, numerically, reverse (descending)
    sorted_results=($(printf '%s\n' "${sortable_lines[@]}" | sort -t'|' -k1,1nr))
    
    IFS="$original_ifs"

    local rank=0
    for sorted_item_line in "${sorted_results[@]}"; do
        rank=$((rank + 1))
        # Remove the sort key part
        local item="${sorted_item_line#*|}"
        
        local host port location dl ul
        IFS='|' read -r host port location dl ul <<< "$item"

        # Ensure numeric fields are padded for alignment if they are numbers
        local dl_display="$dl"
        local ul_display="$ul"

        if [[ "$dl" != "N/A" ]]; then
            dl_display=$(printf "%.2f" "$dl")
        fi
        if [[ "$ul" != "N/A" ]]; then
            ul_display=$(printf "%.2f" "$ul")
        fi

        printf "| %-4s | %-24s | %-23s | %-5s | %15s | %15s |\n" \
               "$rank" \
               "${location:0:24}" \
               "${host:0:23}" \
               "$port" \
               "$dl_display" \
               "$ul_display"
    done

    printf "${COLOR_BOLD}${COLOR_CYAN}"
    printf "+------+--------------------------+-------------------------+-------+-----------------+-----------------+\n"
    printf "| %-94s |\n" "* Tests run with P=${IPERF_PARALLEL_STREAMS} streams, t=${IPERF_DURATION} seconds each way."
    printf "+----------------------------------------------------------------------------------------------------+\n"
    printf "${COLOR_RESET}"
    echo # Newline after table
}


# === MAIN SCRIPT ===
main() {
    log_info "hyprbench-netblast :: Multi-Server Network Performance Test"
    log_info "Initializing..."

    # --- DEPENDENCY CHECKS ---
    local dependencies=("iperf3" "curl" "jq" "bc")
    local missing_deps=0
    log_info "Checking for required dependencies..."
    for cmd in "${dependencies[@]}"; do
        if ! command -v "$cmd" &>/dev/null; then
            log_error "Required command '$cmd' is not installed. Please install it and try again."
            missing_deps=1
        else
            log_info "Dependency '$cmd' ... ${COLOR_GREEN}found${COLOR_RESET}"
        fi
    done

    if [[ "$missing_deps" -eq 1 ]]; then
        log_error "One or more dependencies are missing. Exiting."
        exit 1
    fi
    log_info "All dependencies satisfied."

    # --- GEO-LOCATION ---
    declare USER_LAT USER_LON USER_COUNTRY USER_CITY USER_IP # Ensure they are scoped for later use
    if ! get_user_location; then
        log_error "Failed to determine user location. Cannot proceed with distance-based server filtering."
        exit 1
    fi

    # --- FETCH IPERF3 SERVER LIST ---
    declare IPERF_SERVER_LIST_JSON # Ensure it's available
    if ! fetch_iperf_servers; then
        log_error "Failed to fetch iperf3 server list. Cannot proceed."
        exit 1
    fi
    # log_debug "Raw server list JSON: ${IPERF_SERVER_LIST_JSON}" # Uncomment for debugging

    # --- FILTER AND SELECT SERVERS ---
    declare -a SELECTED_SERVERS_JSON_ARRAY # Make it available
    if ! filter_and_select_servers; then
        # This function currently returns 0 even on some failures to get the list,
        # error handling within it should be robust.
        # If it did return non-zero for critical internal errors:
        # log_error "Critical error during server filtering. Exiting."
        # exit 1
        : # Continue, it might just mean no servers were found, which is handled by checking array size.
    fi

    if [[ ${#SELECTED_SERVERS_JSON_ARRAY[@]} -eq 0 ]]; then
        log_warn "No suitable iperf3 servers found based on the criteria. Exiting."
        # Optionally, could still print a header and "No servers found" message instead of exiting.
        # For now, exit as per original requirements if no servers.
        exit 0 # Graceful exit as no servers is not a script failure.
    fi

    # --- RUN IPERF3 TESTS ---
    log_info "Starting iperf3 tests on selected servers..."
    declare -a TEST_RESULTS_ARRAY # Store "Host|Port|Location|DownloadMbps|UploadMbps" for sorting

    for server_json in "${SELECTED_SERVERS_JSON_ARRAY[@]}"; do
        local host port city country_code location_str
        host=$(echo "$server_json" | jq -r '.host')
        port=$(echo "$server_json" | jq -r '.port')
        city=$(echo "$server_json" | jq -r '.city // "N/A"')
        country_code=$(echo "$server_json" | jq -r '.country_code // "N/A"')
        location_str="${city}, ${country_code}"

        log_info "${COLOR_CYAN}Testing against server: ${host}:${port} (${location_str})${COLOR_RESET}"

        local dl_result_str ul_result_str
        local dl_mbps="N/A" ul_mbps="N/A"
        local dl_error_msg ul_error_msg

        # Download Test
        dl_result_str=$(run_single_iperf_test "$host" "$port" "")
        IFS='|' read -r dl_mbps dl_error_msg <<< "$dl_result_str"
        
        # Upload Test
        ul_result_str=$(run_single_iperf_test "$host" "$port" "-R")
        IFS='|' read -r ul_mbps ul_error_msg <<< "$ul_result_str"

        TEST_RESULTS_ARRAY+=("${host}|${port}|${location_str}|${dl_mbps}|${ul_mbps}")
        
        if [[ "$dl_mbps" == "N/A" && -n "$dl_error_msg" ]]; then
            log_warn "Download test for $host failed: $dl_error_msg"
        fi
        if [[ "$ul_mbps" == "N/A" && -n "$ul_error_msg" ]]; then
            log_warn "Upload test for $host failed: $ul_error_msg"
        fi
    done
    
    log_info "All iperf3 tests completed."

    report_results

    log_info "hyprbench-netblast finished."
}

# Ensure script is not sourced
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi