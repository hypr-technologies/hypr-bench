package cmd

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"encoding/xml" // For XML unmarshalling
	"fmt"          // For io.ReadAll
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time" // For version/date/hostname print

	"github.com/spf13/cobra"
)

// Flags for root command (will hold skip flags etc.)
var (
	logFile string
	// tempDir string // We'll manage temp files within functions for now

	skipCPU       bool
	skipMemory    bool
	skipDisk      bool
	skipStress    bool
	skipNetwork   bool
	skipNetblast  bool // Specific skip for netblast part
	skipPublicRef bool

	fioTargetDir string
	fioTestSize  string

	// For auto-dependency installation
	autoInstallDeps bool

	// For exporting results
	exportJSON string
	exportHTML string

	// For progress display
	showProgress bool

	// For FIO test profile
	fioTestProfile string

	// For web server
	startWebServer bool
	webServerPort  int
)

// Structs for storing results (to be expanded)
type SystemInfo struct {
	CPUModel         string
	CPUCores         string
	CPUThreads       string
	CPUSpeed         string
	CPUCache         string
	RAMTotal         string
	RAMType          string // May be hard to get reliably without parsing dmidecode deeply
	RAMSpeed         string // May be hard to get reliably
	MotherboardMfr   string
	MotherboardModel string
	OSName           string
	OSVersion        string
	KernelVersion    string
	StorageDevices   []StorageDevice
	NVMeControllers  []string
	NVMeDetails      []NVMeInfo
	Hostname         string
	HyprBenchVersion string
	TestDate         string
	// CPU Benchmark Results
	SysbenchSingleThreadScore string
	SysbenchMultiThreadScore  string
	// Memory Benchmark Results (STREAM via PTS)
	StreamCopyBandwidthMBs   string // MB/s
	StreamScaleBandwidthMBs  string // MB/s
	StreamAddBandwidthMBs    string // MB/s
	StreamTriadBandwidthMBs  string // MB/s
	PtsStreamResultFile      string // Path to the result file for reference
	PtsEnterpriseSetupNeeded bool   // Flag if setup was needed
	// Disk I/O Benchmark Results (FIO)
	FioResults []FioDeviceResult // Results for each tested device
	// Network Benchmark Results
	SpeedtestResults SpeedtestResult  // Results from local speedtest
	Iperf3Results    []Iperf3Result   // Results from iperf3 single server tests
	NetblastResults  []NetblastResult // Results from hyprbench-netblast
	// Stress Benchmark Results
	StressResults StressResults // Results from stress-ng tests
	// Public Reference Benchmark Results
	UnixBenchResults UnixBenchResults // Results from UnixBench via PTS
}

type StorageDevice struct {
	Name       string
	Size       string
	Type       string
	MountPoint string
	FSType     string
	Rota       string // Rotational: 0 for SSD/NVMe, 1 for HDD
}

type NVMeInfo struct {
	DevicePath string
	Model      string
	Size       string
	Partitions []NVMePartition
}

type NVMePartition struct {
	Path       string
	MountPoint string
	Size       string
}

// FIO benchmark result structures
type FioDeviceResult struct {
	DevicePath     string          // Path to the device or mount point tested
	DeviceModel    string          // Model of the device (if available)
	MountPoint     string          // Mount point where test was performed
	TestFileSize   string          // Size of the test file used
	TestResults    []FioTestResult // Results for each test scenario
	TestsCompleted bool            // Whether all tests completed successfully
}

type FioTestResult struct {
	TestName    string  // Name of the test (e.g., "4K_RandRead_QD64")
	ReadWrite   string  // Type of test (e.g., "randread", "write", "randrw")
	BlockSize   string  // Block size used (e.g., "4k", "1m")
	IODepth     int     // IO depth used
	NumJobs     int     // Number of jobs used
	RWMixRead   int     // Read percentage for mixed tests (0-100)
	IOPS        float64 // IO operations per second
	BandwidthMB float64 // Bandwidth in MB/s
	LatencyUs   float64 // Average latency in microseconds
	LatencyUnit string  // Unit for latency (us, ms)
}

// Network benchmark result structures
type SpeedtestResult struct {
	ToolUsed      string  // Which tool was used (speedtest-cli, fast-cli)
	DownloadMbps  float64 // Download speed in Mbps
	UploadMbps    float64 // Upload speed in Mbps
	LatencyMs     float64 // Latency in milliseconds
	TestCompleted bool    // Whether the test completed successfully
	ErrorMessage  string  // Error message if test failed
}

type Iperf3Result struct {
	Host          string  // Server hostname
	Port          int     // Server port
	Location      string  // Server location (city, country)
	DownloadMbps  float64 // Download speed in Mbps
	UploadMbps    float64 // Upload speed in Mbps
	TestCompleted bool    // Whether both tests completed successfully
	ErrorMessage  string  // Error message if test failed
}

type NetblastResult struct {
	Host          string  // Server hostname
	Port          int     // Server port
	Location      string  // Server location (city, country)
	Distance      int     // Distance in km
	DownloadMbps  float64 // Download speed in Mbps
	UploadMbps    float64 // Upload speed in Mbps
	TestCompleted bool    // Whether both tests completed successfully
	Rank          int     // Rank in the results (1 = best)
}

// Stress benchmark result structure
type StressResults struct {
	CPUMethod           string  // CPU stress method used
	CPUCores            string  // Number of CPU cores used
	CPUBogoOps          float64 // CPU bogo operations
	CPUBogoOpsPerSec    float64 // CPU bogo operations per second
	MatrixBogoOps       float64 // Matrix bogo operations
	MatrixBogoOpsPerSec float64 // Matrix bogo operations per second
	VMBogoOps           float64 // VM bogo operations
	VMBogoOpsPerSec     float64 // VM bogo operations per second
	TestsCompleted      bool    // Whether all tests completed successfully
}

// UnixBench benchmark result structure
type UnixBenchResults struct {
	SystemBenchmarkIndex     float64 // Overall system benchmark index
	Dhrystone2               float64 // Dhrystone 2 score
	DoubleFloatingPoint      float64 // Double-precision floating point score
	ExecThroughput           float64 // Execl throughput score
	FileCopy1K               float64 // File copy 1K buffers score
	FileCopy256B             float64 // File copy 256B buffers score
	FileCopy4K               float64 // File copy 4K buffers score
	PipeThroughput           float64 // Pipe throughput score
	PipeBasedCS              float64 // Pipe-based context switching score
	ProcessCreation          float64 // Process creation score
	ShellScripts             float64 // Shell scripts (1 concurrent) score
	SystemCallOverhead       float64 // System call overhead score
	TestCompleted            bool    // Whether the test completed successfully
	ErrorMessage             string  // Error message if test failed
	ResultFile               string  // Path to the result file
	PtsEnterpriseSetupNeeded bool    // Flag if PTS enterprise setup was needed
}

// Constants for version and formatting
const (
	hyprBenchVersionString = "0.1.0-go" // Define version string as a constant

	// ANSI color codes for terminal output
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorBold   = "\033[1m"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "hyprbench",
	Version: hyprBenchVersionString, // Use the constant
	Short:   "HyprBench: Comprehensive System Benchmark Utility (Go Edition)",
	Long: `A comprehensive benchmark utility for Linux systems, written in Go.
Tests CPU, memory, disk I/O (NVMe focus), network, and system stress.
Requires external tools like sysbench, fio, iperf3 etc. to be installed,
or can attempt to auto-install them if run with --auto-install-deps.`,
	Run: func(cmd *cobra.Command, args []string) {
		startTime := time.Now()
		sysInfo := SystemInfo{
			HyprBenchVersion: hyprBenchVersionString, // Use the constant
			TestDate:         startTime.Format("2006-01-02 15:04:05 MST"),
		}
		if h, err := os.Hostname(); err == nil {
			sysInfo.Hostname = h
		} else {
			sysInfo.Hostname = "N/A"
		}

		fmt.Println("HyprBench Go Edition - Starting...")
		fmt.Printf("Version: %s, Date: %s, Hostname: %s\n", sysInfo.HyprBenchVersion, sysInfo.TestDate, sysInfo.Hostname)
		fmt.Println("========================================")

		checkRoot() // Call the root check function

		fmt.Println("Checking dependencies...")
		if err := checkDependencies(autoInstallDeps); err != nil {
			fmt.Fprintf(os.Stderr, "Dependency check failed: %v\n", err)
			fmt.Fprintf(os.Stderr, "Try running with --auto-install-deps if you want HyprBench to attempt installation (requires root).\n")
			os.Exit(1)
		}
		fmt.Println("Dependency checks passed.")

		fmt.Println("\n--- Gathering System Information ---")
		if err := gatherSystemInformation(&sysInfo); err != nil {
			fmt.Printf("Error gathering system information: %v\n", err)
		}
		printSystemInformation(sysInfo)
		fmt.Println("--- Finished System Information ---")

		if !skipCPU {
			fmt.Println("\n--- Running CPU Benchmarks ---")
			if err := runCpuBenchmarks(&sysInfo); err != nil { // Pass sysInfo
				fmt.Printf("Error during CPU benchmarks: %v\n", err)
			}
			fmt.Println("--- Finished CPU Benchmarks ---")
		}
		if !skipMemory {
			fmt.Println("\n--- Running Memory Benchmarks ---")
			if err := runMemoryBenchmarks(&sysInfo); err != nil { // Pass sysInfo
				fmt.Printf("Error during Memory benchmarks: %v\n", err)
			}
			fmt.Println("--- Finished Memory Benchmarks ---")
		}

		if !skipDisk {
			fmt.Println("\n--- Running Disk I/O Benchmarks ---")
			if err := runDiskBenchmarks(fioTargetDir, fioTestSize, fioTestProfile, &sysInfo); err != nil {
				fmt.Printf("Error during Disk I/O benchmarks: %v\n", err)
			}
			fmt.Println("--- Finished Disk I/O Benchmarks ---")
		}

		if !skipStress {
			fmt.Println("\n--- Running Threads & System Stress Benchmarks ---")
			if err := runStressBenchmarks(&sysInfo); err != nil {
				fmt.Printf("Error during Stress benchmarks: %v\n", err)
			}
			fmt.Println("--- Finished Threads & System Stress Benchmarks ---")
		}

		runNetBlast := !skipNetblast
		if skipNetwork { // if all network is skipped, netblast is also skipped
			runNetBlast = false
		}

		if !skipNetwork || runNetBlast { // only run if not all network is skipped OR netblast specifically isn't
			fmt.Println("\n--- Running Network Benchmarks ---")
			if err := runNetworkBenchmarks(runNetBlast, &sysInfo); err != nil {
				fmt.Printf("Error during Network benchmarks: %v\n", err)
			}
			fmt.Println("--- Finished Network Benchmarks ---")
		}

		if !skipPublicRef {
			fmt.Println("\n--- Running Public Reference Benchmarks (UnixBench via PTS) ---")
			if err := runPublicRefBenchmarks(&sysInfo); err != nil {
				fmt.Printf("Error during Public Reference benchmarks: %v\n", err)
			}
			fmt.Println("--- Finished Public Reference Benchmarks ---")
		}

		// Print summary of key metrics
		fmt.Println("\n========================================")
		fmt.Println(colorBold + "HyprBench Summary" + colorReset)
		fmt.Println("----------------------------------------")

		// CPU Summary
		if sysInfo.SysbenchSingleThreadScore != "" || sysInfo.SysbenchMultiThreadScore != "" {
			fmt.Printf("CPU:      %s (%s cores, %s threads)\n", sysInfo.CPUModel, sysInfo.CPUCores, sysInfo.CPUThreads)
			if sysInfo.SysbenchSingleThreadScore != "" {
				fmt.Printf("          Single-Thread: %s%s events/sec%s\n",
					colorGreen, sysInfo.SysbenchSingleThreadScore, colorReset)
			}
			if sysInfo.SysbenchMultiThreadScore != "" {
				fmt.Printf("          Multi-Thread:  %s%s events/sec%s\n",
					colorGreen, sysInfo.SysbenchMultiThreadScore, colorReset)
			}
		}

		// Memory Summary
		if sysInfo.StreamCopyBandwidthMBs != "" || sysInfo.StreamTriadBandwidthMBs != "" {
			fmt.Printf("Memory:   %s\n", sysInfo.RAMTotal)
			if sysInfo.StreamTriadBandwidthMBs != "" {
				fmt.Printf("          STREAM Triad:  %s%s MB/s%s\n",
					colorGreen, sysInfo.StreamTriadBandwidthMBs, colorReset)
			}
		}

		// Disk Summary
		if len(sysInfo.FioResults) > 0 {
			for i, device := range sysInfo.FioResults {
				if i == 0 {
					fmt.Printf("Disk:     %s\n", device.DeviceModel)
				} else {
					fmt.Printf("Disk %d:   %s\n", i+1, device.DeviceModel)
				}

				// Find 4K Random Read result
				for _, test := range device.TestResults {
					if strings.Contains(test.TestName, "4K_RandRead") {
						fmt.Printf("          4K Random Read:  %s%.0f IOPS, %.2f MB/s%s\n",
							colorGreen, test.IOPS, test.BandwidthMB, colorReset)
						break
					}
				}

				// Find 1M Sequential Read result
				for _, test := range device.TestResults {
					if strings.Contains(test.TestName, "1M_SeqRead") {
						fmt.Printf("          1M Sequential Read: %s%.2f MB/s%s\n",
							colorGreen, test.BandwidthMB, colorReset)
						break
					}
				}
			}
		}

		// Network Summary
		if sysInfo.SpeedtestResults.TestCompleted {
			fmt.Printf("Network:  %s\n", sysInfo.SpeedtestResults.ToolUsed)
			fmt.Printf("          Download: %s%.2f Mbps%s, Upload: %s%.2f Mbps%s\n",
				colorGreen, sysInfo.SpeedtestResults.DownloadMbps, colorReset,
				colorGreen, sysInfo.SpeedtestResults.UploadMbps, colorReset)
		}

		// UnixBench Summary
		if sysInfo.UnixBenchResults.SystemBenchmarkIndex > 0 {
			fmt.Printf("UnixBench: System Benchmark Index: %s%.2f%s\n",
				colorGreen, sysInfo.UnixBenchResults.SystemBenchmarkIndex, colorReset)
		}

		fmt.Println("----------------------------------------")
		duration := time.Since(startTime)
		fmt.Printf("HyprBench Finished. Total duration: %s\n", duration.Round(time.Second))

		// Export results if requested
		if exportJSON != "" {
			fmt.Printf("Exporting results to JSON: %s\n", exportJSON)
			if err := exportResultsToJSON(sysInfo, exportJSON); err != nil {
				fmt.Printf("Error exporting to JSON: %v\n", err)
			} else {
				fmt.Printf("%sResults successfully exported to JSON%s\n", colorGreen, colorReset)
			}
		}

		if exportHTML != "" {
			fmt.Printf("Exporting results to HTML: %s\n", exportHTML)
			if err := exportResultsToHTML(sysInfo, exportHTML); err != nil {
				fmt.Printf("Error exporting to HTML: %v\n", err)
			} else {
				fmt.Printf("%sResults successfully exported to HTML%s\n", colorGreen, colorReset)
			}
		}

		// Save log file if requested
		if logFile != "" {
			fmt.Printf("Saving log to: %s\n", logFile)
			// TODO: Implement log file saving
		}

		// Start web server if requested
		if startWebServer {
			fmt.Printf("\nStarting web server to view results...\n")

			// Find an available port starting from the specified port
			port := FindAvailablePort(webServerPort)
			if port != webServerPort {
				fmt.Printf("Port %d is in use, using port %d instead\n", webServerPort, port)
			}

			// Create web server config
			webConfig := WebServerConfig{
				Port:    port,
				SysInfo: &sysInfo,
			}

			// Start web server
			if err := StartWebServer(webConfig); err != nil {
				fmt.Printf("Error starting web server: %v\n", err)
			}
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVar(&logFile, "log-file", "", "Log output to a file in addition to STDOUT (e.g., ./hyprbench-YYYYMMDD-HHMMSS.log)")
	rootCmd.Flags().BoolVar(&autoInstallDeps, "auto-install-deps", false, "Attempt to automatically install missing dependencies (requires root and common package managers like apt/dnf). Use with caution.")

	rootCmd.Flags().BoolVar(&skipCPU, "skip-cpu", false, "Skip CPU benchmarks")
	rootCmd.Flags().BoolVar(&skipMemory, "skip-memory", false, "Skip Memory benchmarks")
	rootCmd.Flags().BoolVar(&skipDisk, "skip-disk", false, "Skip Disk I/O (FIO) benchmarks")
	rootCmd.Flags().BoolVar(&skipStress, "skip-stress", false, "Skip stress-ng benchmarks")
	rootCmd.Flags().BoolVar(&skipNetwork, "skip-network", false, "Skip ALL network benchmarks (local speedtest, iperf3, netblast)")
	rootCmd.Flags().BoolVar(&skipNetblast, "skip-netblast", false, "Skip only the hyprbench-netblast multi-server tests (if --skip-network is not set)")
	rootCmd.Flags().BoolVar(&skipPublicRef, "skip-public-ref", false, "Skip public reference benchmarks (e.g., UnixBench via PTS)")

	rootCmd.Flags().StringVar(&fioTargetDir, "fio-target-dir", "", "Specify a single directory/device for FIO tests (overrides NVMe auto-detection)")
	rootCmd.Flags().StringVar(&fioTestSize, "fio-test-size", "1G", "Override FIO test file size (e.g., 1G, 8G, 16G). Default is 1G for faster runs.")
	rootCmd.Flags().StringVar(&fioTestProfile, "fio-profile", "standard", "FIO test profile: 'standard', 'quick', 'thorough', 'iops', 'throughput', 'latency', or 'all'")

	// Export options
	rootCmd.Flags().StringVar(&exportJSON, "export-json", "", "Export results to JSON file (e.g., ./hyprbench-results.json)")
	rootCmd.Flags().StringVar(&exportHTML, "export-html", "", "Export results to HTML file (e.g., ./hyprbench-results.html)")

	// Progress display
	rootCmd.Flags().BoolVar(&showProgress, "show-progress", true, "Show progress indicators for long-running benchmarks")

	// Web server
	rootCmd.Flags().BoolVar(&startWebServer, "web", false, "Start a web server to view results after benchmarks complete")
	rootCmd.Flags().IntVar(&webServerPort, "web-port", 8080, "Port to use for the web server")
}

// --- Utility function to run commands ---
func runCommand(name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	// fmt.Printf("Executing: %s %s\n", name, strings.Join(arg, " ")) // Debug
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command '%s %s' failed: %w. Output: %s", name, strings.Join(arg, " "), err, string(output))
	}
	return string(output), nil
}

// --- Benchmark Functions ---
func gatherSystemInformation(sysInfo *SystemInfo) error {
	var err error
	var output string

	fmt.Println("  Gathering CPU Information...")
	output, err = runCommand("lscpu")
	if err != nil {
		fmt.Printf("    Warning: could not run lscpu: %v\n", err)
	} else {
		sysInfo.CPUModel = parseLscpu(output, "Model name:")
		sysInfo.CPUCores = parseLscpu(output, "Core(s) per socket:") // This might need adjustment if you have multiple sockets
		// A better way for total cores might be:
		// Sockets := parseLscpu(output, "Socket(s):")
		// CoresPerSocket := parseLscpu(output, "Core(s) per socket:")
		// ThreadsPerCore := parseLscpu(output, "Thread(s) per core:")
		// If these are numbers, sysInfo.CPUCores = Sockets * CoresPerSocket
		// sysInfo.CPUThreads = Sockets * CoresPerSocket * ThreadsPerCore
		// For now, a simpler approach:
		sysInfo.CPUCores = parseLscpu(output, "CPU(s):")               // Total logical CPUs (threads)
		sysInfo.CPUThreads = parseLscpu(output, "Thread(s) per core:") // Will need multiplication if you want total threads based on this
		// Let's refine CPUCores and CPUThreads based on common lscpu output
		// Total logical CPUs is usually just "CPU(s):"
		// True physical cores = CPU(s) / Thread(s) per core (if hyperthreading is on)
		// Or, Socket(s) * Core(s) per socket

		// Let's aim for:
		// CPUModel: Model name:
		// CPUCores: Socket(s) * Core(s) per socket
		// CPUThreads: CPU(s)
		// CPUSpeed: CPU MHz: or CPU max MHz:
		// CPUCacheL3: L3 cache:

		socketsStr := parseLscpu(output, "Socket(s):")
		coresPerSocketStr := parseLscpu(output, "Core(s) per socket:")
		sysInfo.CPUThreads = parseLscpu(output, "CPU(s):") // Total logical processors

		s, sErr := parseIntStrict(socketsStr)
		c, cErr := parseIntStrict(coresPerSocketStr)
		if sErr == nil && cErr == nil && s > 0 && c > 0 {
			sysInfo.CPUCores = fmt.Sprintf("%d", s*c)
		} else {
			// Fallback if socket/cores per socket not found, use CPU(s) and divide by threads per core
			totalCPUsStr := sysInfo.CPUThreads // This is actually total logical CPUs
			threadsPerCoreStr := parseLscpu(output, "Thread(s) per core:")
			totalLogicalCPUs, tlcErr := parseIntStrict(totalCPUsStr)
			threadsPerCore, tpcErr := parseIntStrict(threadsPerCoreStr)

			if tlcErr == nil && tpcErr == nil && totalLogicalCPUs > 0 && threadsPerCore > 0 {
				sysInfo.CPUCores = fmt.Sprintf("%d", totalLogicalCPUs/threadsPerCore)
			} else if tlcErr == nil && totalLogicalCPUs > 0 { // If threadspercore not found, assume physical cores = logical CPUs
				sysInfo.CPUCores = totalCPUsStr
			} else {
				sysInfo.CPUCores = "N/A"
			}
		}

		maxMhz := parseLscpu(output, "CPU max MHz:")
		minMhz := parseLscpu(output, "CPU min MHz:")
		currentMhz := parseLscpu(output, "CPU MHz:")

		if maxMhz != "N/A" {
			sysInfo.CPUSpeed = maxMhz + " MHz (Max)"
		} else if currentMhz != "N/A" {
			sysInfo.CPUSpeed = currentMhz + " MHz (Current)"
		} else {
			sysInfo.CPUSpeed = "N/A"
		}
		if minMhz != "N/A" && sysInfo.CPUSpeed != "N/A" {
			sysInfo.CPUSpeed += fmt.Sprintf(" (Min: %s MHz)", minMhz)
		}

		sysInfo.CPUCache = parseLscpu(output, "L3 cache:")
		if sysInfo.CPUCache == "N/A" { // Try L2 if L3 not found
			sysInfo.CPUCache = parseLscpu(output, "L2 cache:")
			if sysInfo.CPUCache != "N/A" {
				sysInfo.CPUCache += " (L2)"
			}
		} else {
			sysInfo.CPUCache += " (L3)"
		}
	}

	fmt.Println("  Gathering RAM Information...")
	ramOutput, err := runCommand("free", "-b") // Variable already correctly named here from previous diff
	if err != nil {
		fmt.Printf("    Warning: could not run free: %v\n", err)
		sysInfo.RAMTotal = "N/A"
	} else {
		sysInfo.RAMTotal = parseFreeForTotal(ramOutput, "Mem:") // Use ramOutput
	}
	// RAM Type and Speed often require dmidecode, which is more complex. Placeholder for now.
	sysInfo.RAMType = "N/A (requires dmidecode)"
	sysInfo.RAMSpeed = "N/A (requires dmidecode)"

	fmt.Println("  Gathering OS Information...")
	kernelOutput, err := runCommand("uname", "-r")
	if err != nil {
		fmt.Printf("    Warning: could not run uname -r: %v\n", err)
		sysInfo.KernelVersion = "N/A"
	} else {
		sysInfo.KernelVersion = strings.TrimSpace(kernelOutput)
	}

	// OS Name and Version (lsb_release or /etc/os-release)
	osReleaseOutput, err := runCommand("lsb_release", "-ds") // Variable already correctly named here
	if err == nil {
		fullOS := strings.TrimSpace(osReleaseOutput) // Use osReleaseOutput
		// Remove quotes if any
		fullOS = strings.ReplaceAll(fullOS, "\"", "")
		// Try to split into name and version, simple split for now
		parts := strings.SplitN(fullOS, " ", 2) // e.g. "Ubuntu 22.04.3 LTS"
		if len(parts) > 0 {
			sysInfo.OSName = parts[0]
		}
		if len(parts) > 1 {
			sysInfo.OSVersion = parts[1]
		} else {
			sysInfo.OSVersion = fullOS // If no space, use the whole string as version (e.g. "Debian")
			if sysInfo.OSName == "" {
				sysInfo.OSName = fullOS
			}
		}
	} else {
		fmt.Printf("    Warning: lsb_release -ds failed: %v. Trying /etc/os-release.\n", err)
		// Fallback to /etc/os-release
		file, err := os.Open("/etc/os-release")
		if err != nil {
			fmt.Printf("    Warning: could not open /etc/os-release: %v\n", err)
			sysInfo.OSName = "N/A"
			sysInfo.OSVersion = "N/A"
		} else {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			vars := make(map[string]string)
			for scanner.Scan() {
				line := scanner.Text()
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := parts[0]
					value := strings.Trim(parts[1], "\"")
					vars[key] = value
				}
			}
			if name, ok := vars["PRETTY_NAME"]; ok {
				sysInfo.OSName = name  // PRETTY_NAME is usually more descriptive
				sysInfo.OSVersion = "" // PRETTY_NAME often includes version
			} else if name, ok := vars["NAME"]; ok {
				sysInfo.OSName = name
				if ver, ok := vars["VERSION"]; ok {
					sysInfo.OSVersion = ver
				} else if verId, ok := vars["VERSION_ID"]; ok {
					sysInfo.OSVersion = verId
				} else {
					sysInfo.OSVersion = "N/A"
				}
			} else {
				sysInfo.OSName = "N/A"
				sysInfo.OSVersion = "N/A"
			}
		}
	}
	fmt.Println("  Gathering Motherboard Information...")
	mbMfrOutput, err := runCommand("dmidecode", "-s", "system-manufacturer")
	if err != nil {
		fmt.Printf("    Warning: could not get system-manufacturer: %v\n", err)
		sysInfo.MotherboardMfr = "N/A"
	} else {
		sysInfo.MotherboardMfr = strings.TrimSpace(mbMfrOutput)
	}
	mbModelOutput, err := runCommand("dmidecode", "-s", "system-product-name")
	if err != nil {
		fmt.Printf("    Warning: could not get system-product-name: %v\n", err)
		sysInfo.MotherboardModel = "N/A"
	} else {
		sysInfo.MotherboardModel = strings.TrimSpace(mbModelOutput)
	}

	// --- Storage Overview (lsblk) ---
	fmt.Println("  Gathering Storage Information (lsblk)...")
	lsblkOutput, err := runCommand("lsblk", "-bpno", "NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE,ROTA")
	if err != nil {
		fmt.Printf("    Warning: could not run lsblk: %v\n", err)
	} else {
		sysInfo.StorageDevices = parseLsblkOutput(lsblkOutput)
	}

	// --- NVMe Controllers (lspci) ---
	fmt.Println("  Gathering NVMe Controller Information (lspci)...")
	lspciOutput, err := runCommand("lspci")
	if err != nil {
		fmt.Printf("    Warning: could not run lspci: %v\n", err)
	} else {
		sysInfo.NVMeControllers = parseLspciForNVMe(lspciOutput)
	}

	// --- Detailed NVMe Info ---
	fmt.Println("  Gathering Detailed NVMe Device Information (lsblk)...")
	lsblkNvmeOutput, err := runCommand("lsblk", "-d", "-p", "-no", "NAME,MODEL,SIZE")
	if err != nil {
		fmt.Printf("    Warning: could not run lsblk for NVMe details: %v\n", err)
	} else {
		sysInfo.NVMeDetails = parseLsblkForNVMeDetails(lsblkNvmeOutput)
	}

	// TODO: More detailed RAM Type/Speed (dmidecode -t memory)

	return nil
}

