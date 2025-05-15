package cmd

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

// WebServerConfig holds configuration for the embedded web server
type WebServerConfig struct {
	Port       int
	ResultsDir string
	SysInfo    *SystemInfo
}

// StartWebServer starts the embedded web server for viewing benchmark results
func StartWebServer(config WebServerConfig) error {
	// Create HTTP server mux
	mux := http.NewServeMux()

	// Serve static files
	mux.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		// Simple static file server for CSS, JS, etc.
		http.ServeFile(w, r, r.URL.Path[1:])
	})

	// Main page handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		// Create a template for the main page
		tmpl := template.Must(template.New("index").Parse(indexTemplate))

		// Execute the template with the system info
		if err := tmpl.Execute(w, config.SysInfo); err != nil {
			http.Error(w, fmt.Sprintf("Error rendering template: %v", err), http.StatusInternalServerError)
			return
		}
	})

	// API endpoint for raw JSON data
	mux.HandleFunc("/api/results", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config.SysInfo)
	})

	// API endpoint for system info
	mux.HandleFunc("/api/sysinfo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		sysInfoMap := map[string]string{
			"CPUModel":   config.SysInfo.CPUModel,
			"CPUCores":   config.SysInfo.CPUCores,
			"CPUThreads": config.SysInfo.CPUThreads,
			"RAMTotal":   config.SysInfo.RAMTotal,
			"OSName":     config.SysInfo.OSName,
			"OSVersion":  config.SysInfo.OSVersion,
		}
		json.NewEncoder(w).Encode(sysInfoMap)
	})

	// API endpoint for CPU benchmark results
	mux.HandleFunc("/api/cpu", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cpuResults := map[string]string{
			"SingleThread": config.SysInfo.SysbenchSingleThreadScore,
			"MultiThread":  config.SysInfo.SysbenchMultiThreadScore,
		}
		json.NewEncoder(w).Encode(cpuResults)
	})

	// API endpoint for memory benchmark results
	mux.HandleFunc("/api/memory", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		memResults := map[string]string{
			"Copy":  config.SysInfo.StreamCopyBandwidthMBs,
			"Scale": config.SysInfo.StreamScaleBandwidthMBs,
			"Add":   config.SysInfo.StreamAddBandwidthMBs,
			"Triad": config.SysInfo.StreamTriadBandwidthMBs,
		}
		json.NewEncoder(w).Encode(memResults)
	})

	// API endpoint for disk benchmark results
	mux.HandleFunc("/api/disk", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config.SysInfo.FioResults)
	})

	// API endpoint for network benchmark results
	mux.HandleFunc("/api/network", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		netResults := map[string]interface{}{
			"Speedtest": config.SysInfo.SpeedtestResults,
			"Iperf3":    config.SysInfo.Iperf3Results,
			"Netblast":  config.SysInfo.NetblastResults,
		}
		json.NewEncoder(w).Encode(netResults)
	})

	// Start the server
	addr := fmt.Sprintf("0.0.0.0:%d", config.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Get the server's IP address for display
	serverIP := getServerIP()

	// Open browser only if we're on a desktop system
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		go func() {
			time.Sleep(500 * time.Millisecond) // Give the server a moment to start
			url := fmt.Sprintf("http://localhost:%d", config.Port)
			fmt.Printf("Opening web browser to %s\n", url)
			openBrowser(url)
		}()
	}

	fmt.Printf("Starting web server on http://%s:%d\n", serverIP, config.Port)
	fmt.Printf("You can access the results from any browser at:\n")
	fmt.Printf("  http://%s:%d\n", serverIP, config.Port)
	fmt.Println("Press Ctrl+C to stop the server")
	return server.ListenAndServe()
}

// openBrowser opens the default browser to the specified URL
func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	if err != nil {
		fmt.Printf("Error opening browser: %v\n", err)
	}
}

// FindAvailablePort finds an available port starting from the given port
func FindAvailablePort(startPort int) int {
	port := startPort
	for {
		addr := fmt.Sprintf(":%d", port)
		server, err := net.Listen("tcp", addr)
		if err == nil {
			server.Close()
			return port
		}
		port++
		if port > 65535 {
			return startPort // If no ports are available, return the start port
		}
	}
}

// getServerIP returns the server's primary IP address
func getServerIP() string {
	// Default to localhost if we can't determine the IP
	defaultIP := "127.0.0.1"

	// Get all network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return defaultIP
	}

	// Look for a suitable interface
	for _, iface := range interfaces {
		// Skip loopback, down, and interfaces without addresses
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Get addresses for this interface
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		// Look for a suitable IPv4 address
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Skip IPv6 addresses
			if ip == nil || ip.To4() == nil {
				continue
			}

			// Skip loopback addresses
			if ip.IsLoopback() {
				continue
			}

			// Skip link-local addresses
			if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}

			// We found a suitable address
			return ip.String()
		}
	}

	// If we couldn't find a suitable address, return the default
	return defaultIP
}

