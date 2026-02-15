package handlers

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"linuxtorouter/internal/middleware"
	"linuxtorouter/internal/services"
)

type DashboardHandler struct {
	templates      TemplateExecutor
	netlinkService *services.NetlinkService
}

func NewDashboardHandler(templates TemplateExecutor, netlinkService *services.NetlinkService) *DashboardHandler {
	return &DashboardHandler{
		templates:      templates,
		netlinkService: netlinkService,
	}
}

type SystemInfo struct {
	Hostname      string
	KernelVersion string
	Uptime        string
	LoadAverage   string
	MemoryUsed    string
	MemoryTotal   string
	MemoryPercent int
}

type NetworkStats struct {
	TotalInterfaces int
	ActiveInterfaces int
	TotalRxBytes    uint64
	TotalTxBytes    uint64
}

type DashboardData struct {
	SystemInfo   SystemInfo
	NetworkStats NetworkStats
	Interfaces   []InterfaceSummary
}

type InterfaceSummary struct {
	Name    string
	State   string
	IPv4    string
	RxBytes string
	TxBytes string
}

func (h *DashboardHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	data := map[string]interface{}{
		"Title":      "Dashboard",
		"ActivePage": "dashboard",
		"User":       user,
		"Dashboard":  h.getDashboardData(),
	}

	if err := h.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *DashboardHandler) Stats(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Dashboard": h.getDashboardData(),
	}

	if err := h.templates.ExecuteTemplate(w, "dashboard_stats", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *DashboardHandler) getDashboardData() DashboardData {
	sysInfo := h.getSystemInfo()
	netStats, interfaces := h.getNetworkStats()

	return DashboardData{
		SystemInfo:   sysInfo,
		NetworkStats: netStats,
		Interfaces:   interfaces,
	}
}

func (h *DashboardHandler) getSystemInfo() SystemInfo {
	info := SystemInfo{}

	// Hostname
	if hostname, err := os.Hostname(); err == nil {
		info.Hostname = hostname
	}

	// Kernel version
	if data, err := os.ReadFile("/proc/version"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) >= 3 {
			info.KernelVersion = parts[2]
		}
	}

	// Uptime
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) >= 1 {
			if uptime, err := strconv.ParseFloat(parts[0], 64); err == nil {
				info.Uptime = formatUptime(int64(uptime))
			}
		}
	}

	// Load average
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) >= 3 {
			info.LoadAverage = strings.Join(parts[:3], " ")
		}
	}

	// Memory info
	memInfo := parseMemInfo()
	if memInfo != nil {
		info.MemoryTotal = formatBytes(memInfo["MemTotal"])
		used := memInfo["MemTotal"] - memInfo["MemAvailable"]
		info.MemoryUsed = formatBytes(used)
		if memInfo["MemTotal"] > 0 {
			info.MemoryPercent = int(float64(used) / float64(memInfo["MemTotal"]) * 100)
		}
	}

	return info
}

func (h *DashboardHandler) getNetworkStats() (NetworkStats, []InterfaceSummary) {
	stats := NetworkStats{}
	var interfaces []InterfaceSummary

	links, err := h.netlinkService.ListInterfaces()
	if err != nil {
		log.Printf("Failed to get interfaces: %v", err)
		return stats, interfaces
	}

	stats.TotalInterfaces = len(links)

	for _, link := range links {
		if link.Name == "lo" {
			continue
		}

		iface := InterfaceSummary{
			Name:  link.Name,
			State: link.State,
		}

		if link.State == "UP" {
			stats.ActiveInterfaces++
		}

		if len(link.IPv4Addrs) > 0 {
			iface.IPv4 = link.IPv4Addrs[0]
		}

		// Get interface statistics
		rxBytes, txBytes := getInterfaceStats(link.Name)
		stats.TotalRxBytes += rxBytes
		stats.TotalTxBytes += txBytes
		iface.RxBytes = formatBytes(rxBytes)
		iface.TxBytes = formatBytes(txBytes)

		interfaces = append(interfaces, iface)
	}

	return stats, interfaces
}

func parseMemInfo() map[string]uint64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil
	}
	defer file.Close()

	info := make(map[string]uint64)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			key := strings.TrimSuffix(fields[0], ":")
			if val, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
				// Values in /proc/meminfo are in kB
				info[key] = val * 1024
			}
		}
	}

	return info
}

func getInterfaceStats(name string) (rxBytes, txBytes uint64) {
	rxData, err := os.ReadFile("/sys/class/net/" + name + "/statistics/rx_bytes")
	if err == nil {
		rxBytes, _ = strconv.ParseUint(strings.TrimSpace(string(rxData)), 10, 64)
	}

	txData, err := os.ReadFile("/sys/class/net/" + name + "/statistics/tx_bytes")
	if err == nil {
		txBytes, _ = strconv.ParseUint(strings.TrimSpace(string(txData)), 10, 64)
	}

	return
}

func formatUptime(seconds int64) string {
	duration := time.Duration(seconds) * time.Second
	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	if days > 0 {
		return strconv.Itoa(days) + "d " + strconv.Itoa(hours) + "h " + strconv.Itoa(minutes) + "m"
	}
	if hours > 0 {
		return strconv.Itoa(hours) + "h " + strconv.Itoa(minutes) + "m"
	}
	return strconv.Itoa(minutes) + "m"
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return strconv.FormatUint(bytes, 10) + " B"
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return strconv.FormatFloat(float64(bytes)/float64(div), 'f', 1, 64) + " " + []string{"KB", "MB", "GB", "TB"}[exp]
}

func getActiveConnections() int {
	cmd := exec.Command("ss", "-t", "-u", "-n", "state", "established")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(string(output), "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "Netid") {
			count++
		}
	}
	return count
}