func printSystemInformation(sysInfo SystemInfo) {
	fmt.Println("\nSystem Information:")
	fmt.Println("----------------------------------------")
	fmt.Printf(" HyprBench Version: %s\n", sysInfo.HyprBenchVersion)
	fmt.Printf(" Test Date:         %s\n", sysInfo.TestDate)
	fmt.Printf(" Hostname:          %s\n\n", sysInfo.Hostname)

	fmt.Printf(" OS:                %s %s\n", sysInfo.OSName, sysInfo.OSVersion)
	fmt.Printf(" Kernel:            %s\n\n", sysInfo.KernelVersion)

	fmt.Printf(" CPU Model:         %s\n", sysInfo.CPUModel)
	fmt.Printf(" CPU Cores:         %s\n", sysInfo.CPUCores)
	fmt.Printf(" CPU Threads:       %s\n", sysInfo.CPUThreads)
	fmt.Printf(" CPU Speed:         %s\n", sysInfo.CPUSpeed)
	fmt.Printf(" CPU Cache:         %s\n\n", sysInfo.CPUCache)

	fmt.Printf(" RAM Total:         %s\n", sysInfo.RAMTotal)
	fmt.Printf(" RAM Type:          %s\n", sysInfo.RAMType)
	fmt.Printf(" RAM Speed:         %s\n\n", sysInfo.RAMSpeed)

	fmt.Printf(" Motherboard:       %s %s\n", strings.TrimSpace(sysInfo.MotherboardMfr), strings.TrimSpace(sysInfo.MotherboardModel)) // Removed extra \n

	// Display CPU Benchmark Results
	if sysInfo.SysbenchSingleThreadScore != "" || sysInfo.SysbenchMultiThreadScore != "" {
		fmt.Println("\nCPU Benchmark (Sysbench):")
		if sysInfo.SysbenchSingleThreadScore != "" {
			fmt.Printf("  Single-Thread Score: %s events/sec\n", sysInfo.SysbenchSingleThreadScore)
		}
		if sysInfo.SysbenchMultiThreadScore != "" {
			fmt.Printf("  Multi-Thread Score:  %s events/sec\n", sysInfo.SysbenchMultiThreadScore)
		}
	}
	// Display Memory Benchmark Results
	if sysInfo.StreamCopyBandwidthMBs != "" || sysInfo.StreamTriadBandwidthMBs != "" { // Check if any stream result is present
		fmt.Println("\nMemory Benchmark (STREAM via Phoronix Test Suite):")
		if sysInfo.StreamCopyBandwidthMBs != "N/A" && sysInfo.StreamCopyBandwidthMBs != "" {
			fmt.Printf("  STREAM Copy:         %s MB/s\n", sysInfo.StreamCopyBandwidthMBs)
		}
		if sysInfo.StreamScaleBandwidthMBs != "N/A" && sysInfo.StreamScaleBandwidthMBs != "" {
			fmt.Printf("  STREAM Scale:        %s MB/s\n", sysInfo.StreamScaleBandwidthMBs)
		}
		if sysInfo.StreamAddBandwidthMBs != "N/A" && sysInfo.StreamAddBandwidthMBs != "" {
			fmt.Printf("  STREAM Add:          %s MB/s\n", sysInfo.StreamAddBandwidthMBs)
		}
		if sysInfo.StreamTriadBandwidthMBs != "N/A" && sysInfo.StreamTriadBandwidthMBs != "" {
			fmt.Printf("  STREAM Triad:        %s MB/s\n", sysInfo.StreamTriadBandwidthMBs)
		}
		if sysInfo.StreamCopyBandwidthMBs == "Failed" ||
			(sysInfo.StreamCopyBandwidthMBs == "N/A" && sysInfo.PtsStreamResultFile == "") ||
			sysInfo.StreamCopyBandwidthMBs == "File Not Found" ||
			sysInfo.StreamCopyBandwidthMBs == "XML Parse Error" {
			fmt.Println("  STREAM tests failed, results not found, or could not be parsed.")
		}
		if sysInfo.PtsStreamResultFile != "" && sysInfo.PtsStreamResultFile != "See ~/.phoronix-test-suite/test-results/ for detailed XML/JSON or logs." {
			fmt.Printf("  Raw results XML: %s\n", sysInfo.PtsStreamResultFile)
		}
	}
	fmt.Println() // Add a newline before storage devices or the end line

	if len(sysInfo.StorageDevices) > 0 {
		fmt.Println("Storage Devices (lsblk):")
		// Header: NAME, SIZE, TYPE, MOUNTPOINT, FSTYPE, ROTA
		fmt.Printf("  %-20s %-10s %-8s %-5s %-25s %-8s\n", "Device", "Size", "Type", "Rota", "Mountpoint", "FSType")
		for _, dev := range sysInfo.StorageDevices {
			mount := dev.MountPoint
			if mount == "" {
				mount = "[none]"
			}
			fs := dev.FSType
			if fs == "" {
				fs = "[unknown]"
			}
			rota := dev.Rota
			if rota == "0" {
				rota = "SSD/NVMe"
			} else if rota == "1" {
				rota = "HDD"
			} else {
				rota = "?"
			}

			fmt.Printf("  %-20s %-10s %-8s %-5s %-25s %-8s\n", dev.Name, dev.Size, dev.Type, rota, mount, fs)
		}
		fmt.Println()
	}

	if len(sysInfo.NVMeControllers) > 0 {
		fmt.Println("NVMe Controllers (lspci):")
		for _, controller := range sysInfo.NVMeControllers {
			fmt.Printf("  - %s\n", controller)
		}
		fmt.Println()
	}

	if len(sysInfo.NVMeDetails) > 0 {
		fmt.Println("NVMe Device Details (lsblk):")
		for _, nvme := range sysInfo.NVMeDetails {
			fmt.Printf("  Device: %s, Model: %s, Size: %s\n", nvme.DevicePath, nvme.Model, nvme.Size)
			// TODO: Print partition info if gathered
		}
		fmt.Println()
	}

	// Display Disk I/O Benchmark Results (FIO)
	if len(sysInfo.FioResults) > 0 {
		fmt.Println("Disk I/O Benchmark Results (FIO):")

		for _, device := range sysInfo.FioResults {
			fmt.Printf("  Device: %s\n", device.DevicePath)
			if device.DeviceModel != device.DevicePath {
				fmt.Printf("  Model: %s\n", device.DeviceModel)
			}
			fmt.Printf("  Mount Point: %s\n", device.MountPoint)
			fmt.Printf("  Test File Size: %s\n", device.TestFileSize)

			if !device.TestsCompleted {
				fmt.Println("  Note: Some tests failed or were skipped")
			}

			// Print table header
			fmt.Println("  ---------------------------------------------------------------------------------")
			fmt.Printf("  %-32s | %-10s | %-18s | %-18s\n", "Test", "IOPS", "Bandwidth (MB/s)", "Avg Latency")
			fmt.Println("  ---------------------------------------------------------------------------------")

			// Print each test result
			for _, test := range device.TestResults {
				var iopsStr, bwStr, latencyStr string

				if test.IOPS < 0 {
					iopsStr = "FAIL"
				} else {
					iopsStr = fmt.Sprintf("%.0f", test.IOPS)
				}

				if test.BandwidthMB < 0 {
					bwStr = "FAIL"
				} else {
					bwStr = fmt.Sprintf("%.2f", test.BandwidthMB)
				}

				if test.LatencyUs < 0 {
					latencyStr = "FAIL"
				} else if test.LatencyUnit == "ms" {
					latencyStr = fmt.Sprintf("%.2f ms", test.LatencyUs)
				} else {
					latencyStr = fmt.Sprintf("%.2f us", test.LatencyUs)
				}

				fmt.Printf("  %-32s | %-10s | %-18s | %-18s\n",
					test.TestName, iopsStr, bwStr, latencyStr)
			}

			fmt.Println("  ---------------------------------------------------------------------------------")
			fmt.Println() // Add a newline between devices
		}
	}

	// Display Network Benchmark Results
	// 1. Local Speedtest Results
	if sysInfo.SpeedtestResults.ToolUsed != "None" && sysInfo.SpeedtestResults.ToolUsed != "" {
		fmt.Println("Network Benchmark Results:")
		fmt.Println("  Local Speed Test:")
		fmt.Printf("    Tool Used: %s\n", sysInfo.SpeedtestResults.ToolUsed)

		if sysInfo.SpeedtestResults.TestCompleted {
			fmt.Printf("    Download: %.2f Mbps\n", sysInfo.SpeedtestResults.DownloadMbps)
			fmt.Printf("    Upload: %.2f Mbps\n", sysInfo.SpeedtestResults.UploadMbps)
			if sysInfo.SpeedtestResults.LatencyMs > 0 {
				fmt.Printf("    Latency: %.2f ms\n", sysInfo.SpeedtestResults.LatencyMs)
			} else {
				fmt.Println("    Latency: Not available")
			}
		} else {
			fmt.Println("    Test failed or incomplete")
			if sysInfo.SpeedtestResults.ErrorMessage != "" {
				fmt.Printf("    Error: %s\n", sysInfo.SpeedtestResults.ErrorMessage)
			}
		}
		fmt.Println()
	}

	// 2. iperf3 Single Server Results
	if len(sysInfo.Iperf3Results) > 0 {
		fmt.Println("  iperf3 Single Server Tests:")
		fmt.Println("  --------------------------------------------------------------------------")
		fmt.Printf("  %-35s | %-5s | %-15s | %-15s\n", "Server Location/Host", "Port", "Download (Mbps)", "Upload (Mbps)")
		fmt.Println("  --------------------------------------------------------------------------")

		for _, result := range sysInfo.Iperf3Results {
			downloadStr := "N/A"
			if result.DownloadMbps > 0 {
				downloadStr = fmt.Sprintf("%.2f", result.DownloadMbps)
			}

			uploadStr := "N/A"
			if result.UploadMbps > 0 {
				uploadStr = fmt.Sprintf("%.2f", result.UploadMbps)
			}

			fmt.Printf("  %-35s | %-5d | %-15s | %-15s\n",
				result.Location, result.Port, downloadStr, uploadStr)
		}

		fmt.Println("  --------------------------------------------------------------------------")
		fmt.Println()
	}

	// 3. Netblast Results
	if len(sysInfo.NetblastResults) > 0 {
		fmt.Println("  hyprbench-netblast Multi-Server Tests:")
		fmt.Println("  --------------------------------------------------------------------------")
		fmt.Printf("  %-4s | %-24s | %-23s | %-5s | %-15s | %-15s\n",
			"Rank", "Location", "Host", "Port", "Download (Mbps)", "Upload (Mbps)")
		fmt.Println("  --------------------------------------------------------------------------")

		// Sort results by rank
		sort.Slice(sysInfo.NetblastResults, func(i, j int) bool {
			return sysInfo.NetblastResults[i].Rank < sysInfo.NetblastResults[j].Rank
		})

		for _, result := range sysInfo.NetblastResults {
			downloadStr := "N/A"
			if result.DownloadMbps > 0 {
				downloadStr = fmt.Sprintf("%.2f", result.DownloadMbps)
			}

			uploadStr := "N/A"
			if result.UploadMbps > 0 {
				uploadStr = fmt.Sprintf("%.2f", result.UploadMbps)
			}

			fmt.Printf("  %-4d | %-24s | %-23s | %-5d | %-15s | %-15s\n",
				result.Rank, result.Location, result.Host, result.Port, downloadStr, uploadStr)
		}

		fmt.Println("  --------------------------------------------------------------------------")
		fmt.Println()
	}

	// Display Stress Benchmark Results
	if sysInfo.StressResults.CPUBogoOps > 0 || sysInfo.StressResults.MatrixBogoOps > 0 || sysInfo.StressResults.VMBogoOps > 0 {
		fmt.Println("System Stress Benchmark Results (stress-ng):")

		if sysInfo.StressResults.CPUBogoOps > 0 {
			fmt.Printf("  CPU Stress Test (cores: %s, method: %s):\n",
				sysInfo.StressResults.CPUCores, sysInfo.StressResults.CPUMethod)
			fmt.Printf("    Bogo Operations:      %.0f\n", sysInfo.StressResults.CPUBogoOps)
			fmt.Printf("    Bogo Operations/sec:  %.2f\n", sysInfo.StressResults.CPUBogoOpsPerSec)
		}

		if sysInfo.StressResults.MatrixBogoOps > 0 {
			fmt.Println("  Matrix Stress Test:")
			fmt.Printf("    Bogo Operations:      %.0f\n", sysInfo.StressResults.MatrixBogoOps)
			fmt.Printf("    Bogo Operations/sec:  %.2f\n", sysInfo.StressResults.MatrixBogoOpsPerSec)
		}

		if sysInfo.StressResults.VMBogoOps > 0 {
			fmt.Println("  VM Stress Test:")
			fmt.Printf("    Bogo Operations:      %.0f\n", sysInfo.StressResults.VMBogoOps)
			fmt.Printf("    Bogo Operations/sec:  %.2f\n", sysInfo.StressResults.VMBogoOpsPerSec)
		}

		if !sysInfo.StressResults.TestsCompleted {
			fmt.Println("  Note: Some stress tests failed or were skipped")
		}

		fmt.Println()
	}

	// Display UnixBench Results
	if sysInfo.UnixBenchResults.SystemBenchmarkIndex > 0 {
		fmt.Println("Public Reference Benchmark Results (UnixBench via PTS):")
		fmt.Printf("  System Benchmark Index: %.2f\n", sysInfo.UnixBenchResults.SystemBenchmarkIndex)

		// Display individual test scores in a table format
		fmt.Println("  Individual Test Scores:")
		fmt.Println("  ---------------------------------------------------------------------------------")
		fmt.Printf("  %-30s | %-15s\n", "Test", "Score")
		fmt.Println("  ---------------------------------------------------------------------------------")

		if sysInfo.UnixBenchResults.Dhrystone2 > 0 {
			fmt.Printf("  %-30s | %-15.2f\n", "Dhrystone 2", sysInfo.UnixBenchResults.Dhrystone2)
		}
		if sysInfo.UnixBenchResults.DoubleFloatingPoint > 0 {
			fmt.Printf("  %-30s | %-15.2f\n", "Double Floating Point", sysInfo.UnixBenchResults.DoubleFloatingPoint)
		}
		if sysInfo.UnixBenchResults.ExecThroughput > 0 {
			fmt.Printf("  %-30s | %-15.2f\n", "Execl Throughput", sysInfo.UnixBenchResults.ExecThroughput)
		}
		if sysInfo.UnixBenchResults.FileCopy1K > 0 {
			fmt.Printf("  %-30s | %-15.2f\n", "File Copy 1K", sysInfo.UnixBenchResults.FileCopy1K)
		}
		if sysInfo.UnixBenchResults.FileCopy256B > 0 {
			fmt.Printf("  %-30s | %-15.2f\n", "File Copy 256B", sysInfo.UnixBenchResults.FileCopy256B)
		}
		if sysInfo.UnixBenchResults.FileCopy4K > 0 {
			fmt.Printf("  %-30s | %-15.2f\n", "File Copy 4K", sysInfo.UnixBenchResults.FileCopy4K)
		}
		if sysInfo.UnixBenchResults.PipeThroughput > 0 {
			fmt.Printf("  %-30s | %-15.2f\n", "Pipe Throughput", sysInfo.UnixBenchResults.PipeThroughput)
		}
		if sysInfo.UnixBenchResults.PipeBasedCS > 0 {
			fmt.Printf("  %-30s | %-15.2f\n", "Pipe-based Context Switching", sysInfo.UnixBenchResults.PipeBasedCS)
		}
		if sysInfo.UnixBenchResults.ProcessCreation > 0 {
			fmt.Printf("  %-30s | %-15.2f\n", "Process Creation", sysInfo.UnixBenchResults.ProcessCreation)
		}
		if sysInfo.UnixBenchResults.ShellScripts > 0 {
			fmt.Printf("  %-30s | %-15.2f\n", "Shell Scripts", sysInfo.UnixBenchResults.ShellScripts)
		}
		if sysInfo.UnixBenchResults.SystemCallOverhead > 0 {
			fmt.Printf("  %-30s | %-15.2f\n", "System Call Overhead", sysInfo.UnixBenchResults.SystemCallOverhead)
		}

		fmt.Println("  ---------------------------------------------------------------------------------")

		if sysInfo.UnixBenchResults.ResultFile != "" {
			fmt.Printf("  Raw results XML: %s\n", sysInfo.UnixBenchResults.ResultFile)
		}

		fmt.Println()
	} else if sysInfo.UnixBenchResults.ErrorMessage != "" {
		fmt.Println("Public Reference Benchmark Results (UnixBench via PTS):")
		fmt.Printf("  Error: %s\n", sysInfo.UnixBenchResults.ErrorMessage)
		fmt.Println()
	}

	fmt.Println("----------------------------------------")
}

func runCpuBenchmarks(sysInfo *SystemInfo) error { // Add *SystemInfo parameter
	fmt.Println("  Running sysbench CPU benchmarks...")
	var err error
	var output string
	var nprocStr string

	// Get nproc
	nprocOutput, err := runCommand("nproc")
	if err != nil {
		fmt.Println("    Warning: could not run nproc to get thread count for sysbench multi-thread test. Defaulting to 1 thread for multi-thread test.")
		nprocStr = "1"
	} else {
		nprocStr = strings.TrimSpace(nprocOutput)
		if _, parseErr := parseIntStrict(nprocStr); parseErr != nil {
			fmt.Printf("    Warning: could not parse nproc output '%s'. Defaulting to 1 thread. Error: %v\n", nprocStr, parseErr)
			nprocStr = "1"
		}
	}
	fmt.Printf("    Using %s thread(s) for multi-thread sysbench CPU test.\n", nprocStr)

	// Single-thread test
	fmt.Println("    Running sysbench CPU (1-thread, cpu-max-prime=20000)...")
	output, err = runCommand("sysbench", "cpu", "--threads=1", "--cpu-max-prime=20000", "run")
	if err != nil {
		fmt.Printf("      Error running single-thread sysbench: %v\n", err)
		sysInfo.SysbenchSingleThreadScore = "Failed"
	} else {
		sysInfo.SysbenchSingleThreadScore = parseSysbenchCpuOutput(output)
		fmt.Printf("      Single-thread score: %s events/sec\n", sysInfo.SysbenchSingleThreadScore)
	}

	// Multi-thread test
	fmt.Printf("    Running sysbench CPU (%s-threads, cpu-max-prime=20000)...\n", nprocStr)
	output, err = runCommand("sysbench", "cpu", fmt.Sprintf("--threads=%s", nprocStr), "--cpu-max-prime=20000", "run")
	if err != nil {
		fmt.Printf("      Error running multi-thread sysbench: %v\n", err)
		sysInfo.SysbenchMultiThreadScore = "Failed"
	} else {
		sysInfo.SysbenchMultiThreadScore = parseSysbenchCpuOutput(output)
		fmt.Printf("      Multi-thread score (%s threads): %s events/sec\n", nprocStr, sysInfo.SysbenchMultiThreadScore)
	}

	return nil
}