// HTML template for the main page
const indexTemplate = `<!DOCTYPE html>
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
        .tabs {
            display: flex;
            margin-bottom: 20px;
        }
        .tab {
            padding: 10px 20px;
            cursor: pointer;
            border: 1px solid #ddd;
            border-bottom: none;
            border-radius: 5px 5px 0 0;
            background-color: #f2f2f2;
            margin-right: 5px;
        }
        .tab.active {
            background-color: #3498db;
            color: white;
        }
        .tab-content {
            display: none;
        }
        .tab-content.active {
            display: block;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>HyprBench Results</h1>
        <p>Version: {{.HyprBenchVersion}}</p>
        <p>Date: {{.TestDate}}</p>
        <p>Hostname: {{.Hostname}}</p>
    </div>

    <div class="tabs">
        <div class="tab active" onclick="openTab(event, 'system')">System Info</div>
        <div class="tab" onclick="openTab(event, 'cpu')">CPU</div>
        <div class="tab" onclick="openTab(event, 'memory')">Memory</div>
        <div class="tab" onclick="openTab(event, 'disk')">Disk I/O</div>
        <div class="tab" onclick="openTab(event, 'network')">Network</div>
        <div class="tab" onclick="openTab(event, 'stress')">Stress</div>
        <div class="tab" onclick="openTab(event, 'unixbench')">UnixBench</div>
    </div>

    <div id="system" class="tab-content active">
        <div class="section">
            <h2>System Information</h2>
            <table>
                <tr><th>Component</th><th>Details</th></tr>
                <tr><td>CPU Model</td><td>{{.CPUModel}}</td></tr>
                <tr><td>CPU Cores</td><td>{{.CPUCores}}</td></tr>
                <tr><td>CPU Threads</td><td>{{.CPUThreads}}</td></tr>
                <tr><td>CPU Speed</td><td>{{.CPUSpeed}}</td></tr>
                <tr><td>CPU Cache</td><td>{{.CPUCache}}</td></tr>
                <tr><td>RAM Total</td><td>{{.RAMTotal}}</td></tr>
                <tr><td>RAM Type</td><td>{{.RAMType}}</td></tr>
                <tr><td>RAM Speed</td><td>{{.RAMSpeed}}</td></tr>
                <tr><td>Motherboard</td><td>{{.MotherboardMfr}} {{.MotherboardModel}}</td></tr>
                <tr><td>OS</td><td>{{.OSName}} {{.OSVersion}}</td></tr>
                <tr><td>Kernel</td><td>{{.KernelVersion}}</td></tr>
            </table>
        </div>
    </div>

    <div id="cpu" class="tab-content">
        <div class="section">
            <h2>CPU Benchmark Results</h2>
            <table>
                <tr><th>Test</th><th>Score</th></tr>
                <tr><td>Sysbench Single-Thread</td><td class="highlight">{{.SysbenchSingleThreadScore}} events/sec</td></tr>
                <tr><td>Sysbench Multi-Thread</td><td class="highlight">{{.SysbenchMultiThreadScore}} events/sec</td></tr>
            </table>
        </div>
    </div>

    <div id="memory" class="tab-content">
        <div class="section">
            <h2>Memory Benchmark Results (STREAM)</h2>
            <table>
                <tr><th>Test</th><th>Bandwidth (MB/s)</th></tr>
                <tr><td>Copy</td><td class="highlight">{{.StreamCopyBandwidthMBs}}</td></tr>
                <tr><td>Scale</td><td class="highlight">{{.StreamScaleBandwidthMBs}}</td></tr>
                <tr><td>Add</td><td class="highlight">{{.StreamAddBandwidthMBs}}</td></tr>
                <tr><td>Triad</td><td class="highlight">{{.StreamTriadBandwidthMBs}}</td></tr>
            </table>
        </div>
    </div>

    <script>
        function openTab(evt, tabName) {
            var i, tabcontent, tablinks;
            tabcontent = document.getElementsByClassName("tab-content");
            for (i = 0; i < tabcontent.length; i++) {
                tabcontent[i].className = tabcontent[i].className.replace(" active", "");
            }
            tablinks = document.getElementsByClassName("tab");
            for (i = 0; i < tablinks.length; i++) {
                tablinks[i].className = tablinks[i].className.replace(" active", "");
            }
            document.getElementById(tabName).className += " active";
            evt.currentTarget.className += " active";
        }
    </script>
</body>
</html>
`