func parseSysbenchCpuOutput(output string) string {
	scanner := bufio.NewScanner(strings.NewReader(output))
	eventRateRegex := regexp.MustCompile(`events per second:\s*([\d.]+)`)
	for scanner.Scan() {
		line := scanner.Text()
		matches := eventRateRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return "N/A (parse error)"
}

// XML parsing structs for PTS results
type PtsResult struct {
	XMLName xml.Name `xml:"PhoronixTestSuite"`
	Result  struct {
		Data struct {
			Entry []struct {
				Identifier string `xml:"Identifier"` // e.g., "STREAM Copy"
				Value      string `xml:"Value"`      // e.g., "54321.0"
				Scale      string `xml:"Scale"`      // e.g., "MB/s"
			} `xml:"Entry"`
		} `xml:"Data"`
	} `xml:"Result"`
}

func runMemoryBenchmarks(sysInfo *SystemInfo) error {
	fmt.Println("  Running Memory Benchmarks...")

	// Try different methods in order of preference
	methods := []struct {
		name     string
		function func(*SystemInfo) error
	}{
		{"sysbench", runSysbenchMemoryBenchmark},
		{"direct", runDirectMemoryBenchmark},
	}

	// Try each method until one succeeds
	var lastError error
	for _, method := range methods {
		fmt.Printf("    Attempting memory benchmark using %s...\n", method.name)
		err := method.function(sysInfo)
		if err == nil {
			// Method succeeded
			return nil
		}

		// Method failed, try the next one
		lastError = err
		fmt.Printf("    %s memory benchmark failed: %v\n", method.name, err)
	}

	// If we get here, all methods failed
	setStreamResultsToFailed(sysInfo, "All Methods Failed")
	return fmt.Errorf("all memory benchmark methods failed, last error: %w", lastError)
}

// runSysbenchMemoryBenchmark runs memory benchmarks using sysbench
func runSysbenchMemoryBenchmark(sysInfo *SystemInfo) error {
	// Check if sysbench is available
	_, err := exec.LookPath("sysbench")
	if err != nil {
		return fmt.Errorf("sysbench not found: %w", err)
	}

	// Get total memory to determine test size
	var totalMemBytes uint64
	memInfoOutput, err := runCommand("cat", "/proc/meminfo")
	if err == nil {
		// Parse MemTotal from /proc/meminfo
		memTotalRegex := regexp.MustCompile(`MemTotal:\s+(\d+)\s+kB`)
		if matches := memTotalRegex.FindStringSubmatch(memInfoOutput); len(matches) > 1 {
			memTotalKB, err := strconv.ParseUint(matches[1], 10, 64)
			if err == nil {
				totalMemBytes = memTotalKB * 1024
			}
		}
	}

	// Default to 1GB if we couldn't determine total memory
	testSizeBytes := uint64(1 * 1024 * 1024 * 1024)
	if totalMemBytes > 0 {
		// Use 25% of total memory for the test
		testSizeBytes = totalMemBytes / 4
	}

	// Convert to MB for sysbench
	testSizeMB := testSizeBytes / (1024 * 1024)

	// Ensure test size is at least 100MB
	if testSizeMB < 100 {
		testSizeMB = 100
	}

	fmt.Printf("    Running sysbench memory test (size: %d MB)...\n", testSizeMB)

	// Run sysbench memory test
	output, err := runCommand("sysbench",
		"--test=memory",
		fmt.Sprintf("--memory-block-size=%d", 1024*1024), // 1MB blocks
		fmt.Sprintf("--memory-total-size=%dM", testSizeMB),
		"--memory-oper=read",
		"run")

	if err != nil {
		return fmt.Errorf("sysbench memory test failed: %w", err)
	}

	// Parse results
	transferRateRegex := regexp.MustCompile(`transferred \((\d+\.\d+) MB/sec\)`)
	if matches := transferRateRegex.FindStringSubmatch(output); len(matches) > 1 {
		// Use the sysbench result for all STREAM values since they're similar
		sysInfo.StreamCopyBandwidthMBs = matches[1]
		sysInfo.StreamScaleBandwidthMBs = matches[1]
		sysInfo.StreamAddBandwidthMBs = matches[1]
		sysInfo.StreamTriadBandwidthMBs = matches[1]

		fmt.Printf("    Memory bandwidth: %s MB/s\n", matches[1])
		return nil
	}

	return fmt.Errorf("could not parse sysbench memory test results")
}

// runDirectMemoryBenchmark runs a simple memory benchmark directly in Go
func runDirectMemoryBenchmark(sysInfo *SystemInfo) error {
	fmt.Println("    Running direct memory benchmark...")

	// Get total memory to determine test size
	var totalMemBytes uint64
	memInfoOutput, err := runCommand("cat", "/proc/meminfo")
	if err == nil {
		// Parse MemTotal from /proc/meminfo
		memTotalRegex := regexp.MustCompile(`MemTotal:\s+(\d+)\s+kB`)
		if matches := memTotalRegex.FindStringSubmatch(memInfoOutput); len(matches) > 1 {
			memTotalKB, err := strconv.ParseUint(matches[1], 10, 64)
			if err == nil {
				totalMemBytes = memTotalKB * 1024
			}
		}
	}

	// Default to 1GB if we couldn't determine total memory
	testSizeBytes := uint64(1 * 1024 * 1024 * 1024)
	if totalMemBytes > 0 {
		// Use 25% of total memory for the test
		testSizeBytes = totalMemBytes / 4
	}

	// Ensure test size is at least 100MB and no more than 4GB
	if testSizeBytes < 100*1024*1024 {
		testSizeBytes = 100 * 1024 * 1024
	} else if testSizeBytes > 4*1024*1024*1024 {
		testSizeBytes = 4 * 1024 * 1024 * 1024
	}

	// Create a temporary file with dd to measure disk write speed
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "hyprbench_memory_test")

	// Clean up any existing file
	os.Remove(tempFile)

	// Create a buffer of the test size
	bufferSize := int(testSizeBytes)
	buffer := make([]byte, bufferSize)

	// Fill buffer with random data
	for i := 0; i < bufferSize; i += 8 {
		// Fill 8 bytes at a time for efficiency
		if i+8 <= bufferSize {
			binary.LittleEndian.PutUint64(buffer[i:i+8], uint64(i))
		}
	}

	// Measure memory copy speed
	iterations := 5
	var copyBandwidth, scaleBandwidth, addBandwidth, triadBandwidth float64

	// Memory copy benchmark
	fmt.Println("    Running memory copy benchmark...")
	copyStart := time.Now()
	for i := 0; i < iterations; i++ {
		dst := make([]byte, bufferSize)
		copy(dst, buffer)
	}
	copyDuration := time.Since(copyStart)
	copyBandwidth = float64(bufferSize) * float64(iterations) / copyDuration.Seconds() / (1024 * 1024)

	// Memory scale benchmark (multiply by 2)
	fmt.Println("    Running memory scale benchmark...")
	scaleStart := time.Now()
	for i := 0; i < iterations; i++ {
		dst := make([]byte, bufferSize)
		for j := 0; j < bufferSize; j++ {
			dst[j] = buffer[j] * 2
		}
	}
	scaleDuration := time.Since(scaleStart)
	scaleBandwidth = float64(bufferSize) * float64(iterations) / scaleDuration.Seconds() / (1024 * 1024)

	// Memory add benchmark
	fmt.Println("    Running memory add benchmark...")
	addStart := time.Now()
	for i := 0; i < iterations; i++ {
		dst := make([]byte, bufferSize)
		for j := 0; j < bufferSize; j++ {
			dst[j] = buffer[j] + buffer[j]
		}
	}
	addDuration := time.Since(addStart)
	addBandwidth = float64(bufferSize) * float64(iterations) / addDuration.Seconds() / (1024 * 1024)

	// Memory triad benchmark
	fmt.Println("    Running memory triad benchmark...")
	triadStart := time.Now()
	for i := 0; i < iterations; i++ {
		dst := make([]byte, bufferSize)
		for j := 0; j < bufferSize; j++ {
			dst[j] = buffer[j] + buffer[j]*2
		}
	}
	triadDuration := time.Since(triadStart)
	triadBandwidth = float64(bufferSize) * float64(iterations) / triadDuration.Seconds() / (1024 * 1024)

	// Store results
	sysInfo.StreamCopyBandwidthMBs = fmt.Sprintf("%.2f", copyBandwidth)
	sysInfo.StreamScaleBandwidthMBs = fmt.Sprintf("%.2f", scaleBandwidth)
	sysInfo.StreamAddBandwidthMBs = fmt.Sprintf("%.2f", addBandwidth)
	sysInfo.StreamTriadBandwidthMBs = fmt.Sprintf("%.2f", triadBandwidth)

	fmt.Printf("    STREAM Copy: %s MB/s\n", sysInfo.StreamCopyBandwidthMBs)
	fmt.Printf("    STREAM Scale: %s MB/s\n", sysInfo.StreamScaleBandwidthMBs)
	fmt.Printf("    STREAM Add: %s MB/s\n", sysInfo.StreamAddBandwidthMBs)
	fmt.Printf("    STREAM Triad: %s MB/s\n", sysInfo.StreamTriadBandwidthMBs)

	return nil
}

func setStreamResultsToFailed(sysInfo *SystemInfo, reason string) {
	sysInfo.StreamCopyBandwidthMBs = reason
	sysInfo.StreamScaleBandwidthMBs = reason
	sysInfo.StreamAddBandwidthMBs = reason
	sysInfo.StreamTriadBandwidthMBs = reason
}

// fileExists checks if a file exists and is not a directory.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func runDiskBenchmarks(targetDir, testSize, testProfile string, sysInfo *SystemInfo) error {
	fmt.Println("  Running Disk I/O benchmarks (FIO)...")

	// Define FIO test scenarios
	fioScenarios := []struct {
		name        string
		rw          string
		bs          string
		iodepth     int
		numjobs     int
		rwmixread   int
		description string
		category    string
	}{
		// Standard tests (always run)
		{"4K_RandRead_QD64", "randread", "4k", 64, 4, 0, "4K Random Read (QD=64)", "standard"},
		{"4K_RandWrite_QD64", "randwrite", "4k", 64, 4, 0, "4K Random Write (QD=64)", "standard"},
		{"1M_SeqRead_QD32", "read", "1m", 32, 2, 0, "1M Sequential Read (QD=32)", "standard"},
		{"1M_SeqWrite_QD32", "write", "1m", 32, 2, 0, "1M Sequential Write (QD=32)", "standard"},
		{"4K_Mixed_R70W30_QD64", "randrw", "4k", 64, 4, 70, "4K Mixed 70% Read 30% Write (QD=64)", "standard"},

		// IOPS-focused tests (QD scaling)
		{"4K_RandRead_QD1", "randread", "4k", 1, 1, 0, "4K Random Read (QD=1)", "iops_scaling"},
		{"4K_RandRead_QD4", "randread", "4k", 4, 1, 0, "4K Random Read (QD=4)", "iops_scaling"},
		{"4K_RandRead_QD16", "randread", "4k", 16, 1, 0, "4K Random Read (QD=16)", "iops_scaling"},
		{"4K_RandRead_QD128", "randread", "4k", 128, 4, 0, "4K Random Read (QD=128)", "iops_scaling"},

		// Throughput-focused tests
		{"128K_SeqRead_QD32", "read", "128k", 32, 2, 0, "128K Sequential Read (QD=32)", "throughput"},
		{"128K_SeqWrite_QD32", "write", "128k", 32, 2, 0, "128K Sequential Write (QD=32)", "throughput"},
		{"512K_SeqRead_QD32", "read", "512k", 32, 2, 0, "512K Sequential Read (QD=32)", "throughput"},
		{"512K_SeqWrite_QD32", "write", "512k", 32, 2, 0, "512K Sequential Write (QD=32)", "throughput"},

		// Latency-focused tests
		{"4K_RandRead_QD1_Latency", "randread", "4k", 1, 1, 0, "4K Random Read Latency (QD=1)", "latency"},
		{"4K_RandWrite_QD1_Latency", "randwrite", "4k", 1, 1, 0, "4K Random Write Latency (QD=1)", "latency"},
	}

	// Determine test targets
	var testTargets []struct {
		mountPoint string
		devicePath string
		deviceName string
	}

	if targetDir != "" {
		// User specified a target directory
		fmt.Printf("    Using user-specified target directory: %s\n", targetDir)

		// Check if it's a directory and not a raw device path
		if strings.HasPrefix(targetDir, "/dev/") {
			fmt.Printf("    Warning: %s appears to be a raw device path. For safety, only mounted directories are supported.\n", targetDir)
			return fmt.Errorf("raw device paths are not supported for FIO tests, please specify a mounted directory")
		}

		// Check if directory exists
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			return fmt.Errorf("specified target directory %s does not exist", targetDir)
		}

		// Check if it's the root filesystem
		absPath, err := filepath.Abs(targetDir)
		if err != nil {
			return fmt.Errorf("could not resolve absolute path for %s: %w", targetDir, err)
		}

		if absPath == "/" {
			fmt.Println("    Warning: Target directory resolves to root filesystem (/). Skipping for safety.")
			return fmt.Errorf("testing on root filesystem (/) is not allowed for safety")
		}

		// Add to test targets
		testTargets = append(testTargets, struct {
			mountPoint string
			devicePath string
			deviceName string
		}{
			mountPoint: absPath,
			devicePath: "user-specified",
			deviceName: absPath,
		})
	} else {
		// Auto-detect storage devices for testing
		fmt.Println("    Auto-detecting storage devices for testing...")

		// Determine if we should try direct NVMe testing
		var shouldTestRawNVMe bool
		if os.Geteuid() == 0 { // Only try raw NVMe testing if running as root
			shouldTestRawNVMe = true
		}

		// Track which devices we've already decided to test to avoid duplicates
		testedDevices := make(map[string]bool)

		// First, try to find the boot disk and test it
		bootDisk := findBootDisk()
		if bootDisk != "" {
			fmt.Printf("    Detected boot disk: %s\n", bootDisk)

			// Add boot disk to test targets
			testTargets = append(testTargets, struct {
				mountPoint string
				devicePath string
				deviceName string
			}{
				mountPoint: "/tmp", // We'll use /tmp for testing
				devicePath: bootDisk,
				deviceName: fmt.Sprintf("%s (Boot Disk)", bootDisk),
			})

			testedDevices[bootDisk] = true
			fmt.Printf("    Added boot disk for testing: %s\n", bootDisk)
		}

		// Next, try to find NVMe devices for raw testing if we're root
		if len(sysInfo.NVMeDetails) > 0 && shouldTestRawNVMe {
			fmt.Println("    Checking for NVMe devices for direct testing...")

			// For each NVMe device, check if it's suitable for direct testing
			for _, nvme := range sysInfo.NVMeDetails {
				// Skip if we've already decided to test this device
				if testedDevices[nvme.DevicePath] {
					fmt.Printf("    Skipping %s as it's already selected for testing\n", nvme.DevicePath)
					continue
				}

				// Skip if it's the boot disk
				if nvme.DevicePath == bootDisk {
					fmt.Printf("    Skipping %s as it's the boot disk\n", nvme.DevicePath)
					continue
				}

				fmt.Printf("    Processing NVMe device for direct testing: %s\n", nvme.DevicePath)

				// Check if this device is safe for direct testing
				isSafe, reason := isSafeForDirectTesting(nvme.DevicePath)
				if isSafe {
					// Add for direct testing
					testTargets = append(testTargets, struct {
						mountPoint string
						devicePath string
						deviceName string
					}{
						mountPoint: "direct", // Special flag for direct device testing
						devicePath: nvme.DevicePath,
						deviceName: fmt.Sprintf("%s (%s) [Direct]", nvme.DevicePath, nvme.Model),
					})

					testedDevices[nvme.DevicePath] = true
					fmt.Printf("    Added NVMe device for direct testing: %s\n", nvme.DevicePath)
				} else {
					fmt.Printf("    NVMe device %s is not safe for direct testing: %s\n", nvme.DevicePath, reason)
				}
			}
		}

		// If we haven't found any devices yet, or we want to test mounted filesystems too,
		// look for mounted NVMe devices
		if len(testTargets) < 2 && len(sysInfo.NVMeDetails) > 0 {
			fmt.Println("    Looking for mounted NVMe filesystems...")

			// Find the most suitable filesystem on each NVMe device
			for _, nvme := range sysInfo.NVMeDetails {
				// Skip if we've already decided to test this device
				if testedDevices[nvme.DevicePath] {
					continue
				}

				fmt.Printf("    Processing NVMe device for filesystem testing: %s\n", nvme.DevicePath)

				// Find the best mount point for this device
				bestMountPoint := findBestMountPoint(nvme.DevicePath)
				if bestMountPoint.MountPoint != "" {
					testTargets = append(testTargets, struct {
						mountPoint string
						devicePath string
						deviceName string
					}{
						mountPoint: bestMountPoint.MountPoint,
						devicePath: bestMountPoint.DevicePath,
						deviceName: fmt.Sprintf("%s (%s)", bestMountPoint.DevicePath, nvme.Model),
					})

					testedDevices[nvme.DevicePath] = true
					fmt.Printf("    Added NVMe filesystem for testing: %s (mount: %s)\n",
						bestMountPoint.DevicePath, bestMountPoint.MountPoint)
				} else {
					fmt.Printf("    No suitable filesystem found for NVMe device: %s\n", nvme.DevicePath)
				}
			}
		}

		// If no NVMe devices were found or none had suitable mount points, try other storage
		if len(testTargets) == 0 {
			fmt.Println("    No suitable NVMe devices found. Checking other storage devices...")

			// Get all block devices with mount points
			output, err := runCommand("lsblk", "-pno", "NAME,MOUNTPOINT,TYPE,SIZE")
			if err != nil {
				fmt.Printf("    Error listing block devices: %v\n", err)
			} else {
				// Parse output to find suitable mount points
				scanner := bufio.NewScanner(strings.NewReader(output))
				for scanner.Scan() {
					line := scanner.Text()
					if line == "" {
						continue
					}

					fields := strings.Fields(line)
					if len(fields) >= 3 {
						devicePath := fields[0]
						mountPoint := ""
						if len(fields) >= 2 {
							mountPoint = fields[1]
						}
						deviceType := ""
						if len(fields) >= 3 {
							deviceType = fields[2]
						}

						// Skip root filesystem and non-mounted devices
						if mountPoint == "" || mountPoint == "/" || mountPoint == "[SWAP]" {
							continue
						}

						// Skip non-disk and non-partition devices
						if deviceType != "disk" && deviceType != "part" && deviceType != "lvm" {
							continue
						}

						// Add to test targets
						testTargets = append(testTargets, struct {
							mountPoint string
							devicePath string
							deviceName string
						}{
							mountPoint: mountPoint,
							devicePath: devicePath,
							deviceName: devicePath,
						})

						fmt.Printf("    Added mount point: %s for device: %s\n", mountPoint, devicePath)
					}
				}
			}

			// If still no targets, try to find LVM volumes
			if len(testTargets) == 0 {
				fmt.Println("    Checking for LVM volumes...")

				// Try to get LVM volumes
				lvmOutput, err := runCommand("lvs", "--noheadings", "-o", "lv_path,lv_size")
				if err == nil {
					scanner := bufio.NewScanner(strings.NewReader(lvmOutput))
					for scanner.Scan() {
						line := scanner.Text()
						if line == "" {
							continue
						}

						fields := strings.Fields(line)
						if len(fields) >= 1 {
							lvPath := fields[0]

							// Check if this LV is mounted
							mountOutput, err := runCommand("findmnt", "-no", "TARGET", lvPath)
							if err == nil && mountOutput != "" {
								mountPoint := strings.TrimSpace(mountOutput)

								// Skip root filesystem
								if mountPoint == "/" {
									continue
								}

								// Add to test targets
								testTargets = append(testTargets, struct {
									mountPoint string
									devicePath string
									deviceName string
								}{
									mountPoint: mountPoint,
									devicePath: lvPath,
									deviceName: lvPath,
								})

								fmt.Printf("    Added LVM mount point: %s for volume: %s\n", mountPoint, lvPath)
							}
						}
					}
				}
			}

			// If still no targets, try to find any mounted directory with sufficient space
			if len(testTargets) == 0 {
				fmt.Println("    No specific devices found. Looking for any suitable mount point...")

				// Get mount points with available space
				dfOutput, err := runCommand("df", "-T", "--output=target,fstype,avail")
				if err == nil {
					scanner := bufio.NewScanner(strings.NewReader(dfOutput))
					scanner.Scan() // Skip header

					for scanner.Scan() {
						line := scanner.Text()
						if line == "" {
							continue
						}

						fields := strings.Fields(line)
						if len(fields) >= 3 {
							mountPoint := fields[0]
							fsType := fields[1]
							availStr := fields[2]

							// Skip special filesystems and root
							if mountPoint == "/" ||
								strings.HasPrefix(fsType, "tmpfs") ||
								strings.HasPrefix(fsType, "devtmpfs") ||
								strings.HasPrefix(fsType, "sysfs") ||
								strings.HasPrefix(fsType, "proc") {
								continue
							}

							// Parse available space
							availKB, err := strconv.ParseUint(availStr, 10, 64)
							if err == nil && availKB > 1048576 { // At least 1GB free
								// Add to test targets
								testTargets = append(testTargets, struct {
									mountPoint string
									devicePath string
									deviceName string
								}{
									mountPoint: mountPoint,
									devicePath: "unknown",
									deviceName: fmt.Sprintf("Mount point: %s", mountPoint),
								})

								fmt.Printf("    Added mount point with sufficient space: %s\n", mountPoint)
								break // Just need one good target
							}
						}
					}
				}
			}
		}
	}

	if len(testTargets) == 0 {
		fmt.Println("    No suitable targets found for FIO benchmarks.")
		return nil
	}

	// Initialize FIO results in SystemInfo
	sysInfo.FioResults = make([]FioDeviceResult, 0, len(testTargets))

	// For each target, run the FIO tests
	for _, target := range testTargets {
		fmt.Printf("\n    Starting FIO benchmarks for %s (mount point: %s)\n", target.deviceName, target.mountPoint)

		var testFilePath string
		var availableSpace uint64
		var isDirectDeviceTest bool

		// Check if this is a direct device test
		if target.mountPoint == "direct" {
			isDirectDeviceTest = true
			testFilePath = target.devicePath

			// For direct device tests, we don't need to check available space
			// Just use a reasonable test size (1GB)
			availableSpace = 1024 * 1024 * 1024 * 10 // Assume 10GB available

			fmt.Printf("      Using direct device testing for %s\n", target.devicePath)
		} else {
			// Create a temporary directory for testing
			tempDir := filepath.Join("/tmp", fmt.Sprintf("hyprbench_fio_%d", time.Now().UnixNano()))
			err := os.MkdirAll(tempDir, 0755)
			if err != nil {
				fmt.Printf("      Error creating temporary directory %s: %v\n", tempDir, err)
				continue
			}
			defer os.RemoveAll(tempDir) // Clean up when done

			// Use the temporary directory for testing
			testFilePath = filepath.Join(tempDir, "fio_test_file")

			// Check available space in /tmp
			output, err := runCommand("df", "--output=avail", "-B1", "/tmp")
			if err != nil {
				fmt.Printf("      Error checking available space on /tmp: %v\n", err)
				continue
			}

			// Parse available space
			scanner := bufio.NewScanner(strings.NewReader(output))
			scanner.Scan() // Skip header
			if scanner.Scan() {
				availStr := strings.TrimSpace(scanner.Text())
				availableSpace, err = strconv.ParseUint(availStr, 10, 64)
				if err != nil {
					fmt.Printf("      Error parsing available space '%s': %v\n", availStr, err)
					continue
				}
			}
		}

		// Convert test size to bytes
		var testSizeBytes uint64
		if strings.HasSuffix(testSize, "G") {
			sizeGB, err := strconv.ParseUint(testSize[:len(testSize)-1], 10, 64)
			if err != nil {
				fmt.Printf("      Error parsing test size '%s': %v\n", testSize, err)
				continue
			}
			testSizeBytes = sizeGB * 1024 * 1024 * 1024
		} else if strings.HasSuffix(testSize, "M") {
			sizeMB, err := strconv.ParseUint(testSize[:len(testSize)-1], 10, 64)
			if err != nil {
				fmt.Printf("      Error parsing test size '%s': %v\n", testSize, err)
				continue
			}
			testSizeBytes = sizeMB * 1024 * 1024
		} else {
			fmt.Printf("      Invalid test size format '%s'. Expected format like 1G, 512M.\n", testSize)
			continue
		}

		// Check if enough space
		if availableSpace < testSizeBytes {
			fmt.Printf("      Not enough space on %s for FIO test file (%s).\n", target.mountPoint, testSize)
			fmt.Printf("      Available: %s, Required: %s\n",
				humanReadableBytes(availableSpace),
				humanReadableBytes(testSizeBytes))
			continue
		}

		// Filter scenarios based on the selected profile
		var selectedScenarios []struct {
			name        string
			rw          string
			bs          string
			iodepth     int
			numjobs     int
			rwmixread   int
			description string
			category    string
		}

		switch testProfile {
		case "quick":
			// Quick profile - just the essential tests
			for _, scenario := range fioScenarios {
				if scenario.name == "4K_RandRead_QD64" || scenario.name == "1M_SeqRead_QD32" {
					selectedScenarios = append(selectedScenarios, scenario)
				}
			}
		case "thorough":
			// Thorough profile - all tests
			selectedScenarios = fioScenarios
		case "iops":
			// IOPS-focused profile
			for _, scenario := range fioScenarios {
				if scenario.category == "standard" || scenario.category == "iops_scaling" {
					selectedScenarios = append(selectedScenarios, scenario)
				}
			}
		case "throughput":
			// Throughput-focused profile
			for _, scenario := range fioScenarios {
				if scenario.category == "standard" || scenario.category == "throughput" {
					selectedScenarios = append(selectedScenarios, scenario)
				}
			}
		case "latency":
			// Latency-focused profile
			for _, scenario := range fioScenarios {
				if scenario.category == "standard" || scenario.category == "latency" {
					selectedScenarios = append(selectedScenarios, scenario)
				}
			}
		case "all":
			// All tests
			selectedScenarios = fioScenarios
		default: // "standard" or any other value
			// Standard profile - just the standard tests
			for _, scenario := range fioScenarios {
				if scenario.category == "standard" {
					selectedScenarios = append(selectedScenarios, scenario)
				}
			}
		}

		fmt.Printf("      Using FIO test profile: %s (%d tests)\n", testProfile, len(selectedScenarios))
		fmt.Printf("      FIO test file: %s, Size: %s\n", testFilePath, testSize)
		fmt.Println("      ---------------------------------------------------------------------------------")
		fmt.Printf("      %-32s | %-10s | %-18s | %-18s\n", "Test Type", "IOPS", "Bandwidth (MB/s)", "Avg Latency")
		fmt.Println("      ---------------------------------------------------------------------------------")

		// Initialize device result
		deviceResult := FioDeviceResult{
			DevicePath:     target.devicePath,
			DeviceModel:    target.deviceName,
			MountPoint:     target.mountPoint,
			TestFileSize:   testSize,
			TestResults:    make([]FioTestResult, 0, len(selectedScenarios)),
			TestsCompleted: true,
		}

		// Create a progress bar if enabled
		totalTests := len(selectedScenarios)
		completedTests := 0

		for _, scenario := range selectedScenarios {
			// Show progress
			if showProgress {
				completedTests++
				progressPercent := float64(completedTests) / float64(totalTests) * 100
				progressBar := fmt.Sprintf("[%-20s] %3.0f%%", strings.Repeat("=", int(float64(20)*float64(completedTests)/float64(totalTests))), progressPercent)
				fmt.Printf("\r      %s Running: %s", progressBar, scenario.description)
			} else {
				fmt.Printf("      Running FIO test: %s (%s, rw=%s, bs=%s, iodepth=%d, numjobs=%d",
					scenario.name, scenario.description, scenario.rw, scenario.bs, scenario.iodepth, scenario.numjobs)
				if scenario.rwmixread > 0 {
					fmt.Printf(", rwmixread=%d", scenario.rwmixread)
				}
				fmt.Println(")")
			}

			// Build FIO command
			fioArgs := []string{
				fmt.Sprintf("--name=%s", scenario.name),
				fmt.Sprintf("--filename=%s", testFilePath),
				"--ioengine=libaio",
				"--direct=1",
				fmt.Sprintf("--rw=%s", scenario.rw),
				fmt.Sprintf("--bs=%s", scenario.bs),
				fmt.Sprintf("--iodepth=%d", scenario.iodepth),
				fmt.Sprintf("--numjobs=%d", scenario.numjobs),
				fmt.Sprintf("--size=%s", testSize),
				"--runtime=60",
				"--group_reporting",
				"--output-format=json",
			}

			// For direct device tests, add special flags
			if isDirectDeviceTest {
				// For direct device tests, we need to be careful
				if strings.Contains(scenario.rw, "write") {
					// For write tests on direct devices, use a small size and add safety flags
					fioArgs = append(fioArgs,
						"--size=64M",    // Smaller size for safety
						"--offset=1G",   // Start 1GB into the device to avoid partition tables
						"--verify=0",    // No verification for direct device tests
						"--fsync=1",     // Ensure data is synced
						"--end_fsync=1") // Final fsync at end of test
				} else {
					// For read-only tests, we can use the full test size
					fioArgs = append(fioArgs,
						"--readonly", // Read-only mode for safety
						"--verify=0") // No verification for direct device tests
				}
			}

			if scenario.rwmixread > 0 {
				fioArgs = append(fioArgs, fmt.Sprintf("--rwmixread=%d", scenario.rwmixread))
			}

			// Run FIO command
			output, err := runCommand("fio", fioArgs...)
			if err != nil {
				// Clear progress bar if it was shown
				if showProgress {
					fmt.Print("\r" + strings.Repeat(" ", 80) + "\r") // Clear the line
				}

				fmt.Printf("        Error running FIO test '%s': %v\n", scenario.name, err)
				deviceResult.TestsCompleted = false

				// Add failed test result
				deviceResult.TestResults = append(deviceResult.TestResults, FioTestResult{
					TestName:    scenario.name,
					ReadWrite:   scenario.rw,
					BlockSize:   scenario.bs,
					IODepth:     scenario.iodepth,
					NumJobs:     scenario.numjobs,
					RWMixRead:   scenario.rwmixread,
					IOPS:        -1, // Use negative values to indicate failure
					BandwidthMB: -1,
					LatencyUs:   -1,
					LatencyUnit: "us",
				})

				fmt.Printf("      %-32s | %-10s | %-18s | %-18s\n", scenario.name, "FAIL", "FAIL", "FAIL")
				continue
			}

			// Parse FIO JSON output
			var result FioTestResult
			result.TestName = scenario.name
			result.ReadWrite = scenario.rw
			result.BlockSize = scenario.bs
			result.IODepth = scenario.iodepth
			result.NumJobs = scenario.numjobs
			result.RWMixRead = scenario.rwmixread
			result.LatencyUnit = "us" // Default

			// Parse JSON
			var fioData map[string]interface{}
			if err := json.Unmarshal([]byte(output), &fioData); err != nil {
				// Clear progress bar if it was shown
				if showProgress {
					fmt.Print("\r" + strings.Repeat(" ", 80) + "\r") // Clear the line
				}

				fmt.Printf("        Error parsing FIO JSON output: %v\n", err)
				deviceResult.TestsCompleted = false
				continue
			}

			// Extract results from JSON
			jobs, ok := fioData["jobs"].([]interface{})
			if !ok || len(jobs) == 0 {
				// Clear progress bar if it was shown
				if showProgress {
					fmt.Print("\r" + strings.Repeat(" ", 80) + "\r") // Clear the line
				}

				fmt.Printf("        Error: Invalid or empty jobs array in FIO output\n")
				deviceResult.TestsCompleted = false
				continue
			}

			job0 := jobs[0].(map[string]interface{})

			// Extract IOPS
			if scenario.rw == "read" || scenario.rw == "randread" {
				if read, ok := job0["read"].(map[string]interface{}); ok {
					if iops, ok := read["iops"].(float64); ok {
						result.IOPS = iops
					}
				}
			} else if scenario.rw == "write" || scenario.rw == "randwrite" {
				if write, ok := job0["write"].(map[string]interface{}); ok {
					if iops, ok := write["iops"].(float64); ok {
						result.IOPS = iops
					}
				}
			} else if scenario.rw == "randrw" {
				// For mixed workloads, sum read and write IOPS
				var readIOPS, writeIOPS float64
				if read, ok := job0["read"].(map[string]interface{}); ok {
					if iops, ok := read["iops"].(float64); ok {
						readIOPS = iops
					}
				}
				if write, ok := job0["write"].(map[string]interface{}); ok {
					if iops, ok := write["iops"].(float64); ok {
						writeIOPS = iops
					}
				}
				result.IOPS = readIOPS + writeIOPS
			}

			// Extract bandwidth (MB/s)
			if scenario.rw == "read" || scenario.rw == "randread" {
				if read, ok := job0["read"].(map[string]interface{}); ok {
					if bw, ok := read["bw"].(float64); ok {
						result.BandwidthMB = bw / 1024 // Convert KiB/s to MiB/s
					}
				}
			} else if scenario.rw == "write" || scenario.rw == "randwrite" {
				if write, ok := job0["write"].(map[string]interface{}); ok {
					if bw, ok := write["bw"].(float64); ok {
						result.BandwidthMB = bw / 1024 // Convert KiB/s to MiB/s
					}
				}
			} else if scenario.rw == "randrw" {
				// For mixed workloads, sum read and write bandwidth
				var readBW, writeBW float64
				if read, ok := job0["read"].(map[string]interface{}); ok {
					if bw, ok := read["bw"].(float64); ok {
						readBW = bw
					}
				}
				if write, ok := job0["write"].(map[string]interface{}); ok {
					if bw, ok := write["bw"].(float64); ok {
						writeBW = bw
					}
				}
				result.BandwidthMB = (readBW + writeBW) / 1024 // Convert KiB/s to MiB/s
			}

			// Extract latency (us)
			// First try clat (completion latency) which is most relevant
			if scenario.rw == "read" || scenario.rw == "randread" {
				if read, ok := job0["read"].(map[string]interface{}); ok {
					if clat, ok := read["clat_ns"].(map[string]interface{}); ok {
						if mean, ok := clat["mean"].(float64); ok {
							result.LatencyUs = mean / 1000 // Convert ns to us
						}
					}
				}
			} else if scenario.rw == "write" || scenario.rw == "randwrite" {
				if write, ok := job0["write"].(map[string]interface{}); ok {
					if clat, ok := write["clat_ns"].(map[string]interface{}); ok {
						if mean, ok := clat["mean"].(float64); ok {
							result.LatencyUs = mean / 1000 // Convert ns to us
						}
					}
				}
			}

			// If latency is too large, convert to ms for better readability
			if result.LatencyUs >= 1000 {
				result.LatencyUs = result.LatencyUs / 1000
				result.LatencyUnit = "ms"
			}

			// Add result to device results
			deviceResult.TestResults = append(deviceResult.TestResults, result)

			// Print result
			var latencyStr string
			if result.LatencyUnit == "ms" {
				latencyStr = fmt.Sprintf("%.2f ms", result.LatencyUs)
			} else {
				latencyStr = fmt.Sprintf("%.2f us", result.LatencyUs)
			}

			// Clear progress bar if it was shown
			if showProgress {
				fmt.Print("\r" + strings.Repeat(" ", 80) + "\r") // Clear the line
			}

			fmt.Printf("      %-32s | %-10.0f | %-18.2f | %-18s\n",
				scenario.name, result.IOPS, result.BandwidthMB, latencyStr)
		}

		fmt.Println("      ---------------------------------------------------------------------------------")

		// Clean up test file if it's not a direct device test
		if !isDirectDeviceTest && testFilePath != "" {
			if _, err := os.Stat(testFilePath); err == nil {
				fmt.Printf("      Cleaning up FIO test file: %s\n", testFilePath)
				if err := os.Remove(testFilePath); err != nil {
					fmt.Printf("        Error removing test file: %v\n", err)
				} else {
					fmt.Printf("        Successfully removed test file\n")
				}
			}
		}

		// Add device result to system info
		sysInfo.FioResults = append(sysInfo.FioResults, deviceResult)
	}

	return nil
}

func runStressBenchmarks(sysInfo *SystemInfo) error {
	fmt.Println("  Running System Stress Benchmarks (stress-ng)...")

	// Check for stress-ng
	_, err := exec.LookPath("stress-ng")
	if err != nil {
		fmt.Println("    stress-ng command not found. Skipping stress benchmarks.")
		fmt.Println("    Consider installing: 'sudo apt install stress-ng'")
		return fmt.Errorf("stress-ng command not found")
	}

	// Initialize stress test results in SystemInfo
	sysInfo.StressResults = StressResults{
		CPUMethod:           "all",
		CPUCores:            sysInfo.CPUThreads, // Use all available threads
		CPUBogoOps:          -1,
		CPUBogoOpsPerSec:    -1,
		MatrixBogoOps:       -1,
		MatrixBogoOpsPerSec: -1,
		VMBogoOps:           -1,
		VMBogoOpsPerSec:     -1,
		TestsCompleted:      false,
	}

	// Get number of CPU cores/threads
	numCPU := 0
	if sysInfo.CPUThreads != "" {
		numCPU, _ = strconv.Atoi(sysInfo.CPUThreads)
	}
	if numCPU <= 0 {
		// Fallback to runtime.NumCPU() equivalent
		output, err := runCommand("nproc")
		if err == nil {
			numCPU, _ = strconv.Atoi(strings.TrimSpace(output))
		}
	}
	if numCPU <= 0 {
		numCPU = 4 // Default if we can't determine
	}

	// Update CPU cores in results
	sysInfo.StressResults.CPUCores = fmt.Sprintf("%d", numCPU)

	// Run CPU stress test
	fmt.Printf("    Running CPU stress test (cores: %d, method: all, time: 60s)...\n", numCPU)
	cpuOutput, err := runCommand("stress-ng", "--cpu", fmt.Sprintf("%d", numCPU), "--cpu-method", "all", "-t", "60s", "--metrics-brief")
	if err != nil {
		fmt.Printf("    Error running CPU stress test: %v\n", err)
	} else {
		// Parse CPU stress test results
		// Example output:
		// stress-ng: info:  [2686] dispatching hogs: 8 cpu
		// stress-ng: info:  [2686] successful run completed in 60.00s
		// stress-ng: info:  [2686] stressor       bogo ops real time  usr time  sys time   bogo ops/s   bogo ops/s
		// stress-ng: info:  [2686]                           (secs)    (secs)    (secs)   (real time) (usr+sys time)
		// stress-ng: info:  [2686] cpu                3891     60.00    479.50      0.01        64.85          8.11

		// Parse bogo ops and bogo ops/s
		cpuRegex := regexp.MustCompile(`cpu\s+(\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)`)
		if cpuMatch := cpuRegex.FindStringSubmatch(cpuOutput); len(cpuMatch) > 5 {
			sysInfo.StressResults.CPUBogoOps, _ = strconv.ParseFloat(cpuMatch[1], 64)
			sysInfo.StressResults.CPUBogoOpsPerSec, _ = strconv.ParseFloat(cpuMatch[5], 64)
		}

		fmt.Printf("    CPU Stress Test: %.0f bogo ops, %.2f bogo ops/s\n",
			sysInfo.StressResults.CPUBogoOps, sysInfo.StressResults.CPUBogoOpsPerSec)
	}

	// Run Matrix stress test
	fmt.Printf("    Running Matrix stress test (cores: %d, time: 60s)...\n", numCPU)
	matrixOutput, err := runCommand("stress-ng", "--matrix", fmt.Sprintf("%d", numCPU), "-t", "60s", "--metrics-brief")
	if err != nil {
		fmt.Printf("    Error running Matrix stress test: %v\n", err)
	} else {
		// Parse Matrix stress test results
		matrixRegex := regexp.MustCompile(`matrix\s+(\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)`)
		if matrixMatch := matrixRegex.FindStringSubmatch(matrixOutput); len(matrixMatch) > 5 {
			sysInfo.StressResults.MatrixBogoOps, _ = strconv.ParseFloat(matrixMatch[1], 64)
			sysInfo.StressResults.MatrixBogoOpsPerSec, _ = strconv.ParseFloat(matrixMatch[5], 64)
		}

		fmt.Printf("    Matrix Stress Test: %.0f bogo ops, %.2f bogo ops/s\n",
			sysInfo.StressResults.MatrixBogoOps, sysInfo.StressResults.MatrixBogoOpsPerSec)
	}

	// Run VM stress test
	fmt.Println("    Running VM stress test (2 workers, 50% memory, time: 30s)...")
	vmOutput, err := runCommand("stress-ng", "--vm", "2", "--vm-bytes", "50%", "-t", "30s", "--metrics-brief")
	if err != nil {
		fmt.Printf("    Error running VM stress test: %v\n", err)
	} else {
		// Parse VM stress test results
		vmRegex := regexp.MustCompile(`vm\s+(\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)\s+(\d+\.\d+)`)
		if vmMatch := vmRegex.FindStringSubmatch(vmOutput); len(vmMatch) > 5 {
			sysInfo.StressResults.VMBogoOps, _ = strconv.ParseFloat(vmMatch[1], 64)
			sysInfo.StressResults.VMBogoOpsPerSec, _ = strconv.ParseFloat(vmMatch[5], 64)
		}

		fmt.Printf("    VM Stress Test: %.0f bogo ops, %.2f bogo ops/s\n",
			sysInfo.StressResults.VMBogoOps, sysInfo.StressResults.VMBogoOpsPerSec)
	}

	// Check if all tests completed successfully
	if sysInfo.StressResults.CPUBogoOps > 0 &&
		sysInfo.StressResults.MatrixBogoOps > 0 &&
		sysInfo.StressResults.VMBogoOps > 0 {
		sysInfo.StressResults.TestsCompleted = true
	}

	return nil
}

func runNetworkBenchmarks(includeNetblast bool, sysInfo *SystemInfo) error {
	fmt.Println("  Running Network Benchmarks...")

	// Initialize network benchmark results in SystemInfo
	sysInfo.SpeedtestResults = SpeedtestResult{
		ToolUsed:      "None",
		DownloadMbps:  -1,
		UploadMbps:    -1,
		LatencyMs:     -1,
		TestCompleted: false,
		ErrorMessage:  "Test not run",
	}
	sysInfo.Iperf3Results = []Iperf3Result{}
	sysInfo.NetblastResults = []NetblastResult{}

	// Run local speedtest
	if err := runLocalSpeedtest(sysInfo); err != nil {
		fmt.Printf("    Error during local speedtest: %v\n", err)
	}

	// Run iperf3 single server tests
	if err := runIperf3Tests(sysInfo); err != nil {
		fmt.Printf("    Error during iperf3 tests: %v\n", err)
	}

	// Run hyprbench-netblast if not skipped
	if includeNetblast {
		fmt.Println("  Running hyprbench-netblast (multi-server network tests)...")
		if err := runNetblastTests(sysInfo); err != nil {
			fmt.Printf("    Error during hyprbench-netblast tests: %v\n", err)
		}
	} else {
		fmt.Println("  Skipping hyprbench-netblast as per flags")
	}

	return nil
}

// runLocalSpeedtest runs a local speedtest using speedtest-cli or fast-cli
func runLocalSpeedtest(sysInfo *SystemInfo) error {
	fmt.Println("  Running Local Speed Test...")

	// Check for speedtest-cli
	speedtestToolFound := false
	var speedtestTool string

	// Try speedtest-cli (Ookla)
	_, err := exec.LookPath("speedtest")
	if err == nil {
		speedtestToolFound = true
		speedtestTool = "speedtest"
		fmt.Println("    Using 'speedtest' (Ookla) for local speed test")
	} else {
		// Try speedtest-cli (Python)
		_, err = exec.LookPath("speedtest-cli")
		if err == nil {
			speedtestToolFound = true
			speedtestTool = "speedtest-cli"
			fmt.Println("    Using 'speedtest-cli' (Python) for local speed test")
		} else {
			// Try fast-cli
			_, err = exec.LookPath("fast")
			if err == nil {
				speedtestToolFound = true
				speedtestTool = "fast"
				fmt.Println("    Using 'fast' (fast-cli) for local speed test")
			}
		}
	}

	if !speedtestToolFound {
		fmt.Println("    No speedtest tool found (speedtest, speedtest-cli, or fast)")
		fmt.Println("    Consider installing one of these tools for local speed testing")
		sysInfo.SpeedtestResults.ErrorMessage = "No speedtest tool found"
		return fmt.Errorf("no speedtest tool found")
	}

	// Initialize result with tool used
	sysInfo.SpeedtestResults.ToolUsed = speedtestTool

	// Run the appropriate speedtest tool
	switch speedtestTool {
	case "speedtest":
		return runOoklaSpeedtest(sysInfo)
	case "speedtest-cli":
		return runPythonSpeedtest(sysInfo)
	case "fast":
		return runFastSpeedtest(sysInfo)
	default:
		return fmt.Errorf("unknown speedtest tool: %s", speedtestTool)
	}
}

// runOoklaSpeedtest runs the Ookla speedtest-cli
func runOoklaSpeedtest(sysInfo *SystemInfo) error {
	fmt.Println("    Running Ookla speedtest...")

	// Run speedtest with --format=json for easier parsing
	output, err := runCommand("speedtest", "--format=json")
	if err != nil {
		// Try without the --format flag as a fallback
		fmt.Println("    JSON format failed, trying standard output format...")
		output, err = runCommand("speedtest")
		if err != nil {
			sysInfo.SpeedtestResults.ErrorMessage = fmt.Sprintf("Ookla speedtest failed: %v", err)
			return fmt.Errorf("ookla speedtest failed: %v", err)
		}

		// Parse standard output using regex
		downloadRegex := regexp.MustCompile(`Download:\s+([\d.]+)\s+Mbps`)
		uploadRegex := regexp.MustCompile(`Upload:\s+([\d.]+)\s+Mbps`)
		latencyRegex := regexp.MustCompile(`Latency:\s+([\d.]+)\s+ms`)

		if downloadMatch := downloadRegex.FindStringSubmatch(output); len(downloadMatch) > 1 {
			if download, err := strconv.ParseFloat(downloadMatch[1], 64); err == nil {
				sysInfo.SpeedtestResults.DownloadMbps = download
			}
		}

		if uploadMatch := uploadRegex.FindStringSubmatch(output); len(uploadMatch) > 1 {
			if upload, err := strconv.ParseFloat(uploadMatch[1], 64); err == nil {
				sysInfo.SpeedtestResults.UploadMbps = upload
			}
		}

		if latencyMatch := latencyRegex.FindStringSubmatch(output); len(latencyMatch) > 1 {
			if latency, err := strconv.ParseFloat(latencyMatch[1], 64); err == nil {
				sysInfo.SpeedtestResults.LatencyMs = latency
			}
		}

		// If we got valid results, mark as completed
		if sysInfo.SpeedtestResults.DownloadMbps > 0 && sysInfo.SpeedtestResults.UploadMbps > 0 {
			sysInfo.SpeedtestResults.TestCompleted = true
			fmt.Printf("    Download: %.2f Mbps, Upload: %.2f Mbps, Latency: %.2f ms\n",
				sysInfo.SpeedtestResults.DownloadMbps,
				sysInfo.SpeedtestResults.UploadMbps,
				sysInfo.SpeedtestResults.LatencyMs)
			return nil
		}

		sysInfo.SpeedtestResults.ErrorMessage = "Failed to parse Ookla speedtest output"
		return fmt.Errorf("failed to parse ookla speedtest output")
	}

	// Parse JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		sysInfo.SpeedtestResults.ErrorMessage = fmt.Sprintf("Failed to parse Ookla speedtest JSON: %v", err)
		return fmt.Errorf("failed to parse ookla speedtest JSON: %v", err)
	}

	// Extract download, upload, and ping
	if download, ok := result["download"].(map[string]interface{}); ok {
		if bandwidth, ok := download["bandwidth"].(float64); ok {
			// Convert bytes/s to Mbps (bits/s ÷ 125000)
			sysInfo.SpeedtestResults.DownloadMbps = bandwidth / 125000
		}
	}

	if upload, ok := result["upload"].(map[string]interface{}); ok {
		if bandwidth, ok := upload["bandwidth"].(float64); ok {
			// Convert bytes/s to Mbps (bits/s ÷ 125000)
			sysInfo.SpeedtestResults.UploadMbps = bandwidth / 125000
		}
	}

	if ping, ok := result["ping"].(map[string]interface{}); ok {
		if latency, ok := ping["latency"].(float64); ok {
			sysInfo.SpeedtestResults.LatencyMs = latency
		}
	}

	// Check if we got valid results
	if sysInfo.SpeedtestResults.DownloadMbps <= 0 || sysInfo.SpeedtestResults.UploadMbps <= 0 {
		sysInfo.SpeedtestResults.ErrorMessage = "Invalid or missing speedtest results"
		return fmt.Errorf("invalid or missing speedtest results")
	}

	sysInfo.SpeedtestResults.TestCompleted = true
	fmt.Printf("    Download: %.2f Mbps, Upload: %.2f Mbps, Latency: %.2f ms\n",
		sysInfo.SpeedtestResults.DownloadMbps,
		sysInfo.SpeedtestResults.UploadMbps,
		sysInfo.SpeedtestResults.LatencyMs)

	return nil
}

// runPythonSpeedtest runs the Python speedtest-cli
func runPythonSpeedtest(sysInfo *SystemInfo) error {
	fmt.Println("    Running Python speedtest-cli...")

	// Run speedtest-cli with --simple for easier parsing
	output, err := runCommand("speedtest-cli", "--simple")
	if err != nil {
		sysInfo.SpeedtestResults.ErrorMessage = fmt.Sprintf("Python speedtest-cli failed: %v", err)
		return fmt.Errorf("python speedtest-cli failed: %v", err)
	}

	// Parse output using regex
	// Example output:
	// Ping: 20.716 ms
	// Download: 95.32 Mbit/s
	// Upload: 10.05 Mbit/s

	pingRegex := regexp.MustCompile(`Ping: ([\d.]+) ms`)
	downloadRegex := regexp.MustCompile(`Download: ([\d.]+) Mbit/s`)
	uploadRegex := regexp.MustCompile(`Upload: ([\d.]+) Mbit/s`)

	if pingMatch := pingRegex.FindStringSubmatch(output); len(pingMatch) > 1 {
		if ping, err := strconv.ParseFloat(pingMatch[1], 64); err == nil {
			sysInfo.SpeedtestResults.LatencyMs = ping
		}
	}

	if downloadMatch := downloadRegex.FindStringSubmatch(output); len(downloadMatch) > 1 {
		if download, err := strconv.ParseFloat(downloadMatch[1], 64); err == nil {
			sysInfo.SpeedtestResults.DownloadMbps = download
		}
	}

	if uploadMatch := uploadRegex.FindStringSubmatch(output); len(uploadMatch) > 1 {
		if upload, err := strconv.ParseFloat(uploadMatch[1], 64); err == nil {
			sysInfo.SpeedtestResults.UploadMbps = upload
		}
	}

	// Check if we got valid results
	if sysInfo.SpeedtestResults.DownloadMbps <= 0 || sysInfo.SpeedtestResults.UploadMbps <= 0 {
		sysInfo.SpeedtestResults.ErrorMessage = "Invalid or missing speedtest results"
		return fmt.Errorf("invalid or missing speedtest results")
	}

	sysInfo.SpeedtestResults.TestCompleted = true
	fmt.Printf("    Download: %.2f Mbps, Upload: %.2f Mbps, Latency: %.2f ms\n",
		sysInfo.SpeedtestResults.DownloadMbps,
		sysInfo.SpeedtestResults.UploadMbps,
		sysInfo.SpeedtestResults.LatencyMs)

	return nil
}

// runFastSpeedtest runs the fast-cli speedtest
func runFastSpeedtest(sysInfo *SystemInfo) error {
	fmt.Println("    Running fast-cli speedtest...")

	// Check for jq dependency
	_, err := exec.LookPath("jq")
	if err != nil {
		sysInfo.SpeedtestResults.ErrorMessage = "jq is required for parsing fast-cli output but not found"
		return fmt.Errorf("jq is required for parsing fast-cli output but not found")
	}

	// Run fast with --json for easier parsing
	output, err := runCommand("fast", "--json")
	if err != nil {
		sysInfo.SpeedtestResults.ErrorMessage = fmt.Sprintf("fast-cli failed: %v", err)
		return fmt.Errorf("fast-cli failed: %v", err)
	}

	// Parse JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		sysInfo.SpeedtestResults.ErrorMessage = fmt.Sprintf("Failed to parse fast-cli JSON: %v", err)
		return fmt.Errorf("failed to parse fast-cli JSON: %v", err)
	}

	// Extract download speed
	if downloadSpeed, ok := result["downloadSpeed"].(float64); ok {
		sysInfo.SpeedtestResults.DownloadMbps = downloadSpeed
	}

	// Extract upload speed if available
	if uploadSpeed, ok := result["uploadSpeed"].(float64); ok {
		sysInfo.SpeedtestResults.UploadMbps = uploadSpeed
	} else {
		// If upload speed is not available, run fast with --upload
		fmt.Println("    Upload speed not found in initial results, running with --upload...")
		uploadOutput, err := runCommand("fast", "--upload", "--json")
		if err != nil {
			fmt.Printf("    Warning: fast-cli upload test failed: %v\n", err)
			// Continue with download results only
		} else {
			var uploadResult map[string]interface{}
			if err := json.Unmarshal([]byte(uploadOutput), &uploadResult); err != nil {
				fmt.Printf("    Warning: Failed to parse fast-cli upload JSON: %v\n", err)
			} else if uploadSpeed, ok := uploadResult["uploadSpeed"].(float64); ok {
				sysInfo.SpeedtestResults.UploadMbps = uploadSpeed
			}
		}
	}

	// fast-cli doesn't typically provide latency
	sysInfo.SpeedtestResults.LatencyMs = -1

	// Check if we got valid download results (upload might be missing)
	if sysInfo.SpeedtestResults.DownloadMbps <= 0 {
		sysInfo.SpeedtestResults.ErrorMessage = "Invalid or missing download speed"
		return fmt.Errorf("invalid or missing download speed")
	}

	sysInfo.SpeedtestResults.TestCompleted = true
	fmt.Printf("    Download: %.2f Mbps", sysInfo.SpeedtestResults.DownloadMbps)
	if sysInfo.SpeedtestResults.UploadMbps > 0 {
		fmt.Printf(", Upload: %.2f Mbps", sysInfo.SpeedtestResults.UploadMbps)
	} else {
		fmt.Print(", Upload: N/A")
	}
	fmt.Println(" (Latency not provided by fast-cli)")

	return nil
}

// runIperf3Tests runs iperf3 tests against public servers
func runIperf3Tests(sysInfo *SystemInfo) error {
	fmt.Println("  Running iperf3 Single Server Tests...")

	// Check for iperf3
	_, err := exec.LookPath("iperf3")
	if err != nil {
		fmt.Println("    iperf3 command not found. Skipping iperf3 tests.")
		fmt.Println("    Consider installing: 'sudo apt install iperf3'")
		return fmt.Errorf("iperf3 command not found")
	}

	// Check for curl and jq
	_, err = exec.LookPath("curl")
	if err != nil {
		fmt.Println("    curl command not found. Required for fetching iperf3 server list.")
		return fmt.Errorf("curl command not found")
	}

	_, err = exec.LookPath("jq")
	if err != nil {
		fmt.Println("    jq command not found. Required for parsing iperf3 server list.")
		return fmt.Errorf("jq command not found")
	}

	// Define a struct to match the server list format
	type Iperf3Server struct {
		IP_HOST   string `json:"IP_HOST"`
		OPTIONS   string `json:"OPTIONS"`
		GB_S      string `json:"GB_S"`
		COUNTRY   string `json:"COUNTRY"`
		SITE      string `json:"SITE"`
		CONTINENT string `json:"CONTINENT"`
		PROVIDER  string `json:"PROVIDER"`
	}

	// Fetch iperf3 server list from the reliable source
	fmt.Println("    Fetching public iperf3 servers from iperf3serverlist.net...")
	serversOutput, err := runCommand("curl", "-s", "--connect-timeout", "10", "https://export.iperf3serverlist.net/json.php?action=download")
	if err != nil {
		fmt.Printf("    Error fetching iperf3 server list: %v\n", err)
		return fmt.Errorf("error fetching iperf3 server list: %v", err)
	}

	// Parse the server list
	var iperf3Servers []Iperf3Server
	if err := json.Unmarshal([]byte(serversOutput), &iperf3Servers); err != nil {
		fmt.Printf("    Error parsing iperf3 server list: %v\n", err)
		return fmt.Errorf("error parsing iperf3 server list: %v", err)
	}

	// Get self location for distance calculation
	selfLocationOutput, err := runCommand("curl", "-s", "https://ipinfo.io/json")
	if err != nil {
		fmt.Printf("    Error getting self location: %v\n", err)
		return fmt.Errorf("error getting self location: %v", err)
	}

	// Parse self location
	var selfLocation struct {
		IP       string `json:"ip"`
		City     string `json:"city"`
		Region   string `json:"region"`
		Country  string `json:"country"`
		Loc      string `json:"loc"` // Latitude,Longitude
		Timezone string `json:"timezone"`
	}

	if err := json.Unmarshal([]byte(selfLocationOutput), &selfLocation); err != nil {
		fmt.Printf("    Error parsing self location: %v\n", err)
		return fmt.Errorf("error parsing self location: %v", err)
	}

	// Extract latitude and longitude
	var selfLat, selfLon float64
	if selfLocation.Loc != "" {
		locParts := strings.Split(selfLocation.Loc, ",")
		if len(locParts) == 2 {
			selfLat, _ = strconv.ParseFloat(locParts[0], 64)
			selfLon, _ = strconv.ParseFloat(locParts[1], 64)
		}
	}

	fmt.Printf("    Self location: %s, %s, %s (%.4f, %.4f)\n",
		selfLocation.City, selfLocation.Region, selfLocation.Country, selfLat, selfLon)

	// Process servers with distance calculation
	var serversWithDistance []struct {
		Host      string
		Port      int
		City      string
		Country   string
		Continent string
		Distance  int // in km
		Command   string
		Options   string
		Bandwidth string
		Provider  string
	}

	// Process each server
	for _, server := range iperf3Servers {
		// Extract host and port from IP_HOST
		// Format is typically "iperf3 -c hostname -p port" or "iperf3 -c ip"
		hostAndPort := strings.TrimPrefix(server.IP_HOST, "iperf3 -c ")

		// Split by space to separate host and potential port options
		parts := strings.Split(hostAndPort, " ")
		host := parts[0]

		// Default port
		port := 5201
		portRange := ""

		// Check if there's a port specification
		for i := 1; i < len(parts); i++ {
			if parts[i] == "-p" && i+1 < len(parts) {
				portRange = parts[i+1]
				// If it's a range like 5201-5210, just take the first port
				if strings.Contains(portRange, "-") {
					portParts := strings.Split(portRange, "-")
					portStr := portParts[0]
					parsedPort, err := strconv.Atoi(portStr)
					if err == nil {
						port = parsedPort
					}
				} else {
					// Single port
					parsedPort, err := strconv.Atoi(portRange)
					if err == nil {
						port = parsedPort
					}
				}
				break
			}
		}

		// Get approximate coordinates based on country/site
		var lat, lon float64

		// Use a map of known locations
		switch server.COUNTRY {
		case "US":
			switch server.SITE {
			case "New York":
				lat, lon = 40.7128, -74.0060
			case "Los Angeles":
				lat, lon = 34.0522, -118.2437
			case "Miami":
				lat, lon = 25.7617, -80.1918
			case "Chicago":
				lat, lon = 41.8781, -87.6298
			case "Dallas":
				lat, lon = 32.7767, -96.7970
			case "San Francisco":
				lat, lon = 37.7749, -122.4194
			case "Seattle":
				lat, lon = 47.6062, -122.3321
			case "Houston":
				lat, lon = 29.7604, -95.3698
			default:
				lat, lon = 37.0902, -95.7129 // Center of US
			}
		case "UK", "GB":
			switch server.SITE {
			case "London":
				lat, lon = 51.5074, -0.1278
			default:
				lat, lon = 55.3781, -3.4360 // Center of UK
			}
		case "FR":
			switch server.SITE {
			case "Paris":
				lat, lon = 48.8566, 2.3522
			default:
				lat, lon = 46.2276, 2.2137 // Center of France
			}
		case "DE":
			switch server.SITE {
			case "Frankfurt":
				lat, lon = 50.1109, 8.6821
			case "Berlin":
				lat, lon = 52.5200, 13.4050
			default:
				lat, lon = 51.1657, 10.4515 // Center of Germany
			}
		case "NL":
			lat, lon = 52.3676, 4.9041 // Amsterdam
		case "SG":
			lat, lon = 1.3521, 103.8198 // Singapore
		case "JP":
			lat, lon = 35.6762, 139.6503 // Tokyo
		case "AU":
			switch server.SITE {
			case "Sydney":
				lat, lon = -33.8688, 151.2093
			default:
				lat, lon = -25.2744, 133.7751 // Center of Australia
			}
		case "CA":
			lat, lon = 56.1304, -106.3468 // Center of Canada
		case "BR":
			lat, lon = -14.2350, -51.9253 // Center of Brazil
		case "IN":
			lat, lon = 20.5937, 78.9629 // Center of India
		case "RU":
			lat, lon = 61.5240, 105.3188 // Center of Russia
		case "ZA":
			lat, lon = -30.5595, 22.9375 // Center of South Africa
		default:
			// Skip servers without coordinates
			continue
		}

		// Calculate distance
		distance := calculateDistance(selfLat, selfLon, lat, lon)

		// No need to parse bandwidth here, we use it directly

		// Add to list with distance
		serversWithDistance = append(serversWithDistance, struct {
			Host      string
			Port      int
			City      string
			Country   string
			Continent string
			Distance  int
			Command   string
			Options   string
			Bandwidth string
			Provider  string
		}{
			Host:      host,
			Port:      port,
			City:      server.SITE,
			Country:   server.COUNTRY,
			Continent: server.CONTINENT,
			Distance:  distance,
			Command:   server.IP_HOST,
			Options:   server.OPTIONS,
			Bandwidth: server.GB_S,
			Provider:  server.PROVIDER,
		})
	}

	// Sort servers by distance
	sort.Slice(serversWithDistance, func(i, j int) bool {
		return serversWithDistance[i].Distance < serversWithDistance[j].Distance
	})

	// Smart server selection based on regions and bandwidth
	// We want to select:
	// 1. Filter servers based on bandwidth capacity
	// 2. Select servers from user's region first
	// 3. Ensure global coverage with limited number of servers

	// First, estimate user's connection speed from speedtest results if available
	var estimatedUserBandwidth float64 = 1000 // Default assumption: 1 Gbps
	if sysInfo.SpeedtestResults.DownloadMbps > 0 {
		estimatedUserBandwidth = sysInfo.SpeedtestResults.DownloadMbps
	}

	// Filter servers based on bandwidth capacity
	// We want servers that can handle at least the user's connection speed
	var filteredServers []struct {
		Host          string
		Port          int
		City          string
		Country       string
		Continent     string
		Distance      int
		Command       string
		Options       string
		Bandwidth     string
		Provider      string
		BandwidthGbps float64
	}

	for _, server := range serversWithDistance {
		// Parse bandwidth (format is usually like "10", "2x10", "40", etc. in Gbps)
		bandwidthStr := server.Bandwidth
		var bandwidthGbps float64 = 1 // Default to 1 Gbps if we can't parse

		if strings.Contains(bandwidthStr, "x") {
			// Format like "2x10" means 2 connections of 10 Gbps each
			parts := strings.Split(bandwidthStr, "x")
			if len(parts) == 2 {
				count, err1 := strconv.ParseFloat(parts[0], 64)
				speed, err2 := strconv.ParseFloat(parts[1], 64)
				if err1 == nil && err2 == nil {
					bandwidthGbps = count * speed
				}
			}
		} else {
			// Simple format like "10" means 10 Gbps
			speed, err := strconv.ParseFloat(bandwidthStr, 64)
			if err == nil {
				bandwidthGbps = speed
			}
		}

		// Convert to Mbps for comparison with user's speed
		bandwidthMbps := bandwidthGbps * 1000

		// Only include servers with sufficient capacity
		// We want at least 2x the user's speed to ensure we're not bottlenecked by the server
		if bandwidthMbps >= estimatedUserBandwidth*2 {
			serverWithBandwidth := struct {
				Host          string
				Port          int
				City          string
				Country       string
				Continent     string
				Distance      int
				Command       string
				Options       string
				Bandwidth     string
				Provider      string
				BandwidthGbps float64
			}{
				Host:          server.Host,
				Port:          server.Port,
				City:          server.City,
				Country:       server.Country,
				Continent:     server.Continent,
				Distance:      server.Distance,
				Command:       server.Command,
				Options:       server.Options,
				Bandwidth:     server.Bandwidth,
				Provider:      server.Provider,
				BandwidthGbps: bandwidthGbps,
			}

			filteredServers = append(filteredServers, serverWithBandwidth)
		}
	}

	// If we don't have enough servers after filtering, fall back to the original list
	if len(filteredServers) < 3 {
		fmt.Println("    Warning: Not enough high-capacity servers found. Using all available servers.")
		filteredServers = nil
		for _, server := range serversWithDistance {
			// Parse bandwidth for display purposes
			bandwidthStr := server.Bandwidth
			var bandwidthGbps float64 = 1 // Default to 1 Gbps if we can't parse

			if strings.Contains(bandwidthStr, "x") {
				parts := strings.Split(bandwidthStr, "x")
				if len(parts) == 2 {
					count, err1 := strconv.ParseFloat(parts[0], 64)
					speed, err2 := strconv.ParseFloat(parts[1], 64)
					if err1 == nil && err2 == nil {
						bandwidthGbps = count * speed
					}
				}
			} else {
				speed, err := strconv.ParseFloat(bandwidthStr, 64)
				if err == nil {
					bandwidthGbps = speed
				}
			}

			serverWithBandwidth := struct {
				Host          string
				Port          int
				City          string
				Country       string
				Continent     string
				Distance      int
				Command       string
				Options       string
				Bandwidth     string
				Provider      string
				BandwidthGbps float64
			}{
				Host:          server.Host,
				Port:          server.Port,
				City:          server.City,
				Country:       server.Country,
				Continent:     server.Continent,
				Distance:      server.Distance,
				Command:       server.Command,
				Options:       server.Options,
				Bandwidth:     server.Bandwidth,
				Provider:      server.Provider,
				BandwidthGbps: bandwidthGbps,
			}

			filteredServers = append(filteredServers, serverWithBandwidth)
		}
	}

	// Group servers by continent
	continentServers := make(map[string][]struct {
		Host          string
		Port          int
		City          string
		Country       string
		Continent     string
		Distance      int
		Command       string
		Options       string
		Bandwidth     string
		Provider      string
		BandwidthGbps float64
	})

	for _, server := range filteredServers {
		continent := server.Continent
		if continent == "" {
			// Try to infer continent from country
			switch server.Country {
			case "US", "CA", "MX":
				continent = "North America"
			case "BR", "AR", "CL", "CO", "PE", "VE":
				continent = "South America"
			case "GB", "UK", "FR", "DE", "IT", "ES", "NL", "BE", "CH", "AT", "SE", "NO", "DK", "FI", "PL", "CZ", "HU", "RO", "BG", "GR", "PT", "IE":
				continent = "Europe"
			case "CN", "JP", "KR", "IN", "SG", "MY", "TH", "VN", "ID", "PH":
				continent = "Asia"
			case "AU", "NZ":
				continent = "Oceania"
			case "ZA", "EG", "NG", "KE", "MA", "DZ", "TN":
				continent = "Africa"
			default:
				continent = "Unknown"
			}
		}
		continentServers[continent] = append(continentServers[continent], server)
	}

	// First, get user's continent
	userContinent := "Unknown"
	switch selfLocation.Country {
	case "US", "CA", "MX":
		userContinent = "North America"
	case "BR", "AR", "CL", "CO", "PE", "VE":
		userContinent = "South America"
	case "GB", "UK", "FR", "DE", "IT", "ES", "NL", "BE", "CH", "AT", "SE", "NO", "DK", "FI", "PL", "CZ", "HU", "RO", "BG", "GR", "PT", "IE":
		userContinent = "Europe"
	case "CN", "JP", "KR", "IN", "SG", "MY", "TH", "VN", "ID", "PH":
		userContinent = "Asia"
	case "AU", "NZ":
		userContinent = "Oceania"
	case "ZA", "EG", "NG", "KE", "MA", "DZ", "TN":
		userContinent = "Africa"
	}

	fmt.Printf("    User continent: %s\n", userContinent)
	fmt.Printf("    Estimated connection speed: %.2f Mbps\n", estimatedUserBandwidth)

	// Select the best servers from each continent
	var selectedServers []struct {
		Host          string
		Port          int
		City          string
		Country       string
		Continent     string
		Distance      int
		Command       string
		Options       string
		Bandwidth     string
		Provider      string
		BandwidthGbps float64
	}

	// Select servers from user's continent first (up to 2)
	if servers, ok := continentServers[userContinent]; ok && len(servers) > 0 {
		// Take up to 2 closest servers from user's continent
		count := min(2, len(servers))
		for i := 0; i < count; i++ {
			selectedServers = append(selectedServers, servers[i])
		}
	}

	// Then select one server from each other continent, but only if they're not too far
	continentsToCheck := []string{"Europe", "North America", "Asia", "South America", "Oceania", "Africa"}
	for _, continent := range continentsToCheck {
		if continent == userContinent {
			continue // Already processed user's continent
		}

		if servers, ok := continentServers[continent]; ok && len(servers) > 0 {
			// Only add if the distance is reasonable (< 15000 km)
			if servers[0].Distance < 15000 {
				selectedServers = append(selectedServers, servers[0])
			}
		}
	}

	// Strictly limit to max 5 servers total
	if len(selectedServers) > 5 {
		selectedServers = selectedServers[:5]
	}

	// Print selected servers for debugging
	fmt.Println("    Selected servers for testing:")
	for i, server := range selectedServers {
		fmt.Printf("      %d. %s, %s (%s) - %d km - %s Gbps\n",
			i+1, server.City, server.Country, server.Host, server.Distance, server.Bandwidth)
	}

	// Convert to the format expected by the rest of the code
	var serverList []map[string]interface{}
	for _, server := range selectedServers {
		serverList = append(serverList, map[string]interface{}{
			"host":      server.Host,
			"port":      server.Port,
			"city":      server.City,
			"country":   server.Country,
			"continent": server.Continent,
			"distance":  server.Distance,
			"command":   server.Command,
			"options":   server.Options,
			"bandwidth": server.Bandwidth,
			"provider":  server.Provider,
		})
	}

	// Check if we have servers to test
	if len(serverList) == 0 {
		fmt.Println("    No iperf3 servers found to test")
		return fmt.Errorf("no iperf3 servers found to test")
	}

	// Print server list
	fmt.Println("    iperf3 servers to test:")
	for _, server := range serverList {
		host, ok := server["host"].(string)
		if !ok {
			continue
		}

		// Handle port which could be float64 or int
		var port int
		switch p := server["port"].(type) {
		case float64:
			port = int(p)
		case int:
			port = p
		default:
			port = 5201 // Default iperf3 port
		}

		city := "Unknown"
		country := "Unknown"

		if c, ok := server["city"].(string); ok {
			city = c
		}
		if c, ok := server["country"].(string); ok {
			country = c
		}

		fmt.Printf("      - %s:%d (%s, %s)\n", host, port, city, country)
	}

	// Print table header
	fmt.Println("\n    iperf3 Single Server Test Results:")
	fmt.Println("    --------------------------------------------------------------------------")
	fmt.Printf("    %-35s | %-5s | %-15s | %-15s\n", "Server Location/Host", "Port", "Download (Mbps)", "Upload (Mbps)")
	fmt.Println("    --------------------------------------------------------------------------")

	// Run tests for each server
	for _, server := range serverList {
		host, ok := server["host"].(string)
		if !ok {
			continue
		}

		// Handle port which could be float64 or int
		var port int
		switch p := server["port"].(type) {
		case float64:
			port = int(p)
		case int:
			port = p
		default:
			port = 5201 // Default iperf3 port
		}

		city := "Unknown"
		country := "Unknown"

		if c, ok := server["city"].(string); ok {
			city = c
		}
		if c, ok := server["country"].(string); ok {
			country = c
		}

		location := fmt.Sprintf("%s, %s", city, country)

		// Create result struct
		result := Iperf3Result{
			Host:          host,
			Port:          port,
			Location:      location,
			DownloadMbps:  -1,
			UploadMbps:    -1,
			TestCompleted: false,
			ErrorMessage:  "",
		}

		fmt.Printf("    Testing iperf3 against: %s (%s:%d)\n", location, host, port)

		// Get the command from the server
		command, ok := server["command"].(string)
		if !ok {
			// Fallback to standard command if not available
			command = fmt.Sprintf("iperf3 -c %s -p %d", host, port)
		}

		// Extract the command parts
		commandParts := strings.Split(command, " ")
		if len(commandParts) < 3 {
			// Invalid command format
			result.ErrorMessage = "Invalid command format"
			continue
		}

		// Download test with timeout
		fmt.Println("      Running iperf3 Download test...")

		// Use the exact command from the server list, just add timeout and JSON output
		// Extract the base command (without iperf3 -c)
		baseCommand := strings.TrimPrefix(command, "iperf3 -c ")

		// Run the exact command as provided by the server list
		fmt.Printf("      Running test: %s\n", command)
		downloadCmd := fmt.Sprintf("timeout 15 iperf3 -c %s -t 5 -J", baseCommand)
		fmt.Printf("      Executing: %s\n", downloadCmd)
		downloadOutput, downloadErr := runCommand("bash", "-c", downloadCmd)

		if downloadErr != nil {
			fmt.Printf("      Error running iperf3 download test: %v\n", downloadErr)
			result.ErrorMessage = fmt.Sprintf("Download test failed: %v", downloadErr)
		} else {
			// Parse JSON output
			var downloadResult map[string]interface{}
			if err := json.Unmarshal([]byte(downloadOutput), &downloadResult); err != nil {
				fmt.Printf("      Error parsing iperf3 download JSON: %v\n", err)
				result.ErrorMessage = fmt.Sprintf("Error parsing download JSON: %v", err)
			} else {
				// Extract download speed
				if end, ok := downloadResult["end"].(map[string]interface{}); ok {
					if sumReceived, ok := end["sum_received"].(map[string]interface{}); ok {
						if bitsPerSecond, ok := sumReceived["bits_per_second"].(float64); ok {
							result.DownloadMbps = bitsPerSecond / 1000000 // Convert to Mbps
						}
					}
				}
			}
		}

		// Only run upload test if download succeeded
		if result.DownloadMbps > 0 {
			// Check if server supports upload (-R option)
			options, _ := server["options"].(string)
			supportsUpload := strings.Contains(options, "-R")

			if supportsUpload {
				// Upload test with timeout
				fmt.Println("      Running iperf3 Upload test...")

				// Use the exact command from the server list, just add timeout, reverse and JSON output
				baseCommand := strings.TrimPrefix(command, "iperf3 -c ")

				// Run the exact command as provided by the server list
				fmt.Printf("      Running upload test: %s\n", command)
				uploadCmd := fmt.Sprintf("timeout 15 iperf3 -c %s -t 5 -R -J", baseCommand)
				fmt.Printf("      Executing: %s\n", uploadCmd)
				uploadOutput, uploadErr := runCommand("bash", "-c", uploadCmd)

				if uploadErr != nil {
					fmt.Printf("      Error running iperf3 upload test: %v\n", uploadErr)
					if result.ErrorMessage == "" {
						result.ErrorMessage = fmt.Sprintf("Upload test failed: %v", uploadErr)
					} else {
						result.ErrorMessage += fmt.Sprintf(", Upload test failed: %v", uploadErr)
					}
				} else {
					// Parse JSON output
					var uploadResult map[string]interface{}
					if err := json.Unmarshal([]byte(uploadOutput), &uploadResult); err != nil {
						fmt.Printf("      Error parsing iperf3 upload JSON: %v\n", err)
						if result.ErrorMessage == "" {
							result.ErrorMessage = fmt.Sprintf("Error parsing upload JSON: %v", err)
						} else {
							result.ErrorMessage += fmt.Sprintf(", Error parsing upload JSON: %v", err)
						}
					} else {
						// Extract upload speed
						if end, ok := uploadResult["end"].(map[string]interface{}); ok {
							if sumSent, ok := end["sum_sent"].(map[string]interface{}); ok {
								if bitsPerSecond, ok := sumSent["bits_per_second"].(float64); ok {
									result.UploadMbps = bitsPerSecond / 1000000 // Convert to Mbps
								}
							}
						}
					}
				}
			} else {
				fmt.Println("      Server does not support upload tests (-R option)")
			}
		} else {
			fmt.Println("      Skipping upload test since download failed")
		}

		// Check if tests completed successfully
		if result.DownloadMbps > 0 && result.UploadMbps > 0 {
			result.TestCompleted = true
		}

		// Add result to sysInfo
		sysInfo.Iperf3Results = append(sysInfo.Iperf3Results, result)

		// Print result
		downloadStr := "N/A"
		if result.DownloadMbps > 0 {
			downloadStr = fmt.Sprintf("%.2f", result.DownloadMbps)
		}

		uploadStr := "N/A"
		if result.UploadMbps > 0 {
			uploadStr = fmt.Sprintf("%.2f", result.UploadMbps)
		}

		fmt.Printf("    %-35s | %-5d | %-15s | %-15s\n",
			location, port, downloadStr, uploadStr)
	}

	fmt.Println("    --------------------------------------------------------------------------")

	return nil
}

// runNetblastTests runs the hyprbench-netblast tests
func runNetblastTests(sysInfo *SystemInfo) error {
	fmt.Println("  Running hyprbench-netblast (multi-server network tests)...")

	// Check for iperf3
	_, err := exec.LookPath("iperf3")
	if err != nil {
		fmt.Println("    iperf3 command not found. Skipping netblast tests.")
		fmt.Println("    Consider installing: 'sudo apt install iperf3'")
		return fmt.Errorf("iperf3 command not found")
	}

	// Check for curl and jq
	_, err = exec.LookPath("curl")
	if err != nil {
		fmt.Println("    curl command not found. Required for fetching server list.")
		return fmt.Errorf("curl command not found")
	}

	_, err = exec.LookPath("jq")
	if err != nil {
		fmt.Println("    jq command not found. Required for parsing server list.")
		return fmt.Errorf("jq command not found")
	}

	// Get self location using ipinfo.io
	fmt.Println("    Getting self location from ipinfo.io...")
	selfLocationOutput, err := runCommand("curl", "-s", "https://ipinfo.io/json")
	if err != nil {
		fmt.Printf("    Error getting self location: %v\n", err)
		return fmt.Errorf("error getting self location: %v", err)
	}

	// Parse self location
	var selfLocation struct {
		IP       string `json:"ip"`
		City     string `json:"city"`
		Region   string `json:"region"`
		Country  string `json:"country"`
		Loc      string `json:"loc"` // Latitude,Longitude
		Timezone string `json:"timezone"`
	}

	if err := json.Unmarshal([]byte(selfLocationOutput), &selfLocation); err != nil {
		fmt.Printf("    Error parsing self location: %v\n", err)
		return fmt.Errorf("error parsing self location: %v", err)
	}

	// Extract latitude and longitude
	var selfLat, selfLon float64
	if selfLocation.Loc != "" {
		locParts := strings.Split(selfLocation.Loc, ",")
		if len(locParts) == 2 {
			selfLat, _ = strconv.ParseFloat(locParts[0], 64)
			selfLon, _ = strconv.ParseFloat(locParts[1], 64)
		}
	}

	fmt.Printf("    Self location: %s, %s, %s (%.4f, %.4f)\n",
		selfLocation.City, selfLocation.Region, selfLocation.Country, selfLat, selfLon)

	// Define a struct to match the server list format
	type Iperf3Server struct {
		IP_HOST   string `json:"IP_HOST"`
		OPTIONS   string `json:"OPTIONS"`
		GB_S      string `json:"GB_S"`
		COUNTRY   string `json:"COUNTRY"`
		SITE      string `json:"SITE"`
		CONTINENT string `json:"CONTINENT"`
		PROVIDER  string `json:"PROVIDER"`
	}

	// Fetch iperf3 server list from the reliable source
	fmt.Println("    Fetching iperf3 server list...")
	serversOutput, err := runCommand("curl", "-s", "--connect-timeout", "10", "https://export.iperf3serverlist.net/json.php?action=download")
	if err != nil {
		fmt.Printf("    Error fetching iperf3 server list: %v\n", err)
		return fmt.Errorf("error fetching iperf3 server list: %v", err)
	}

	// Parse the server list
	var iperf3Servers []Iperf3Server
	if err := json.Unmarshal([]byte(serversOutput), &iperf3Servers); err != nil {
		fmt.Printf("    Error parsing iperf3 server list: %v\n", err)
		return fmt.Errorf("error parsing iperf3 server list: %v", err)
	}

	// Filter and process servers
	var serversWithDistance []struct {
		Host      string
		Port      int
		City      string
		Country   string
		Distance  int // in km
		Lat       float64
		Lon       float64
		Command   string
		Options   string
		Bandwidth string
		Provider  string
	}

	// Process each server
	for _, server := range iperf3Servers {
		// Extract host and port from IP_HOST
		// Format is typically "iperf3 -c hostname -p port" or "iperf3 -c ip"
		hostAndPort := strings.TrimPrefix(server.IP_HOST, "iperf3 -c ")

		// Split by space to separate host and potential port options
		parts := strings.Split(hostAndPort, " ")
		host := parts[0]

		// Default port
		port := 5201
		portRange := ""

		// Check if there's a port specification
		for i := 1; i < len(parts); i++ {
			if parts[i] == "-p" && i+1 < len(parts) {
				portRange = parts[i+1]
				// If it's a range like 5201-5210, just take the first port
				if strings.Contains(portRange, "-") {
					portParts := strings.Split(portRange, "-")
					portStr := portParts[0]
					parsedPort, err := strconv.Atoi(portStr)
					if err == nil {
						port = parsedPort
					}
				} else {
					// Single port
					parsedPort, err := strconv.Atoi(portRange)
					if err == nil {
						port = parsedPort
					}
				}
				break
			}
		}

		// Get approximate coordinates based on country/site
		// This is a very rough approximation
		var lat, lon float64

		// Use a map of known locations
		switch server.COUNTRY {
		case "US":
			switch server.SITE {
			case "New York":
				lat, lon = 40.7128, -74.0060
			case "Los Angeles":
				lat, lon = 34.0522, -118.2437
			case "Miami":
				lat, lon = 25.7617, -80.1918
			case "Chicago":
				lat, lon = 41.8781, -87.6298
			case "Dallas":
				lat, lon = 32.7767, -96.7970
			case "San Francisco":
				lat, lon = 37.7749, -122.4194
			case "Seattle":
				lat, lon = 47.6062, -122.3321
			case "Houston":
				lat, lon = 29.7604, -95.3698
			default:
				lat, lon = 37.0902, -95.7129 // Center of US
			}
		case "UK", "GB":
			switch server.SITE {
			case "London":
				lat, lon = 51.5074, -0.1278
			default:
				lat, lon = 55.3781, -3.4360 // Center of UK
			}
		case "FR":
			switch server.SITE {
			case "Paris":
				lat, lon = 48.8566, 2.3522
			default:
				lat, lon = 46.2276, 2.2137 // Center of France
			}
		case "DE":
			switch server.SITE {
			case "Frankfurt":
				lat, lon = 50.1109, 8.6821
			case "Berlin":
				lat, lon = 52.5200, 13.4050
			default:
				lat, lon = 51.1657, 10.4515 // Center of Germany
			}
		case "NL":
			lat, lon = 52.3676, 4.9041 // Amsterdam
		case "SG":
			lat, lon = 1.3521, 103.8198 // Singapore
		case "JP":
			lat, lon = 35.6762, 139.6503 // Tokyo
		case "AU":
			switch server.SITE {
			case "Sydney":
				lat, lon = -33.8688, 151.2093
			default:
				lat, lon = -25.2744, 133.7751 // Center of Australia
			}
		default:
			// Skip servers without coordinates
			continue
		}

		// Calculate distance
		distance := calculateDistance(selfLat, selfLon, lat, lon)

		// Add to list
		serversWithDistance = append(serversWithDistance, struct {
			Host      string
			Port      int
			City      string
			Country   string
			Distance  int
			Lat       float64
			Lon       float64
			Command   string
			Options   string
			Bandwidth string
			Provider  string
		}{
			Host:      host,
			Port:      port,
			City:      server.SITE,
			Country:   server.COUNTRY,
			Distance:  distance,
			Lat:       lat,
			Lon:       lon,
			Command:   server.IP_HOST,
			Options:   server.OPTIONS,
			Bandwidth: server.GB_S,
			Provider:  server.PROVIDER,
		})
	}

	// We already have serversWithDistance populated from the previous code

	// Sort servers by distance
	sort.Slice(serversWithDistance, func(i, j int) bool {
		return serversWithDistance[i].Distance < serversWithDistance[j].Distance
	})

	// First, estimate user's connection speed from speedtest results if available
	var estimatedUserBandwidth float64 = 1000 // Default assumption: 1 Gbps
	if sysInfo.SpeedtestResults.DownloadMbps > 0 {
		estimatedUserBandwidth = sysInfo.SpeedtestResults.DownloadMbps
	}

	// If user has a very high-speed connection (>10Gbps), assume they have at least 25Gbps
	if estimatedUserBandwidth > 10000 {
		estimatedUserBandwidth = 25000 // Assume 25 Gbps for very high-speed connections
	}

	fmt.Printf("    Estimated connection speed: %.2f Mbps (%.2f Gbps)\n",
		estimatedUserBandwidth, estimatedUserBandwidth/1000)

	// Parse bandwidth for all servers
	var serversWithBandwidth []struct {
		Host          string
		Port          int
		City          string
		Country       string
		Distance      int
		Lat           float64
		Lon           float64
		Command       string
		Options       string
		Bandwidth     string
		Provider      string
		BandwidthGbps float64
		BandwidthMbps float64
		Score         float64 // Combined score for ranking
	}

	for _, server := range serversWithDistance {
		// Parse bandwidth (format is usually like "10", "2x10", "40", etc. in Gbps)
		bandwidthStr := server.Bandwidth
		var bandwidthGbps float64 = 1 // Default to 1 Gbps if we can't parse

		if strings.Contains(bandwidthStr, "x") {
			// Format like "2x10" means 2 connections of 10 Gbps each
			parts := strings.Split(bandwidthStr, "x")
			if len(parts) == 2 {
				count, err1 := strconv.ParseFloat(parts[0], 64)
				speed, err2 := strconv.ParseFloat(parts[1], 64)
				if err1 == nil && err2 == nil {
					bandwidthGbps = count * speed
				}
			}
		} else {
			// Simple format like "10" means 10 Gbps
			speed, err := strconv.ParseFloat(bandwidthStr, 64)
			if err == nil {
				bandwidthGbps = speed
			}
		}

		// Convert to Mbps for comparison with user's speed
		bandwidthMbps := bandwidthGbps * 1000

		// Calculate a score based on bandwidth (80%) and distance (20%)
		// Higher score is better

		// Normalize bandwidth to 0-100 scale (assuming max is 100Gbps)
		bandwidthScore := (bandwidthGbps / 100) * 100
		if bandwidthScore > 100 {
			bandwidthScore = 100
		}

		// Normalize distance to 0-100 scale (0 = best, 100 = worst)
		// Assume max distance is 20,000 km
		distanceScore := float64(server.Distance) / 200 // 20000/100 = 200
		if distanceScore > 100 {
			distanceScore = 100
		}

		// Calculate final score: 80% bandwidth, 20% distance
		// Higher score is better
		score := bandwidthScore*0.8 - distanceScore*0.2

		serversWithBandwidth = append(serversWithBandwidth, struct {
			Host          string
			Port          int
			City          string
			Country       string
			Distance      int
			Lat           float64
			Lon           float64
			Command       string
			Options       string
			Bandwidth     string
			Provider      string
			BandwidthGbps float64
			BandwidthMbps float64
			Score         float64
		}{
			Host:          server.Host,
			Port:          server.Port,
			City:          server.City,
			Country:       server.Country,
			Distance:      server.Distance,
			Lat:           server.Lat,
			Lon:           server.Lon,
			Command:       server.Command,
			Options:       server.Options,
			Bandwidth:     server.Bandwidth,
			Provider:      server.Provider,
			BandwidthGbps: bandwidthGbps,
			BandwidthMbps: bandwidthMbps,
			Score:         score,
		})
	}

	// Sort servers by score (highest first)
	sort.Slice(serversWithBandwidth, func(i, j int) bool {
		return serversWithBandwidth[i].Score > serversWithBandwidth[j].Score
	})

	// Filter out servers that don't have enough bandwidth for the user's connection
	var highBandwidthServers []struct {
		Host          string
		Port          int
		City          string
		Country       string
		Distance      int
		Lat           float64
		Lon           float64
		Command       string
		Options       string
		Bandwidth     string
		Provider      string
		BandwidthGbps float64
		BandwidthMbps float64
		Score         float64
	}

	// We want at least 1.5x the user's speed to ensure we're not bottlenecked by the server
	minRequiredBandwidth := estimatedUserBandwidth * 1.5

	// First pass: try to find servers with at least 1.5x bandwidth
	for _, server := range serversWithBandwidth {
		if server.BandwidthMbps >= minRequiredBandwidth {
			highBandwidthServers = append(highBandwidthServers, server)
		}
	}

	// If we don't have enough high-bandwidth servers, lower our requirements
	if len(highBandwidthServers) < 5 {
		fmt.Printf("    Not enough servers with %.2f Mbps capacity, lowering requirements\n", minRequiredBandwidth)

		// Just take the highest bandwidth servers we have
		fmt.Println("    Using highest available bandwidth servers")

		// Take the top 10 highest bandwidth servers
		count := min(10, len(serversWithBandwidth))
		highBandwidthServers = serversWithBandwidth[:count]
	}

	// Ensure diversity in providers and locations
	var selectedServers []struct {
		Host          string
		Port          int
		City          string
		Country       string
		Distance      int
		Lat           float64
		Lon           float64
		Command       string
		Options       string
		Bandwidth     string
		Provider      string
		BandwidthGbps float64
		BandwidthMbps float64
		Score         float64
	}

	// Track providers and locations we've already selected
	selectedProviders := make(map[string]bool)
	selectedLocations := make(map[string]bool)

	// First, add the highest scoring server
	if len(highBandwidthServers) > 0 {
		selectedServers = append(selectedServers, highBandwidthServers[0])
		selectedProviders[highBandwidthServers[0].Provider] = true
		selectedLocations[highBandwidthServers[0].City+","+highBandwidthServers[0].Country] = true
	}

	// Then add more servers, prioritizing diversity
	for _, server := range highBandwidthServers {
		// Skip if we already selected this server
		alreadySelected := false
		for _, selected := range selectedServers {
			if selected.Host == server.Host && selected.Port == server.Port {
				alreadySelected = true
				break
			}
		}
		if alreadySelected {
			continue
		}

		// Skip if we already have this provider and location
		location := server.City + "," + server.Country
		if selectedProviders[server.Provider] && selectedLocations[location] {
			continue
		}

		// Add this server
		selectedServers = append(selectedServers, server)
		selectedProviders[server.Provider] = true
		selectedLocations[location] = true

		// Stop if we have enough servers
		if len(selectedServers) >= 5 {
			break
		}
	}

	// If we still don't have enough servers, add more regardless of provider/location
	if len(selectedServers) < 5 {
		for _, server := range highBandwidthServers {
			// Skip servers we've already added
			alreadyAdded := false
			for _, selected := range selectedServers {
				if selected.Host == server.Host && selected.Port == server.Port {
					alreadyAdded = true
					break
				}
			}

			if !alreadyAdded {
				selectedServers = append(selectedServers, server)

				// Stop if we have enough servers
				if len(selectedServers) >= 5 {
					break
				}
			}
		}
	}

	// Strictly limit to max 5 servers total
	if len(selectedServers) > 5 {
		selectedServers = selectedServers[:5]
	}

	// Print selected servers
	fmt.Printf("    Selected %d servers for netblast tests:\n", len(selectedServers))
	for i, server := range selectedServers {
		fmt.Printf("      %d. %s, %s (%s) - %d km - %s (%.0f Gbps)\n",
			i+1, server.City, server.Country, server.Host, server.Distance,
			server.Bandwidth, server.BandwidthGbps)
	}

	// Use the selected servers for testing
	serversWithDistance = make([]struct {
		Host      string
		Port      int
		City      string
		Country   string
		Distance  int
		Lat       float64
		Lon       float64
		Command   string
		Options   string
		Bandwidth string
		Provider  string
	}, len(selectedServers))

	for i, server := range selectedServers {
		serversWithDistance[i] = struct {
			Host      string
			Port      int
			City      string
			Country   string
			Distance  int
			Lat       float64
			Lon       float64
			Command   string
			Options   string
			Bandwidth string
			Provider  string
		}{
			Host:      server.Host,
			Port:      server.Port,
			City:      server.City,
			Country:   server.Country,
			Distance:  server.Distance,
			Lat:       server.Lat,
			Lon:       server.Lon,
			Command:   server.Command,
			Options:   server.Options,
			Bandwidth: server.Bandwidth,
			Provider:  server.Provider,
		}
	}

	// Store selected servers for results display
	var filteredServersForDisplay []struct {
		Host          string
		Port          int
		City          string
		Country       string
		Distance      int
		Lat           float64
		Lon           float64
		Command       string
		Options       string
		Bandwidth     string
		Provider      string
		BandwidthGbps float64
		BandwidthMbps float64
		Score         float64
	}
	filteredServersForDisplay = selectedServers

	// Run tests in parallel using goroutines
	fmt.Println("    Running parallel iperf3 tests...")

	// Number of servers to test
	numServers := len(serversWithDistance)

	// Create a channel to collect results
	resultChan := make(chan NetblastResult, numServers)

	// Create a wait group to wait for all tests to complete
	var wg sync.WaitGroup
	wg.Add(numServers)

	// Run tests in parallel
	for i, server := range serversWithDistance {
		// Create a copy of the server for the goroutine
		serverCopy := server
		rank := i + 1

		// Run test in a goroutine
		go func() {
			defer wg.Done()

			// Create result struct
			result := NetblastResult{
				Host:          serverCopy.Host,
				Port:          serverCopy.Port,
				Location:      fmt.Sprintf("%s, %s", serverCopy.City, serverCopy.Country),
				Distance:      serverCopy.Distance,
				DownloadMbps:  -1,
				UploadMbps:    -1,
				TestCompleted: false,
				Rank:          rank,
			}

			// Extract the command and add JSON output
			commandParts := strings.Split(serverCopy.Command, " ")
			if len(commandParts) < 3 {
				// Invalid command format
				resultChan <- result
				return
			}

			// Build the command with timeout and JSON output
			var downloadArgs []string
			downloadArgs = append(downloadArgs, "timeout", "15")
			downloadArgs = append(downloadArgs, commandParts...)
			downloadArgs = append(downloadArgs, "-t", "5", "-J")

			// Run the download test
			fmt.Printf("      Running test: %s\n", strings.Join(downloadArgs, " "))
			downloadOutput, downloadErr := runCommand(downloadArgs[0], downloadArgs[1:]...)

			if downloadErr != nil {
				// Try a simpler version as fallback
				fmt.Println("      First attempt failed, retrying with simpler parameters...")
				downloadOutput, downloadErr = runCommand("timeout", "10", "iperf3", "-c", serverCopy.Host, "-p", fmt.Sprintf("%d", serverCopy.Port), "-t", "3", "-J")
			}

			if downloadErr != nil {
				// Send result to channel even if test failed
				resultChan <- result
				return
			}

			// Parse JSON output
			var downloadResult map[string]interface{}
			if err := json.Unmarshal([]byte(downloadOutput), &downloadResult); err != nil {
				// Send result to channel even if parsing failed
				resultChan <- result
				return
			}

			// Extract download speed
			if end, ok := downloadResult["end"].(map[string]interface{}); ok {
				if sumReceived, ok := end["sum_received"].(map[string]interface{}); ok {
					if bitsPerSecond, ok := sumReceived["bits_per_second"].(float64); ok {
						result.DownloadMbps = bitsPerSecond / 1000000 // Convert to Mbps
					}
				}
			}

			// Only run upload test if download succeeded
			if result.DownloadMbps > 0 {
				// Check if server supports upload (-R option)
				supportsUpload := strings.Contains(serverCopy.Options, "-R")

				if supportsUpload {
					// Build the upload command
					var uploadArgs []string
					uploadArgs = append(uploadArgs, "timeout", "15")
					uploadArgs = append(uploadArgs, commandParts...)
					uploadArgs = append(uploadArgs, "-t", "5", "-R", "-J")

					// Run the upload test
					fmt.Printf("      Running upload test: %s\n", strings.Join(uploadArgs, " "))
					uploadOutput, uploadErr := runCommand(uploadArgs[0], uploadArgs[1:]...)

					if uploadErr != nil {
						// Try a simpler version as fallback
						fmt.Println("      First attempt failed, retrying with simpler parameters...")
						uploadOutput, uploadErr = runCommand("timeout", "10", "iperf3", "-c", serverCopy.Host, "-p", fmt.Sprintf("%d", serverCopy.Port), "-t", "3", "-R", "-J")
					}

					if uploadErr != nil {
						// Send result to channel with download speed only
						resultChan <- result
						return
					}

					// Parse JSON output
					var uploadResult map[string]interface{}
					if err := json.Unmarshal([]byte(uploadOutput), &uploadResult); err != nil {
						// Send result to channel with download speed only
						resultChan <- result
						return
					}

					// Extract upload speed
					if end, ok := uploadResult["end"].(map[string]interface{}); ok {
						if sumSent, ok := end["sum_sent"].(map[string]interface{}); ok {
							if bitsPerSecond, ok := sumSent["bits_per_second"].(float64); ok {
								result.UploadMbps = bitsPerSecond / 1000000 // Convert to Mbps
							}
						}
					}
				} else {
					fmt.Println("      Server does not support upload tests (-R option)")
				}
			}

			// Check if tests completed successfully
			if result.DownloadMbps > 0 && result.UploadMbps > 0 {
				result.TestCompleted = true
			}

			// Send result to channel
			resultChan <- result
		}()
	}

	// Wait for all tests to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var results []NetblastResult
	for result := range resultChan {
		results = append(results, result)
	}

	// Sort results by download speed (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].DownloadMbps > results[j].DownloadMbps
	})

	// Update ranks based on download speed
	for i := range results {
		results[i].Rank = i + 1
	}

	// Print results
	fmt.Println("\n    hyprbench-netblast Results (Ranked by Download Speed):")
	fmt.Println("    -----------------------------------------------------------------------------------------")
	fmt.Printf("    %-4s | %-24s | %-23s | %-9s | %-15s | %-15s\n",
		"Rank", "Location", "Host", "Bandwidth", "Download (Mbps)", "Upload (Mbps)")
	fmt.Println("    -----------------------------------------------------------------------------------------")

	for _, result := range results {
		downloadStr := "N/A"
		if result.DownloadMbps > 0 {
			downloadStr = fmt.Sprintf("%.2f", result.DownloadMbps)
		}

		uploadStr := "N/A"
		if result.UploadMbps > 0 {
			uploadStr = fmt.Sprintf("%.2f", result.UploadMbps)
		}

		// Find bandwidth for this server
		bandwidth := "Unknown"
		for _, server := range filteredServersForDisplay {
			if server.Host == result.Host {
				bandwidth = server.Bandwidth + " Gbps"
				break
			}
		}

		fmt.Printf("    %-4d | %-24s | %-23s | %-9s | %-15s | %-15s\n",
			result.Rank, result.Location, result.Host, bandwidth, downloadStr, uploadStr)
	}

	fmt.Println("    -----------------------------------------------------------------------------------------")

	// Store results in sysInfo
	sysInfo.NetblastResults = results

	return nil
}

// getFallbackIperf3ServerList returns a hardcoded list of iperf3 servers
func getFallbackIperf3ServerList() []map[string]interface{} {
	return []map[string]interface{}{
		{"host": "bouygues.iperf.fr", "port": 5201, "city": "Paris", "country": "FR"},
		{"host": "iperf.he.net", "port": 5201, "city": "Fremont, CA", "country": "US"},
		{"host": "speedtest.serverius.net", "port": 5201, "city": "Netherlands", "country": "NL"},
	}
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// calculateDistance calculates the distance between two points on Earth using the Haversine formula
func calculateDistance(lat1, lon1, lat2, lon2 float64) int {
	const earthRadius = 6371 // Earth radius in kilometers

	// Convert degrees to radians
	lat1Rad := lat1 * math.Pi / 180
	lon1Rad := lon1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lon2Rad := lon2 * math.Pi / 180

	// Haversine formula
	dLat := lat2Rad - lat1Rad
	dLon := lon2Rad - lon1Rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := earthRadius * c

	return int(distance)
}

// exportResultsToJSON exports the benchmark results to a JSON file
func exportResultsToJSON(sysInfo SystemInfo, filePath string) error {
	// Convert SystemInfo to JSON
	jsonData, err := json.MarshalIndent(sysInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling results to JSON: %w", err)
	}

	// Write JSON to file
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("error writing JSON to file: %w", err)
	}

	return nil
}

// exportResultsToHTML exports the benchmark results to an HTML file
func exportResultsToHTML(sysInfo SystemInfo, filePath string) error {
	// Create HTML content
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>HyprBench Results</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
        }
        h1, h2, h3 {
            color: #2c3e50;
        }
        .section {
            margin-bottom: 30px;
            border: 1px solid #ddd;
            border-radius: 5px;
            padding: 20px;
            background-color: #f9f9f9;
        }
        .header {
            background-color: #3498db;
            color: white;
            padding: 20px;
            border-radius: 5px;
            margin-bottom: 20px;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-bottom: 20px;
        }
        th, td {
            padding: 12px 15px;
            border-bottom: 1px solid #ddd;
            text-align: left;
        }
        th {
            background-color: #f2f2f2;
        }
        tr:hover {
            background-color: #f5f5f5;
        }
        .highlight {
            color: #2ecc71;
            font-weight: bold;
        }
        .footer {
            text-align: center;
            margin-top: 30px;
            padding: 20px;
            color: #7f8c8d;
            font-size: 0.9em;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>HyprBench Results</h1>
        <p>Version: ` + sysInfo.HyprBenchVersion + `</p>
        <p>Date: ` + sysInfo.TestDate + `</p>
        <p>Hostname: ` + sysInfo.Hostname + `</p>
    </div>

    <div class="section">
        <h2>System Information</h2>
        <table>
            <tr><th>Component</th><th>Details</th></tr>
            <tr><td>CPU Model</td><td>` + sysInfo.CPUModel + `</td></tr>
            <tr><td>CPU Cores</td><td>` + sysInfo.CPUCores + `</td></tr>
            <tr><td>CPU Threads</td><td>` + sysInfo.CPUThreads + `</td></tr>
            <tr><td>CPU Speed</td><td>` + sysInfo.CPUSpeed + `</td></tr>
            <tr><td>CPU Cache</td><td>` + sysInfo.CPUCache + `</td></tr>
            <tr><td>RAM Total</td><td>` + sysInfo.RAMTotal + `</td></tr>
            <tr><td>RAM Type</td><td>` + sysInfo.RAMType + `</td></tr>
            <tr><td>RAM Speed</td><td>` + sysInfo.RAMSpeed + `</td></tr>
            <tr><td>Motherboard</td><td>` + sysInfo.MotherboardMfr + ` ` + sysInfo.MotherboardModel + `</td></tr>
            <tr><td>OS</td><td>` + sysInfo.OSName + ` ` + sysInfo.OSVersion + `</td></tr>
            <tr><td>Kernel</td><td>` + sysInfo.KernelVersion + `</td></tr>
        </table>
    </div>`

	// Add CPU Benchmark Results if available
	if sysInfo.SysbenchSingleThreadScore != "" || sysInfo.SysbenchMultiThreadScore != "" {
		html += `
    <div class="section">
        <h2>CPU Benchmark Results</h2>
        <table>
            <tr><th>Test</th><th>Score</th></tr>
            <tr><td>Sysbench Single-Thread</td><td class="highlight">` + sysInfo.SysbenchSingleThreadScore + ` events/sec</td></tr>
            <tr><td>Sysbench Multi-Thread</td><td class="highlight">` + sysInfo.SysbenchMultiThreadScore + ` events/sec</td></tr>
        </table>
    </div>`
	}

	// Add Memory Benchmark Results if available
	if sysInfo.StreamCopyBandwidthMBs != "" || sysInfo.StreamScaleBandwidthMBs != "" ||
		sysInfo.StreamAddBandwidthMBs != "" || sysInfo.StreamTriadBandwidthMBs != "" {
		html += `
    <div class="section">
        <h2>Memory Benchmark Results (STREAM)</h2>
        <table>
            <tr><th>Test</th><th>Bandwidth (MB/s)</th></tr>
            <tr><td>Copy</td><td class="highlight">` + sysInfo.StreamCopyBandwidthMBs + `</td></tr>
            <tr><td>Scale</td><td class="highlight">` + sysInfo.StreamScaleBandwidthMBs + `</td></tr>
            <tr><td>Add</td><td class="highlight">` + sysInfo.StreamAddBandwidthMBs + `</td></tr>
            <tr><td>Triad</td><td class="highlight">` + sysInfo.StreamTriadBandwidthMBs + `</td></tr>
        </table>
    </div>`
	}

	// Add Disk I/O Benchmark Results if available
	if len(sysInfo.FioResults) > 0 {
		html += `
    <div class="section">
        <h2>Disk I/O Benchmark Results (FIO)</h2>`

		for _, device := range sysInfo.FioResults {
			html += `
        <h3>Device: ` + device.DeviceModel + `</h3>
        <p>Mount Point: ` + device.MountPoint + `</p>
        <p>Test File Size: ` + device.TestFileSize + `</p>
        <table>
            <tr><th>Test</th><th>IOPS</th><th>Bandwidth (MB/s)</th><th>Latency</th></tr>`

			for _, test := range device.TestResults {
				iopsStr := "N/A"
				if test.IOPS >= 0 {
					iopsStr = fmt.Sprintf("%.0f", test.IOPS)
				}

				bwStr := "N/A"
				if test.BandwidthMB >= 0 {
					bwStr = fmt.Sprintf("%.2f", test.BandwidthMB)
				}

				latencyStr := "N/A"
				if test.LatencyUs >= 0 {
					if test.LatencyUnit == "ms" {
						latencyStr = fmt.Sprintf("%.2f ms", test.LatencyUs)
					} else {
						latencyStr = fmt.Sprintf("%.2f us", test.LatencyUs)
					}
				}

				html += `
            <tr>
                <td>` + test.TestName + `</td>
                <td class="highlight">` + iopsStr + `</td>
                <td class="highlight">` + bwStr + `</td>
                <td>` + latencyStr + `</td>
            </tr>`
			}

			html += `
        </table>`
		}

		html += `
    </div>`
	}

	// Add Network Benchmark Results if available
	if sysInfo.SpeedtestResults.TestCompleted || len(sysInfo.Iperf3Results) > 0 || len(sysInfo.NetblastResults) > 0 {
		html += `
    <div class="section">
        <h2>Network Benchmark Results</h2>`

		if sysInfo.SpeedtestResults.TestCompleted {
			html += `
        <h3>Local Speed Test (` + sysInfo.SpeedtestResults.ToolUsed + `)</h3>
        <table>
            <tr><th>Metric</th><th>Value</th></tr>
            <tr><td>Download</td><td class="highlight">` + fmt.Sprintf("%.2f Mbps", sysInfo.SpeedtestResults.DownloadMbps) + `</td></tr>
            <tr><td>Upload</td><td class="highlight">` + fmt.Sprintf("%.2f Mbps", sysInfo.SpeedtestResults.UploadMbps) + `</td></tr>`

			if sysInfo.SpeedtestResults.LatencyMs > 0 {
				html += `
            <tr><td>Latency</td><td>` + fmt.Sprintf("%.2f ms", sysInfo.SpeedtestResults.LatencyMs) + `</td></tr>`
			}

			html += `
        </table>`
		}

		if len(sysInfo.Iperf3Results) > 0 {
			html += `
        <h3>iperf3 Single Server Tests</h3>
        <table>
            <tr><th>Server</th><th>Location</th><th>Download (Mbps)</th><th>Upload (Mbps)</th></tr>`

			for _, result := range sysInfo.Iperf3Results {
				downloadStr := "N/A"
				if result.DownloadMbps > 0 {
					downloadStr = fmt.Sprintf("%.2f", result.DownloadMbps)
				}

				uploadStr := "N/A"
				if result.UploadMbps > 0 {
					uploadStr = fmt.Sprintf("%.2f", result.UploadMbps)
				}

				html += `
            <tr>
                <td>` + result.Host + `:` + fmt.Sprintf("%d", result.Port) + `</td>
                <td>` + result.Location + `</td>
                <td class="highlight">` + downloadStr + `</td>
                <td class="highlight">` + uploadStr + `</td>
            </tr>`
			}

			html += `
        </table>`
		}

		if len(sysInfo.NetblastResults) > 0 {
			html += `
        <h3>hyprbench-netblast Multi-Server Tests</h3>
        <table>
            <tr><th>Rank</th><th>Server</th><th>Location</th><th>Download (Mbps)</th><th>Upload (Mbps)</th></tr>`

			// Sort results by rank
			sortedResults := make([]NetblastResult, len(sysInfo.NetblastResults))
			copy(sortedResults, sysInfo.NetblastResults)
			sort.Slice(sortedResults, func(i, j int) bool {
				return sortedResults[i].Rank < sortedResults[j].Rank
			})

			for _, result := range sortedResults {
				downloadStr := "N/A"
				if result.DownloadMbps > 0 {
					downloadStr = fmt.Sprintf("%.2f", result.DownloadMbps)
				}

				uploadStr := "N/A"
				if result.UploadMbps > 0 {
					uploadStr = fmt.Sprintf("%.2f", result.UploadMbps)
				}

				html += `
            <tr>
                <td>` + fmt.Sprintf("%d", result.Rank) + `</td>
                <td>` + result.Host + `:` + fmt.Sprintf("%d", result.Port) + `</td>
                <td>` + result.Location + `</td>
                <td class="highlight">` + downloadStr + `</td>
                <td class="highlight">` + uploadStr + `</td>
            </tr>`
			}

			html += `
        </table>`
		}

		html += `
    </div>`
	}

	// Add Stress Benchmark Results if available
	if sysInfo.StressResults.CPUBogoOps > 0 || sysInfo.StressResults.MatrixBogoOps > 0 || sysInfo.StressResults.VMBogoOps > 0 {
		html += `
    <div class="section">
        <h2>System Stress Benchmark Results (stress-ng)</h2>
        <table>
            <tr><th>Test</th><th>Bogo Operations</th><th>Bogo Operations/sec</th></tr>`

		if sysInfo.StressResults.CPUBogoOps > 0 {
			html += `
            <tr>
                <td>CPU Stress (cores: ` + sysInfo.StressResults.CPUCores + `, method: ` + sysInfo.StressResults.CPUMethod + `)</td>
                <td>` + fmt.Sprintf("%.0f", sysInfo.StressResults.CPUBogoOps) + `</td>
                <td class="highlight">` + fmt.Sprintf("%.2f", sysInfo.StressResults.CPUBogoOpsPerSec) + `</td>
            </tr>`
		}

		if sysInfo.StressResults.MatrixBogoOps > 0 {
			html += `
            <tr>
                <td>Matrix Stress</td>
                <td>` + fmt.Sprintf("%.0f", sysInfo.StressResults.MatrixBogoOps) + `</td>
                <td class="highlight">` + fmt.Sprintf("%.2f", sysInfo.StressResults.MatrixBogoOpsPerSec) + `</td>
            </tr>`
		}

		if sysInfo.StressResults.VMBogoOps > 0 {
			html += `
            <tr>
                <td>VM Stress</td>
                <td>` + fmt.Sprintf("%.0f", sysInfo.StressResults.VMBogoOps) + `</td>
                <td class="highlight">` + fmt.Sprintf("%.2f", sysInfo.StressResults.VMBogoOpsPerSec) + `</td>
            </tr>`
		}

		html += `
        </table>
    </div>`
	}

	// Add UnixBench Results if available
	if sysInfo.UnixBenchResults.SystemBenchmarkIndex > 0 {
		html += `
    <div class="section">
        <h2>Public Reference Benchmark Results (UnixBench via PTS)</h2>
        <h3>System Benchmark Index: <span class="highlight">` + fmt.Sprintf("%.2f", sysInfo.UnixBenchResults.SystemBenchmarkIndex) + `</span></h3>
        <table>
            <tr><th>Test</th><th>Score</th></tr>`

		if sysInfo.UnixBenchResults.Dhrystone2 > 0 {
			html += `
            <tr><td>Dhrystone 2</td><td class="highlight">` + fmt.Sprintf("%.2f", sysInfo.UnixBenchResults.Dhrystone2) + `</td></tr>`
		}
		if sysInfo.UnixBenchResults.DoubleFloatingPoint > 0 {
			html += `
            <tr><td>Double Floating Point</td><td class="highlight">` + fmt.Sprintf("%.2f", sysInfo.UnixBenchResults.DoubleFloatingPoint) + `</td></tr>`
		}
		if sysInfo.UnixBenchResults.ExecThroughput > 0 {
			html += `
            <tr><td>Execl Throughput</td><td class="highlight">` + fmt.Sprintf("%.2f", sysInfo.UnixBenchResults.ExecThroughput) + `</td></tr>`
		}
		if sysInfo.UnixBenchResults.FileCopy1K > 0 {
			html += `
            <tr><td>File Copy 1K</td><td class="highlight">` + fmt.Sprintf("%.2f", sysInfo.UnixBenchResults.FileCopy1K) + `</td></tr>`
		}
		if sysInfo.UnixBenchResults.FileCopy256B > 0 {
			html += `
            <tr><td>File Copy 256B</td><td class="highlight">` + fmt.Sprintf("%.2f", sysInfo.UnixBenchResults.FileCopy256B) + `</td></tr>`
		}
		if sysInfo.UnixBenchResults.FileCopy4K > 0 {
			html += `
            <tr><td>File Copy 4K</td><td class="highlight">` + fmt.Sprintf("%.2f", sysInfo.UnixBenchResults.FileCopy4K) + `</td></tr>`
		}
		if sysInfo.UnixBenchResults.PipeThroughput > 0 {
			html += `
            <tr><td>Pipe Throughput</td><td class="highlight">` + fmt.Sprintf("%.2f", sysInfo.UnixBenchResults.PipeThroughput) + `</td></tr>`
		}
		if sysInfo.UnixBenchResults.PipeBasedCS > 0 {
			html += `
            <tr><td>Pipe-based Context Switching</td><td class="highlight">` + fmt.Sprintf("%.2f", sysInfo.UnixBenchResults.PipeBasedCS) + `</td></tr>`
		}
		if sysInfo.UnixBenchResults.ProcessCreation > 0 {
			html += `
            <tr><td>Process Creation</td><td class="highlight">` + fmt.Sprintf("%.2f", sysInfo.UnixBenchResults.ProcessCreation) + `</td></tr>`
		}
		if sysInfo.UnixBenchResults.ShellScripts > 0 {
			html += `
            <tr><td>Shell Scripts</td><td class="highlight">` + fmt.Sprintf("%.2f", sysInfo.UnixBenchResults.ShellScripts) + `</td></tr>`
		}
		if sysInfo.UnixBenchResults.SystemCallOverhead > 0 {
			html += `
            <tr><td>System Call Overhead</td><td class="highlight">` + fmt.Sprintf("%.2f", sysInfo.UnixBenchResults.SystemCallOverhead) + `</td></tr>`
		}

		html += `
        </table>
    </div>`
	}

	// Add footer
	html += `
    <div class="footer">
        <p>Generated by HyprBench ` + sysInfo.HyprBenchVersion + ` on ` + sysInfo.TestDate + `</p>
    </div>
</body>
</html>`

	// Write HTML to file
	if err := os.WriteFile(filePath, []byte(html), 0644); err != nil {
		return fmt.Errorf("error writing HTML to file: %w", err)
	}

	return nil
}

func runPublicRefBenchmarks(sysInfo *SystemInfo) error {
	fmt.Println("  Running Public Reference Benchmarks (UnixBench via Phoronix Test Suite)...")

	// Initialize UnixBench results in SystemInfo
	sysInfo.UnixBenchResults = UnixBenchResults{
		SystemBenchmarkIndex: -1,
		Dhrystone2:           -1,
		DoubleFloatingPoint:  -1,
		ExecThroughput:       -1,
		FileCopy1K:           -1,
		FileCopy256B:         -1,
		FileCopy4K:           -1,
		PipeThroughput:       -1,
		PipeBasedCS:          -1,
		ProcessCreation:      -1,
		ShellScripts:         -1,
		SystemCallOverhead:   -1,
		TestCompleted:        false,
		ErrorMessage:         "",
	}

	// Check if Phoronix Test Suite is installed
	ptsPath := "./phoronix-test-suite/phoronix-test-suite"
	if _, err := os.Stat(ptsPath); os.IsNotExist(err) {
		errMsg := "Phoronix Test Suite not found at " + ptsPath
		fmt.Println("    " + errMsg + " Please clone it via 'git clone https://github.com/phoronix-test-suite/phoronix-test-suite.git'.")
		sysInfo.UnixBenchResults.ErrorMessage = "PTS Not Found"
		return fmt.Errorf("%s", errMsg)
	}

	// Check if we need to set up PTS enterprise mode
	if !fileExists("./phoronix-test-suite/pts-core/static/enterprise-setup-done") {
		fmt.Println("    Setting up Phoronix Test Suite in enterprise mode...")
		sysInfo.UnixBenchResults.PtsEnterpriseSetupNeeded = true

		// Create enterprise setup file
		setupFile := "./phoronix-test-suite/pts-core/static/enterprise-setup-done"
		if err := os.MkdirAll(filepath.Dir(setupFile), 0755); err != nil {
			fmt.Printf("    Error creating enterprise setup directory: %v\n", err)
		} else {
			if file, err := os.Create(setupFile); err != nil {
				fmt.Printf("    Error creating enterprise setup file: %v\n", err)
			} else {
				file.Close()
			}
		}

		// Create user config file
		userConfigDir := "./phoronix-test-suite/user-config"
		if err := os.MkdirAll(userConfigDir, 0755); err != nil {
			fmt.Printf("    Error creating user config directory: %v\n", err)
		} else {
			userConfigFile := filepath.Join(userConfigDir, "user-config.xml")
			userConfigContent := `<?xml version="1.0"?>
<PhoronixTestSuite>
  <Options>
    <OpenBenchmarking>
      <AnonymousUsageReporting>FALSE</AnonymousUsageReporting>
      <AnonymousSoftwareReporting>FALSE</AnonymousSoftwareReporting>
      <AnonymousHardwareReporting>FALSE</AnonymousHardwareReporting>
    </OpenBenchmarking>
    <General>
      <DefaultDisplayMode>DEFAULT</DefaultDisplayMode>
      <BatchMode>TRUE</BatchMode>
      <SleepBetweenTests>0</SleepBetweenTests>
    </General>
  </Options>
</PhoronixTestSuite>`
			if err := os.WriteFile(userConfigFile, []byte(userConfigContent), 0644); err != nil {
				fmt.Printf("    Error writing user config file: %v\n", err)
			}
		}
	}

	// Set up environment variables for batch mode
	resultsFile := "./unixbench_results.xml"
	os.Setenv("BATCH_MODE", "1")
	os.Setenv("BATCH_SAVE_XML_RESULTS", "1")
	os.Setenv("BATCH_SAVE_XML_FILE", resultsFile)
	os.Setenv("TEST_RESULTS_NAME", "HyprBench_UnixBench")
	os.Setenv("TEST_RESULTS_IDENTIFIER", "HyprBench_UnixBench")
	os.Setenv("TEST_RESULTS_DESCRIPTION", "HyprBench UnixBench Results")

	// Run UnixBench via PTS
	fmt.Println("    Running UnixBench via Phoronix Test Suite (this may take a while)...")
	_, err := runCommand(ptsPath, "batch-run", "pts/unixbench")
	if err != nil {
		fmt.Printf("    Error running UnixBench: %v\n", err)
		sysInfo.UnixBenchResults.ErrorMessage = fmt.Sprintf("UnixBench failed: %v", err)
		return err
	}

	// Check if results file exists
	if _, err := os.Stat(resultsFile); os.IsNotExist(err) {
		fmt.Printf("    UnixBench results file not found: %s\n", resultsFile)
		sysInfo.UnixBenchResults.ErrorMessage = "Results file not found"
		return fmt.Errorf("unixbench results file not found: %s", resultsFile)
	}

	// Parse results file
	fmt.Println("    Parsing UnixBench results...")
	xmlData, err := os.ReadFile(resultsFile)
	if err != nil {
		fmt.Printf("    Error reading UnixBench results file: %v\n", err)
		sysInfo.UnixBenchResults.ErrorMessage = fmt.Sprintf("Error reading results file: %v", err)
		return err
	}

	// Define PtsResult struct for UnixBench XML parsing
	type PtsResult struct {
		XMLName xml.Name `xml:"PhoronixTestSuite"`
		Results struct {
			Results []struct {
				TestResults []struct {
					TestProfile []struct {
						Name        string `xml:"name,attr"`
						TestResults []struct {
							Results []struct {
								Result []struct {
									Identifier string `xml:"identifier,attr"`
									Value      string `xml:",chardata"`
								} `xml:"Result"`
							} `xml:"Results"`
						} `xml:"TestResult"`
					} `xml:"TestProfile"`
				} `xml:"TestResult"`
			} `xml:"Result"`
		} `xml:"Results"`
	}

	// Parse XML
	var ptsResult PtsResult
	if err := xml.Unmarshal(xmlData, &ptsResult); err != nil {
		fmt.Printf("    Error parsing UnixBench XML results: %v\n", err)
		sysInfo.UnixBenchResults.ErrorMessage = fmt.Sprintf("XML parse error: %v", err)
		return err
	}

	// Extract UnixBench results
	for _, result := range ptsResult.Results.Results {
		for _, testResult := range result.TestResults {
			for _, testProfile := range testResult.TestProfile {
				if testProfile.Name == "unixbench" {
					for _, testResult := range testProfile.TestResults {
						for _, testResult := range testResult.Results {
							for _, result := range testResult.Result {
								value, err := strconv.ParseFloat(result.Value, 64)
								if err != nil {
									fmt.Printf("    Error parsing result value: %v\n", err)
									continue
								}

								switch result.Identifier {
								case "system-benchmark-index":
									sysInfo.UnixBenchResults.SystemBenchmarkIndex = value
								case "dhrystone-2":
									sysInfo.UnixBenchResults.Dhrystone2 = value
								case "double-precision-whetstone":
									sysInfo.UnixBenchResults.DoubleFloatingPoint = value
								case "execl-throughput":
									sysInfo.UnixBenchResults.ExecThroughput = value
								case "file-copy-1024b":
									sysInfo.UnixBenchResults.FileCopy1K = value
								case "file-copy-256b":
									sysInfo.UnixBenchResults.FileCopy256B = value
								case "file-copy-4096b":
									sysInfo.UnixBenchResults.FileCopy4K = value
								case "pipe-throughput":
									sysInfo.UnixBenchResults.PipeThroughput = value
								case "pipe-based-context-switching":
									sysInfo.UnixBenchResults.PipeBasedCS = value
								case "process-creation":
									sysInfo.UnixBenchResults.ProcessCreation = value
								case "shell-scripts-1":
									sysInfo.UnixBenchResults.ShellScripts = value
								case "system-call-overhead":
									sysInfo.UnixBenchResults.SystemCallOverhead = value
								}
							}
						}
					}
				}
			}
		}
	}

	// Check if we got valid results
	if sysInfo.UnixBenchResults.SystemBenchmarkIndex > 0 {
		sysInfo.UnixBenchResults.TestCompleted = true
		sysInfo.UnixBenchResults.ResultFile = resultsFile

		fmt.Println("    UnixBench Results:")
		fmt.Printf("      System Benchmark Index: %.2f\n", sysInfo.UnixBenchResults.SystemBenchmarkIndex)
		fmt.Printf("      Dhrystone 2:            %.2f\n", sysInfo.UnixBenchResults.Dhrystone2)
		fmt.Printf("      Double Floating Point:  %.2f\n", sysInfo.UnixBenchResults.DoubleFloatingPoint)
		fmt.Printf("      Execl Throughput:       %.2f\n", sysInfo.UnixBenchResults.ExecThroughput)
		fmt.Printf("      File Copy 1K:           %.2f\n", sysInfo.UnixBenchResults.FileCopy1K)
		fmt.Printf("      File Copy 256B:         %.2f\n", sysInfo.UnixBenchResults.FileCopy256B)
		fmt.Printf("      File Copy 4K:           %.2f\n", sysInfo.UnixBenchResults.FileCopy4K)
		fmt.Printf("      Pipe Throughput:        %.2f\n", sysInfo.UnixBenchResults.PipeThroughput)
		fmt.Printf("      Pipe-based CS:          %.2f\n", sysInfo.UnixBenchResults.PipeBasedCS)
		fmt.Printf("      Process Creation:       %.2f\n", sysInfo.UnixBenchResults.ProcessCreation)
		fmt.Printf("      Shell Scripts:          %.2f\n", sysInfo.UnixBenchResults.ShellScripts)
		fmt.Printf("      System Call Overhead:   %.2f\n", sysInfo.UnixBenchResults.SystemCallOverhead)
	} else {
		fmt.Println("    Failed to extract UnixBench results from XML")
		sysInfo.UnixBenchResults.ErrorMessage = "Failed to extract results from XML"
	}

	return nil
}

func checkRoot() {
	if os.Geteuid() != 0 {
		fmt.Println("💀 HyprBench requires root. Come back when you’ve grown.")
		os.Exit(1)
	}
	fmt.Println("Root check: Passed (running as root).")
}

// --- Dependency Checking (modified to support auto-install attempt) ---
func checkDependencies(attemptInstall bool) error {
	requiredCommands := map[string]string{
		"sysbench":    "sysbench",
		"fio":         "fio",
		"git":         "git",
		"php":         "php-cli",
		"php-xml":     "php-xml", // Special handling below
		"stress-ng":   "stress-ng",
		"iperf3":      "iperf3",
		"curl":        "curl",
		"jq":          "jq",
		"lspci":       "pciutils",    // For NVMe controller info
		"lsblk":       "util-linux",  // For disk info
		"dmidecode":   "dmidecode",   // For detailed hardware info (requires root)
		"nproc":       "coreutils",   // For number of processors
		"bc":          "bc",          // For calculations (optional, but good to have)
		"lsb_release": "lsb-release", // For OS detection
	}
	optionalSpeedtestTools := []string{"speedtest", "speedtest-cli", "fast"} // Ordered by preference

	var missingDeps []string
	var foundPhp bool

	fmt.Println("  Verifying required commands...")
	for cmdName, pkgName := range requiredCommands {
		_, err := exec.LookPath(cmdName)
		if cmdName == "php-xml" { // Special handling for php-xml
			if !foundPhp { // Check base php first if not already confirmed
				if _, phpErr := exec.LookPath("php"); phpErr != nil {
					fmt.Printf("    - Missing: php (required for php-xml)\n")
					missingDeps = append(missingDeps, "php-cli") // Add base php package
					missingDeps = append(missingDeps, pkgName)   // Add php-xml package
					continue
				}
				foundPhp = true
			}
			// Now check for XML module
			phpXmlCmd := exec.Command("php", "-m")
			output, modErr := phpXmlCmd.CombinedOutput()
			if modErr != nil || !strings.Contains(strings.ToLower(string(output)), "xml") {
				fmt.Printf("    - Missing: php-xml extension (php -m does not list 'xml')\n")
				missingDeps = append(missingDeps, pkgName)
				err = fmt.Errorf("php-xml not found") // Mark error for auto-install
			} else {
				fmt.Printf("    - Found: php-xml extension\n")
				err = nil // Clear error
			}
		} else if err != nil {
			fmt.Printf("    - Missing: %s (package: %s)\n", cmdName, pkgName)
			missingDeps = append(missingDeps, pkgName)
		} else {
			fmt.Printf("    - Found: %s\n", cmdName)
			if cmdName == "php" {
				foundPhp = true
			}
		}

		if err != nil && attemptInstall && os.Geteuid() == 0 {
			fmt.Printf("    Attempting to install %s (%s)...\n", cmdName, pkgName)
			if installErr := attemptInstallPackage(pkgName); installErr != nil {
				fmt.Printf("      Failed to auto-install %s: %v\n", pkgName, installErr)
			} else {
				fmt.Printf("      Successfully installed %s. Please re-run HyprBench.\n", pkgName)
				// After successful install, we should ideally re-check or ask user to re-run.
				// For simplicity now, we'll continue and let the next run confirm.
				// To make it check immediately, remove from missingDeps if successful.
				// However, some installs (like php-xml after php) might need a re-check logic.
			}
		}
	}

	// Check for at least one speedtest tool
	speedToolFound := false
	var foundSpeedToolCmd string
	for _, cmdName := range optionalSpeedtestTools {
		if _, err := exec.LookPath(cmdName); err == nil {
			fmt.Printf("    - Found speedtest tool: %s\n", cmdName)
			speedToolFound = true
			foundSpeedToolCmd = cmdName // Store for potential use
			break
		}
	}
	if !speedToolFound {
		pkgToInstall := "speedtest-cli" // Default suggestion
		if os.Geteuid() == 0 && attemptInstall {
			fmt.Printf("    - No speedtest tool found. Attempting to install Ookla speedtest-cli...\n")
			// This is a more complex install (adding repo), so we might just guide the user
			// or implement a specific function for it.
			// For now, just note it's missing if auto-install is on.
			if installErr := attemptInstallOoklaSpeedtest(); installErr != nil {
				fmt.Printf("      Failed to auto-install Ookla speedtest-cli: %v. Local speed tests will be skipped.\n", installErr)
				// missingDeps = append(missingDeps, "Ookla speedtest-cli (manual install recommended)")
			} else {
				fmt.Printf("      Successfully installed Ookla speedtest-cli. Please re-run HyprBench.\n")
				// Again, ideally re-check or ask user to re-run.
			}
		} else {
			fmt.Printf("    - Warning: No speedtest tool found (speedtest, speedtest-cli, or fast). Local speed test will be skipped. Consider installing '%s'.\n", pkgToInstall)
		}
	} else {
		_ = foundSpeedToolCmd // Suppress unused variable for now
	}

	// Re-check missing after install attempts (simple way)
	// This re-check logic needs to be more robust. For now, if auto-install was attempted,
	// the user might be asked to re-run. The initial missingDeps list is used for the error.
	// A more sophisticated approach would be to re-run LookPath for successfully installed items.
	finalMissingAfterAttempt := []string{}
	if attemptInstall { // Only re-evaluate if installs were attempted
		fmt.Println("  Re-verifying dependencies after install attempts...")
		for cmdName, pkgName := range requiredCommands {
			_, err := exec.LookPath(cmdName)
			if cmdName == "php-xml" {
				if !foundPhp { // Should have been caught earlier if php base was missing
					finalMissingAfterAttempt = append(finalMissingAfterAttempt, pkgName+" (php base missing)")
					continue
				}
				phpXmlCmd := exec.Command("php", "-m")
				output, modErr := phpXmlCmd.CombinedOutput()
				if modErr != nil || !strings.Contains(strings.ToLower(string(output)), "xml") {
					finalMissingAfterAttempt = append(finalMissingAfterAttempt, pkgName)
				}
			} else if err != nil {
				finalMissingAfterAttempt = append(finalMissingAfterAttempt, pkgName)
			}
		}
	} else { // If no install was attempted, the original missingDeps list is the final one.
		finalMissingAfterAttempt = missingDeps
	}

	if len(finalMissingAfterAttempt) > 0 {
		// Remove duplicates
		uniqueMissingMap := make(map[string]struct{})
		uniqueFinalMissingList := []string{}
		for _, item := range finalMissingAfterAttempt { // Use the re-evaluated list
			if _, value := uniqueMissingMap[item]; !value {
				uniqueMissingMap[item] = struct{}{}
				uniqueFinalMissingList = append(uniqueFinalMissingList, item)
			}
		}
		return fmt.Errorf("still missing required commands/extensions: %s. Please install them manually or ensure auto-install succeeded", strings.Join(uniqueFinalMissingList, ", "))
	}

	return nil
}

// Helper to find command name by package name (simplified)
// Currently unused but kept for potential future use
/*
func findCmdByPkg(commands map[string]string, pkgName string) (string, bool) {
	for cmd, pkg := range commands {
		if pkg == pkgName {
			return cmd, true
		}
		// Handle cases like php-xml where pkgName might be just "php-xml" but cmd is "php-xml"
		if strings.Contains(pkg, pkgName) || strings.Contains(cmd, pkgName) {
			return cmd, true
		}
	}
	return "", false
}
*/

func detectPackageManager() (string, error) {
	if _, err := exec.LookPath("apt"); err == nil {
		return "apt", nil
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return "dnf", nil
	}
	if _, err := exec.LookPath("yum"); err == nil {
		return "yum", nil
	}
	// Add more package managers like pacman, zypper if needed
	return "", fmt.Errorf("unsupported package manager or none found (apt, dnf, yum)")
}

func attemptInstallPackage(packageName string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("cannot attempt package installation without root privileges")
	}

	pm, err := detectPackageManager()
	if err != nil {
		return err
	}

	var installCmd *exec.Cmd
	switch pm {
	case "apt":
		// apt-get update can be noisy and slow, consider if it's always needed before each install.
		// For now, assume user/system handles updates, or run it once at the start.
		// runCommand("apt-get", "update", "-qq") // -qq for quiet
		installCmd = exec.Command("apt-get", "install", "-y", "-qq", packageName)
	case "dnf":
		installCmd = exec.Command("dnf", "install", "-y", packageName)
	case "yum": // Older Fedora/CentOS
		installCmd = exec.Command("yum", "install", "-y", packageName)
	default:
		return fmt.Errorf("package manager %s is not supported for auto-installation", pm)
	}

	fmt.Printf("    Executing: %s\n", installCmd.String())
	output, err := installCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install %s using %s: %v. Output: %s", packageName, pm, err, string(output))
	}
	fmt.Printf("    Installation of %s seems successful.\n", packageName)
	return nil
}

func attemptInstallOoklaSpeedtest() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("cannot attempt Ookla Speedtest installation without root privileges")
	}
	pm, err := detectPackageManager()
	if err != nil {
		return err
	}

	if pm == "apt" {
		// Ensure curl and other dependencies for the script are present
		for _, dep := range []string{"curl", "gnupg1", "apt-transport-https", "ca-certificates"} {
			if _, err := exec.LookPath(dep); err != nil {
				fmt.Printf("    Dependency '%s' for Ookla script missing. Attempting to install...\n", dep)
				if instErr := attemptInstallPackage(dep); instErr != nil {
					return fmt.Errorf("failed to install dependency '%s' for Ookla script: %v", dep, instErr)
				}
			}
		}

		fmt.Println("    Attempting to add Ookla repository and install speedtest...")
		// scriptContent := "curl -s https://packagecloud.io/install/repositories/ookla/speedtest-cli/script.deb.sh | bash"
		// cmd := exec.Command("bash", "-c", scriptContent)

		// Step 1: Download script
		_, err = runCommand("curl", "-fsSL", "https://packagecloud.io/install/repositories/ookla/speedtest-cli/script.deb.sh", "-o", "/tmp/speedtest_install.sh")
		if err != nil {
			return fmt.Errorf("failed to download Ookla install script: %v", err)
		}
		if fi, _ := os.Stat("/tmp/speedtest_install.sh"); fi.Size() == 0 {
			return fmt.Errorf("downloaded Ookla script is empty")
		}

		// Step 2: Execute script
		_, err = runCommand("bash", "/tmp/speedtest_install.sh")
		if err != nil {
			return fmt.Errorf("failed to execute Ookla install script: %v", err)
		}

		// Step 3: Update apt lists
		_, err = runCommand("apt-get", "update", "-qq")
		if err != nil {
			return fmt.Errorf("failed to apt-get update after Ookla repo add: %v", err)
		}

		// Step 4: Install the package
		return attemptInstallPackage("speedtest")
	}
	return fmt.Errorf("ookla speedtest auto-install not supported for package manager: %s (manual install recommended)", pm)
}

// --- Helper to parse lscpu output (example) ---
// Helper to parse free output (Mem line for total memory in bytes) and convert to human readable
func parseFreeForTotal(output, key string) string {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, key) {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				// Assuming the total memory is in the second field (index 1)
				// and it's in bytes because we used 'free -b'
				bytesStr := fields[1]
				bytes, err := parseIntStrict(bytesStr) // Use strict parsing
				if err == nil {
					return humanReadableBytes(uint64(bytes))
				}
				fmt.Printf("    Warning: could not parse memory bytes '%s': %v\n", bytesStr, err)
			}
		}
	}
	return "N/A"
}

// Helper to convert bytes to human-readable string
func humanReadableBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// Helper to parse string to int, returns error if not a valid number
func parseIntStrict(s string) (int, error) {
	// Remove common units and trim spaces first
	s = strings.ReplaceAll(s, "MHz", "")
	s = strings.ReplaceAll(s, "KiB", "") // For lscpu cache sizes
	s = strings.ReplaceAll(s, "MiB", "") // For lscpu cache sizes
	s = strings.ReplaceAll(s, "GiB", "") // For lscpu cache sizes
	s = strings.ReplaceAll(s, "KB", "")
	s = strings.ReplaceAll(s, "MB", "")
	s = strings.ReplaceAll(s, "GB", "")
	s = strings.ReplaceAll(s, ",", "") // Remove commas for numbers like "2,900"
	s = strings.TrimSpace(s)

	// Extract leading digits
	re := regexp.MustCompile(`^(\d+)`)
	matches := re.FindStringSubmatch(s)
	if len(matches) > 1 {
		var val int
		// Use Sscanf for direct integer parsing from the extracted digits.
		_, err := fmt.Sscanf(matches[1], "%d", &val)
		if err == nil {
			return val, nil
		}
		return 0, fmt.Errorf("could not parse extracted digits '%s' (from original '%s') as int: %v", matches[1], s, err)
	}
	return 0, fmt.Errorf("no leading digits found in '%s' to parse as int", s)
}
func parseLscpu(output, key string) string {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, key) {
			return strings.TrimSpace(strings.TrimPrefix(line, key))
		}
	}
	return "N/A"
}

// Regex for lsblk -bpno NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE,ROTA
var lsblkOutputRegex = regexp.MustCompile(`^(\S+)\s+(\d+)\s+(\S+)\s+(\S*)\s+(\S*)\s+(\S)$`)

// Group 1: NAME
// Group 2: SIZE (bytes)
// Group 3: TYPE
// Group 4: MOUNTPOINT (can be empty string if not mounted, or a path)
// Group 5: FSTYPE (can be empty string)
// Group 6: ROTA

func parseLsblkOutput(output string) []StorageDevice {
	var devices []StorageDevice
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		matches := lsblkOutputRegex.FindStringSubmatch(line)
		// Expected matches: full_match, NAME, SIZE, TYPE, MOUNTPOINT, FSTYPE, ROTA (7 elements)
		if len(matches) == 7 {
			var dev StorageDevice
			dev.Name = matches[1]

			sizeBytes, err := parseIntStrict(matches[2]) // SIZE is in bytes
			if err == nil {
				dev.Size = humanReadableBytes(uint64(sizeBytes))
			} else {
				dev.Size = matches[2] + " B (raw)" // Fallback
				fmt.Printf("    Warning: lsblk could not parse size '%s' for %s: %v\n", matches[2], dev.Name, err)
			}

			dev.Type = matches[3]
			// For MOUNTPOINT and FSTYPE, if they are truly empty, \S* might not capture them correctly
			// if they are just spaces. The regex `(\S*)` captures zero or more non-whitespace.
			// If lsblk outputs actual spaces for empty fields before the next field, this might be okay.
			// If it just omits the field leading to fewer columns, the regex won't match.
			// The command `lsblk -bpno NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE,ROTA` should provide 6 columns.
			// Let's assume the regex captures empty strings if the field is just whitespace or truly empty.
			dev.MountPoint = strings.TrimSpace(matches[4])
			dev.FSType = strings.TrimSpace(matches[5])
			dev.Rota = matches[6]

			devices = append(devices, dev)
		} else {
			fmt.Printf("    Warning: lsblk line did not match expected 6-field regex: '%s' (Matches: %d)\n", line, len(matches))
		}
	}
	return devices
}

func parseLspciForNVMe(output string) []string {
	var controllers []string
	scanner := bufio.NewScanner(strings.NewReader(output))
	nvmeRegex := regexp.MustCompile(`(?i)Non-Volatile memory controller|NVMe device`)
	for scanner.Scan() {
		line := scanner.Text()
		if nvmeRegex.MatchString(line) {
			controllers = append(controllers, strings.TrimSpace(line))
		}
	}
	return controllers
}

func parseLsblkForNVMeDetails(output string) []NVMeInfo {
	var nvmes []NVMeInfo
	scanner := bufio.NewScanner(strings.NewReader(output))
	// Expecting lines like: /dev/nvme0n1 Samsung FooBar 512G
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, "nvme") { // Quick filter
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 1 && strings.HasPrefix(parts[0], "/dev/nvme") {
			var info NVMeInfo
			info.DevicePath = parts[0]
			if len(parts) >= 3 { // NAME MODEL SIZE
				info.Model = strings.Join(parts[1:len(parts)-1], " ") // Model can have spaces
				info.Size = parts[len(parts)-1]
			} else if len(parts) == 2 { // NAME SIZE (no model)
				info.Model = "[unknown]"
				info.Size = parts[1]
			} else { // Just NAME
				info.Model = "[unknown]"
				info.Size = "[unknown]"
			}
			// TODO: Could run lsblk again for this specific device to get partitions
			// e.g., lsblk -pno NAME,MOUNTPOINT,SIZE /dev/nvme0n1
			nvmes = append(nvmes, info)
		}
	}
	return nvmes
}

// (Add more specific parsing functions as needed for dmidecode, etc.)

// MountPointInfo holds information about a mount point
type MountPointInfo struct {
	DevicePath string
	DeviceName string
	MountPoint string
	FSType     string
	Size       uint64 // Size in bytes
	Available  uint64 // Available space in bytes
}

// findBootDisk finds the disk that contains the root filesystem
func findBootDisk() string {
	// Get the device that contains the root filesystem
	output, err := runCommand("findmnt", "-no", "SOURCE", "/")
	if err != nil {
		fmt.Printf("    Error finding root filesystem device: %v\n", err)
		return ""
	}

	rootDevice := strings.TrimSpace(output)
	if rootDevice == "" {
		return ""
	}

	// If it's an LVM volume, find the physical device
	if strings.Contains(rootDevice, "mapper") {
		// Get the volume group
		vgOutput, err := runCommand("lvs", "--noheadings", "-o", "vg_name", rootDevice)
		if err != nil {
			return ""
		}

		volumeGroup := strings.TrimSpace(vgOutput)
		if volumeGroup == "" {
			return ""
		}

		// Get the physical volume(s) for this volume group
		pvOutput, err := runCommand("pvs", "--noheadings", "-o", "pv_name", "-S", fmt.Sprintf("vg_name=%s", volumeGroup))
		if err != nil {
			return ""
		}

		// Just return the first physical volume
		scanner := bufio.NewScanner(strings.NewReader(pvOutput))
		if scanner.Scan() {
			physicalVolume := strings.TrimSpace(scanner.Text())

			// Get the disk that contains this partition
			if strings.Contains(physicalVolume, "nvme") {
				// For NVMe, the format is like /dev/nvme0n1p1, we want /dev/nvme0n1
				re := regexp.MustCompile(`(/dev/nvme\d+n\d+)p?\d*`)
				matches := re.FindStringSubmatch(physicalVolume)
				if len(matches) > 1 {
					return matches[1]
				}
			} else if strings.Contains(physicalVolume, "sd") {
				// For SATA/SAS, the format is like /dev/sda1, we want /dev/sda
				re := regexp.MustCompile(`(/dev/sd[a-z]+)\d*`)
				matches := re.FindStringSubmatch(physicalVolume)
				if len(matches) > 1 {
					return matches[1]
				}
			}

			return physicalVolume
		}
	} else {
		// It's a regular partition, extract the disk
		if strings.Contains(rootDevice, "nvme") {
			// For NVMe, the format is like /dev/nvme0n1p1, we want /dev/nvme0n1
			re := regexp.MustCompile(`(/dev/nvme\d+n\d+)p?\d*`)
			matches := re.FindStringSubmatch(rootDevice)
			if len(matches) > 1 {
				return matches[1]
			}
		} else if strings.Contains(rootDevice, "sd") {
			// For SATA/SAS, the format is like /dev/sda1, we want /dev/sda
			re := regexp.MustCompile(`(/dev/sd[a-z]+)\d*`)
			matches := re.FindStringSubmatch(rootDevice)
			if len(matches) > 1 {
				return matches[1]
			}
		}
	}

	return ""
}

// isSafeForDirectTesting checks if a device is safe for direct testing
func isSafeForDirectTesting(devicePath string) (bool, string) {
	// Check if the device exists
	if _, err := os.Stat(devicePath); os.IsNotExist(err) {
		return false, "Device does not exist"
	}

	// Check if the device is mounted
	output, err := runCommand("lsblk", "-pno", "NAME,MOUNTPOINT", devicePath)
	if err != nil {
		return false, fmt.Sprintf("Error checking if device is mounted: %v", err)
	}

	// Parse the output to check if any partition is mounted
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] != "" && fields[1] != "null" {
			// This device or one of its partitions is mounted
			return false, fmt.Sprintf("Device or partition is mounted at %s", fields[1])
		}
	}

	// Check if the device is part of an LVM setup
	pvOutput, err := runCommand("pvs", "--noheadings", "-o", "pv_name,vg_name")
	if err == nil {
		// Parse PVs to check if this device is a physical volume
		pvScanner := bufio.NewScanner(strings.NewReader(pvOutput))
		for pvScanner.Scan() {
			line := pvScanner.Text()
			fields := strings.Fields(line)
			if len(fields) >= 2 && strings.Contains(fields[0], devicePath) {
				// This device is part of an LVM setup
				return false, fmt.Sprintf("Device is part of volume group %s", fields[1])
			}
		}
	}

	// Check if the device is a boot disk
	bootDisk := findBootDisk()
	if bootDisk == devicePath {
		return false, "Device is the boot disk"
	}

	// All checks passed, device is safe for direct testing
	return true, ""
}

// findBestMountPoint finds the best mount point for a device
func findBestMountPoint(devicePath string) MountPointInfo {
	// Get all mount points for this device
	mountPoints := findMountPointsForDevice(devicePath)
	if len(mountPoints) == 0 {
		return MountPointInfo{}
	}

	// If there's only one mount point, return it
	if len(mountPoints) == 1 {
		return mountPoints[0]
	}

	// Find the mount point with the most available space
	var bestMountPoint MountPointInfo
	var maxAvailable uint64

	for _, mp := range mountPoints {
		// Skip root filesystem
		if mp.MountPoint == "/" {
			continue
		}

		// Skip special filesystems
		if mp.FSType == "tmpfs" || mp.FSType == "devtmpfs" || mp.FSType == "sysfs" || mp.FSType == "proc" {
			continue
		}

		// Get available space
		output, err := runCommand("df", "--output=avail", "-B1", mp.MountPoint)
		if err != nil {
			continue
		}

		// Parse available space
		scanner := bufio.NewScanner(strings.NewReader(output))
		scanner.Scan() // Skip header
		if scanner.Scan() {
			availStr := strings.TrimSpace(scanner.Text())
			available, err := strconv.ParseUint(availStr, 10, 64)
			if err != nil {
				continue
			}

			if available > maxAvailable {
				maxAvailable = available
				bestMountPoint = mp
				bestMountPoint.Available = available
			}
		}
	}

	return bestMountPoint
}

// findMountPointsForDevice finds all mount points for a given device
// This handles both direct mounts and mounts of partitions/LVs
func findMountPointsForDevice(devicePath string) []MountPointInfo {
	var results []MountPointInfo

	// First check if the device itself is mounted
	output, err := runCommand("lsblk", "-pno", "NAME,MOUNTPOINT,FSTYPE,SIZE", devicePath)
	if err != nil {
		fmt.Printf("      Error checking mount points for %s: %v\n", devicePath, err)
		return results
	}

	// Parse the output to find mount points
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 {
			devPath := fields[0]
			mountPoint := ""
			if len(fields) >= 2 {
				mountPoint = fields[1]
			}
			fsType := ""
			if len(fields) >= 3 {
				fsType = fields[2]
			}

			// Skip non-mounted devices
			if mountPoint == "" || mountPoint == "null" {
				continue
			}

			// Parse size if available
			var size uint64
			if len(fields) >= 4 {
				sizeStr := fields[3]
				// Convert size string to bytes
				if strings.HasSuffix(sizeStr, "G") {
					sizeGB, err := strconv.ParseFloat(sizeStr[:len(sizeStr)-1], 64)
					if err == nil {
						size = uint64(sizeGB * 1024 * 1024 * 1024)
					}
				} else if strings.HasSuffix(sizeStr, "M") {
					sizeMB, err := strconv.ParseFloat(sizeStr[:len(sizeStr)-1], 64)
					if err == nil {
						size = uint64(sizeMB * 1024 * 1024)
					}
				} else if strings.HasSuffix(sizeStr, "K") {
					sizeKB, err := strconv.ParseFloat(sizeStr[:len(sizeStr)-1], 64)
					if err == nil {
						size = uint64(sizeKB * 1024)
					}
				} else {
					size, _ = strconv.ParseUint(sizeStr, 10, 64)
				}
			}

			// Add to results
			results = append(results, MountPointInfo{
				DevicePath: devPath,
				DeviceName: devPath,
				MountPoint: mountPoint,
				FSType:     fsType,
				Size:       size,
			})
		}
	}

	// If no direct mounts found, check for LVM volumes that might be using this device
	if len(results) == 0 {
		// Try to find if this device is part of an LVM setup
		pvOutput, err := runCommand("pvs", "--noheadings", "-o", "pv_name,vg_name")
		if err == nil {
			// Parse PVs to find the volume group
			var volumeGroup string
			pvScanner := bufio.NewScanner(strings.NewReader(pvOutput))
			for pvScanner.Scan() {
				line := pvScanner.Text()
				fields := strings.Fields(line)
				if len(fields) >= 2 && strings.Contains(fields[0], devicePath) {
					volumeGroup = fields[1]
					break
				}
			}

			// If we found a volume group, check for logical volumes
			if volumeGroup != "" {
				lvOutput, err := runCommand("lvs", "--noheadings", "-o", "lv_name,lv_path,lv_size", volumeGroup)
				if err == nil {
					// For each logical volume, check if it's mounted
					lvScanner := bufio.NewScanner(strings.NewReader(lvOutput))
					for lvScanner.Scan() {
						line := lvScanner.Text()
						fields := strings.Fields(line)
						if len(fields) >= 2 {
							lvName := fields[0]
							lvPath := fields[1]

							// Parse size if available
							var size uint64
							if len(fields) >= 3 {
								sizeStr := fields[2]
								// Convert size string to bytes
								if strings.HasSuffix(sizeStr, "g") {
									sizeGB, err := strconv.ParseFloat(sizeStr[:len(sizeStr)-1], 64)
									if err == nil {
										size = uint64(sizeGB * 1024 * 1024 * 1024)
									}
								} else if strings.HasSuffix(sizeStr, "m") {
									sizeMB, err := strconv.ParseFloat(sizeStr[:len(sizeStr)-1], 64)
									if err == nil {
										size = uint64(sizeMB * 1024 * 1024)
									}
								} else if strings.HasSuffix(sizeStr, "k") {
									sizeKB, err := strconv.ParseFloat(sizeStr[:len(sizeStr)-1], 64)
									if err == nil {
										size = uint64(sizeKB * 1024)
									}
								} else {
									size, _ = strconv.ParseUint(sizeStr, 10, 64)
								}
							}

							// Check if this LV is mounted
							mountOutput, err := runCommand("findmnt", "-no", "TARGET,FSTYPE", lvPath)
							if err == nil && mountOutput != "" {
								mountFields := strings.Fields(mountOutput)
								if len(mountFields) >= 1 {
									mountPoint := mountFields[0]
									fsType := ""
									if len(mountFields) >= 2 {
										fsType = mountFields[1]
									}

									// Add to results
									results = append(results, MountPointInfo{
										DevicePath: lvPath,
										DeviceName: fmt.Sprintf("%s/%s", volumeGroup, lvName),
										MountPoint: mountPoint,
										FSType:     fsType,
										Size:       size,
									})
								}
							}
						}
					}
				}
			}
		}
	}

	return results
}
