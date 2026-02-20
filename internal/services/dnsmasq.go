package services

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"linuxtorouter/internal/models"
)

type DnsmasqService struct {
	configDir string
}

func NewDnsmasqService(configDir string) *DnsmasqService {
	return &DnsmasqService{
		configDir: configDir,
	}
}

// GetConfig retrieves the current Dnsmasq configuration
func (s *DnsmasqService) GetConfig() (*models.DnsmasqConfig, error) {
	configPath := s.getConfigPath()

	// If config doesn't exist, return default configuration
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return s.getDefaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config models.DnsmasqConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// UpdateDHCPConfig updates the DHCP configuration
func (s *DnsmasqService) UpdateDHCPConfig(input models.DHCPConfigInput) error {
	config, err := s.GetConfig()
	if err != nil {
		return err
	}

	// Validate input
	if err := s.validateDHCPConfig(input); err != nil {
		return err
	}

	// Update configuration
	config.DHCP.Enabled = input.Enabled
	config.DHCP.Interface = input.Interface
	config.DHCP.StartIP = input.StartIP
	config.DHCP.EndIP = input.EndIP
	config.DHCP.LeaseTime = input.LeaseTime
	config.DHCP.Gateway = input.Gateway
	config.DHCP.DNSServers = input.DNSServers

	// Save configuration
	if err := s.saveConfig(config); err != nil {
		return err
	}

	// Regenerate dnsmasq.conf
	return s.generateDnsmasqConf(config)
}

// UpdateDNSConfig updates the DNS configuration
func (s *DnsmasqService) UpdateDNSConfig(input models.DNSConfigInput) error {
	config, err := s.GetConfig()
	if err != nil {
		return err
	}

	// Validate input
	if err := s.validateDNSConfig(input); err != nil {
		return err
	}

	// Update configuration
	config.DNS.Enabled = input.Enabled
	config.DNS.Port = input.Port
	config.DNS.UpstreamServers = input.UpstreamServers
	config.DNS.CacheSize = input.CacheSize

	// Save configuration
	if err := s.saveConfig(config); err != nil {
		return err
	}

	// Regenerate dnsmasq.conf
	return s.generateDnsmasqConf(config)
}

// AddStaticLease adds a DHCP static lease
func (s *DnsmasqService) AddStaticLease(input models.StaticLeaseInput) error {
	config, err := s.GetConfig()
	if err != nil {
		return err
	}

	// Validate input
	if err := s.validateStaticLease(input); err != nil {
		return err
	}

	// Check for duplicate MAC or IP
	for _, lease := range config.DHCP.StaticLeases {
		if strings.EqualFold(lease.MAC, input.MAC) {
			return fmt.Errorf("static lease with MAC %s already exists", input.MAC)
		}
		if lease.IP == input.IP {
			return fmt.Errorf("static lease with IP %s already exists", input.IP)
		}
	}

	// Add lease
	config.DHCP.StaticLeases = append(config.DHCP.StaticLeases, models.StaticLease{
		MAC:      strings.ToLower(input.MAC),
		IP:       input.IP,
		Hostname: input.Hostname,
	})

	// Save configuration
	if err := s.saveConfig(config); err != nil {
		return err
	}

	// Regenerate dnsmasq.conf
	return s.generateDnsmasqConf(config)
}

// RemoveStaticLease removes a DHCP static lease by MAC address
func (s *DnsmasqService) RemoveStaticLease(mac string) error {
	config, err := s.GetConfig()
	if err != nil {
		return err
	}

	// Find and remove lease
	found := false
	newLeases := make([]models.StaticLease, 0)
	for _, lease := range config.DHCP.StaticLeases {
		if !strings.EqualFold(lease.MAC, mac) {
			newLeases = append(newLeases, lease)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("static lease with MAC %s not found", mac)
	}

	config.DHCP.StaticLeases = newLeases

	// Save configuration
	if err := s.saveConfig(config); err != nil {
		return err
	}

	// Regenerate dnsmasq.conf
	return s.generateDnsmasqConf(config)
}

// AddDomainRule adds a domain-specific DNS rule
func (s *DnsmasqService) AddDomainRule(input models.DomainRuleInput) error {
	config, err := s.GetConfig()
	if err != nil {
		return err
	}

	// Validate input
	if err := s.validateDomainRule(input); err != nil {
		return err
	}

	// Check for duplicate domain
	for _, rule := range config.DNS.DomainRules {
		if rule.Domain == input.Domain {
			return fmt.Errorf("domain rule for %s already exists", input.Domain)
		}
	}

	// Add rule
	config.DNS.DomainRules = append(config.DNS.DomainRules, models.DomainRule{
		Domain:    input.Domain,
		CustomDNS: input.CustomDNS,
		IPSetName: input.IPSetName,
	})

	// Save configuration
	if err := s.saveConfig(config); err != nil {
		return err
	}

	// Regenerate dnsmasq.conf
	return s.generateDnsmasqConf(config)
}

// RemoveDomainRule removes a domain-specific DNS rule
func (s *DnsmasqService) RemoveDomainRule(domain string) error {
	config, err := s.GetConfig()
	if err != nil {
		return err
	}

	// Find and remove rule
	found := false
	newRules := make([]models.DomainRule, 0)
	for _, rule := range config.DNS.DomainRules {
		if rule.Domain != domain {
			newRules = append(newRules, rule)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("domain rule for %s not found", domain)
	}

	config.DNS.DomainRules = newRules

	// Save configuration
	if err := s.saveConfig(config); err != nil {
		return err
	}

	// Regenerate dnsmasq.conf
	return s.generateDnsmasqConf(config)
}

// AddDNSHost adds a custom DNS host entry
func (s *DnsmasqService) AddDNSHost(input models.DNSHostInput) error {
	config, err := s.GetConfig()
	if err != nil {
		return err
	}

	// Validate input
	if err := s.validateDNSHost(input); err != nil {
		return err
	}

	// Check for duplicate hostname
	for _, host := range config.DNS.CustomHosts {
		if strings.EqualFold(host.Hostname, input.Hostname) {
			return fmt.Errorf("DNS host entry for %s already exists", input.Hostname)
		}
	}

	// Add host
	config.DNS.CustomHosts = append(config.DNS.CustomHosts, models.DNSHost{
		Hostname: strings.ToLower(input.Hostname),
		IP:       input.IP,
	})

	// Save configuration
	if err := s.saveConfig(config); err != nil {
		return err
	}

	// Regenerate dnsmasq.conf
	return s.generateDnsmasqConf(config)
}

// RemoveDNSHost removes a custom DNS host entry
func (s *DnsmasqService) RemoveDNSHost(hostname string) error {
	config, err := s.GetConfig()
	if err != nil {
		return err
	}

	// Find and remove host
	found := false
	newHosts := make([]models.DNSHost, 0)
	for _, host := range config.DNS.CustomHosts {
		if !strings.EqualFold(host.Hostname, hostname) {
			newHosts = append(newHosts, host)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("DNS host entry for %s not found", hostname)
	}

	config.DNS.CustomHosts = newHosts

	// Save configuration
	if err := s.saveConfig(config); err != nil {
		return err
	}

	// Regenerate dnsmasq.conf
	return s.generateDnsmasqConf(config)
}

// GetDHCPLeases retrieves active DHCP leases
func (s *DnsmasqService) GetDHCPLeases() ([]models.DHCPLease, error) {
	leaseFile := "/var/lib/misc/dnsmasq.leases"

	// Check if file exists
	if _, err := os.Stat(leaseFile); os.IsNotExist(err) {
		return []models.DHCPLease{}, nil
	}

	file, err := os.Open(leaseFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open lease file: %w", err)
	}
	defer file.Close()

	leases := make([]models.DHCPLease, 0)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Format: <expiry_time> <mac> <ip> <hostname> <client_id>
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		expiryTime, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}

		lease := models.DHCPLease{
			ExpiryTime: expiryTime,
			MAC:        fields[1],
			IP:         fields[2],
			Hostname:   fields[3],
		}

		if len(fields) >= 5 {
			lease.ClientID = fields[4]
		}

		leases = append(leases, lease)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read lease file: %w", err)
	}

	return leases, nil
}

// Start starts the Dnsmasq service
func (s *DnsmasqService) Start() error {
	cmd := exec.Command("systemctl", "start", "dnsmasq")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start dnsmasq: %w, output: %s", err, string(output))
	}
	return nil
}

// Stop stops the Dnsmasq service
func (s *DnsmasqService) Stop() error {
	cmd := exec.Command("systemctl", "stop", "dnsmasq")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stop dnsmasq: %w, output: %s", err, string(output))
	}
	return nil
}

// Restart restarts the Dnsmasq service
func (s *DnsmasqService) Restart() error {
	cmd := exec.Command("systemctl", "restart", "dnsmasq")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restart dnsmasq: %w, output: %s", err, string(output))
	}
	return nil
}

// GetStatus returns the status of the Dnsmasq service
func (s *DnsmasqService) GetStatus() (string, error) {
	cmd := exec.Command("systemctl", "is-active", "dnsmasq")
	output, err := cmd.Output()
	if err != nil {
		// is-active returns non-zero exit code if service is not active
		return strings.TrimSpace(string(output)), nil
	}
	return strings.TrimSpace(string(output)), nil
}

// SaveConfig persists the current configuration
func (s *DnsmasqService) SaveConfig() error {
	config, err := s.GetConfig()
	if err != nil {
		return err
	}

	// Generate dnsmasq configuration files
	return s.generateDnsmasqConf(config)
}

// RestoreConfig restores the configuration from saved state
func (s *DnsmasqService) RestoreConfig() error {
	config, err := s.GetConfig()
	if err != nil {
		return err
	}

	// Generate dnsmasq configuration files
	if err := s.generateDnsmasqConf(config); err != nil {
		return err
	}

	// Restart service if configuration exists and is enabled
	if config.DHCP.Enabled || config.DNS.Enabled {
		return s.Restart()
	}

	return nil
}

// Helper methods

func (s *DnsmasqService) getConfigPath() string {
	return filepath.Join(s.configDir, "dnsmasq", "config.json")
}

func (s *DnsmasqService) getDnsmasqConfDir() string {
	return "/etc/dnsmasq.d"
}

func (s *DnsmasqService) getDefaultConfig() *models.DnsmasqConfig {
	return &models.DnsmasqConfig{
		DHCP: models.DHCPConfig{
			Enabled:      false,
			Interface:    "",
			StartIP:      "",
			EndIP:        "",
			LeaseTime:    "12h",
			Gateway:      "",
			DNSServers:   []string{},
			StaticLeases: []models.StaticLease{},
		},
		DNS: models.DNSConfig{
			Enabled:         false,
			Port:            53,
			UpstreamServers: []string{"8.8.8.8", "8.8.4.4"},
			CacheSize:       1000,
			DomainRules:     []models.DomainRule{},
			CustomHosts:     []models.DNSHost{},
		},
	}
}

func (s *DnsmasqService) saveConfig(config *models.DnsmasqConfig) error {
	// Ensure config directory exists
	configPath := s.getConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal configuration to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func (s *DnsmasqService) generateDnsmasqConf(config *models.DnsmasqConfig) error {
	// Ensure dnsmasq.d directory exists
	dnsmasqDir := s.getDnsmasqConfDir()
	if err := os.MkdirAll(dnsmasqDir, 0755); err != nil {
		return fmt.Errorf("failed to create dnsmasq.d directory: %w", err)
	}

	var confLines []string

	// DNS Configuration
	if config.DNS.Enabled {
		confLines = append(confLines, "# DNS Configuration")
		confLines = append(confLines, fmt.Sprintf("port=%d", config.DNS.Port))
		confLines = append(confLines, fmt.Sprintf("cache-size=%d", config.DNS.CacheSize))

		// Upstream DNS servers
		for _, server := range config.DNS.UpstreamServers {
			confLines = append(confLines, fmt.Sprintf("server=%s", server))
		}

		// Domain-specific rules
		for _, rule := range config.DNS.DomainRules {
			if rule.CustomDNS != "" {
				confLines = append(confLines, fmt.Sprintf("server=/%s/%s", rule.Domain, rule.CustomDNS))
			}
			if rule.IPSetName != "" {
				confLines = append(confLines, fmt.Sprintf("ipset=/%s/%s", rule.Domain, rule.IPSetName))
			}
		}

		// Custom host entries
		for _, host := range config.DNS.CustomHosts {
			confLines = append(confLines, fmt.Sprintf("address=/%s/%s", host.Hostname, host.IP))
		}

		confLines = append(confLines, "")
	}

	// DHCP Configuration
	if config.DHCP.Enabled {
		confLines = append(confLines, "# DHCP Configuration")
		confLines = append(confLines, fmt.Sprintf("interface=%s", config.DHCP.Interface))
		confLines = append(confLines, "bind-interfaces")

		// DHCP range
		dhcpRange := fmt.Sprintf("dhcp-range=%s,%s,%s",
			config.DHCP.StartIP,
			config.DHCP.EndIP,
			config.DHCP.LeaseTime)
		confLines = append(confLines, dhcpRange)

		// Gateway (router option)
		if config.DHCP.Gateway != "" {
			confLines = append(confLines, fmt.Sprintf("dhcp-option=3,%s", config.DHCP.Gateway))
		}

		// DNS servers
		if len(config.DHCP.DNSServers) > 0 {
			dnsServers := strings.Join(config.DHCP.DNSServers, ",")
			confLines = append(confLines, fmt.Sprintf("dhcp-option=6,%s", dnsServers))
		}

		// Static leases
		for _, lease := range config.DHCP.StaticLeases {
			if lease.Hostname != "" {
				confLines = append(confLines, fmt.Sprintf("dhcp-host=%s,%s,%s",
					lease.MAC, lease.IP, lease.Hostname))
			} else {
				confLines = append(confLines, fmt.Sprintf("dhcp-host=%s,%s",
					lease.MAC, lease.IP))
			}
		}

		confLines = append(confLines, "")
	}

	// Additional settings
	confLines = append(confLines, "# Additional Settings")
	confLines = append(confLines, "no-resolv")   // Don't read /etc/resolv.conf
	confLines = append(confLines, "no-poll")     // Don't poll /etc/resolv.conf for changes
	confLines = append(confLines, "log-queries") // Log DNS queries
	confLines = append(confLines, "log-dhcp")    // Log DHCP transactions

	// Write configuration file
	confPath := filepath.Join(dnsmasqDir, "linux2router.conf")
	content := strings.Join(confLines, "\n") + "\n"

	if err := os.WriteFile(confPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write dnsmasq configuration: %w", err)
	}

	return nil
}

// Validation methods

func (s *DnsmasqService) validateDHCPConfig(input models.DHCPConfigInput) error {
	if !input.Enabled {
		return nil
	}

	if input.Interface == "" {
		return fmt.Errorf("interface is required")
	}

	if input.StartIP == "" {
		return fmt.Errorf("start IP is required")
	}

	if input.EndIP == "" {
		return fmt.Errorf("end IP is required")
	}

	// Validate IP addresses
	if net.ParseIP(input.StartIP) == nil {
		return fmt.Errorf("invalid start IP address: %s", input.StartIP)
	}

	if net.ParseIP(input.EndIP) == nil {
		return fmt.Errorf("invalid end IP address: %s", input.EndIP)
	}

	if input.Gateway != "" && net.ParseIP(input.Gateway) == nil {
		return fmt.Errorf("invalid gateway IP address: %s", input.Gateway)
	}

	// Validate DNS servers
	for _, server := range input.DNSServers {
		if net.ParseIP(server) == nil {
			return fmt.Errorf("invalid DNS server IP address: %s", server)
		}
	}

	// Validate lease time format
	if !s.isValidLeaseTime(input.LeaseTime) {
		return fmt.Errorf("invalid lease time format: %s (use format like '12h', '24h', '7d')", input.LeaseTime)
	}

	return nil
}

func (s *DnsmasqService) validateDNSConfig(input models.DNSConfigInput) error {
	if !input.Enabled {
		return nil
	}

	if input.Port < 1 || input.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	if input.CacheSize < 0 {
		return fmt.Errorf("cache size must be non-negative")
	}

	// Validate upstream servers
	for _, server := range input.UpstreamServers {
		if net.ParseIP(server) == nil {
			return fmt.Errorf("invalid upstream DNS server IP address: %s", server)
		}
	}

	return nil
}

func (s *DnsmasqService) validateStaticLease(input models.StaticLeaseInput) error {
	if input.MAC == "" {
		return fmt.Errorf("MAC address is required")
	}

	if input.IP == "" {
		return fmt.Errorf("IP address is required")
	}

	// Validate MAC address format
	macRegex := regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`)
	if !macRegex.MatchString(input.MAC) {
		return fmt.Errorf("invalid MAC address format: %s", input.MAC)
	}

	// Validate IP address
	if net.ParseIP(input.IP) == nil {
		return fmt.Errorf("invalid IP address: %s", input.IP)
	}

	return nil
}

func (s *DnsmasqService) validateDomainRule(input models.DomainRuleInput) error {
	if input.Domain == "" {
		return fmt.Errorf("domain is required")
	}

	// Validate domain format (basic check)
	domainRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
	if !domainRegex.MatchString(input.Domain) {
		return fmt.Errorf("invalid domain format: %s", input.Domain)
	}

	// Validate custom DNS server if provided
	if input.CustomDNS != "" {
		if net.ParseIP(input.CustomDNS) == nil {
			return fmt.Errorf("invalid custom DNS server IP address: %s", input.CustomDNS)
		}
	}

	// Note: IPSet validation will be done at the handler level where IPSetService is available

	return nil
}

func (s *DnsmasqService) validateDNSHost(input models.DNSHostInput) error {
	if input.Hostname == "" {
		return fmt.Errorf("hostname is required")
	}

	if input.IP == "" {
		return fmt.Errorf("IP address is required")
	}

	// Validate IP address
	if net.ParseIP(input.IP) == nil {
		return fmt.Errorf("invalid IP address: %s", input.IP)
	}

	return nil
}

func (s *DnsmasqService) isValidLeaseTime(leaseTime string) bool {
	// Valid formats: 1h, 12h, 24h, 7d, etc.
	matched, _ := regexp.MatchString(`^\d+[hHdDwWmM]$`, leaseTime)
	return matched
}

// GetInterfaces returns a list of available network interfaces
func (s *DnsmasqService) GetInterfaces() ([]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}

	var result []string
	for _, iface := range interfaces {
		// Skip loopback
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		result = append(result, iface.Name)
	}

	return result, nil
}

// DNSQueryLog represents a DNS query log entry
type DNSQueryLog struct {
	Timestamp string `json:"timestamp"`
	QueryType string `json:"query_type"` // "query" or "forwarded"
	Domain    string `json:"domain"`
	QueryFrom string `json:"query_from"` // Client IP
	Result    string `json:"result"`     // IP address or status
}

// GetDNSQueryLogs retrieves recent DNS query logs from system journal
func (s *DnsmasqService) GetDNSQueryLogs(limit int) ([]DNSQueryLog, error) {
	if limit <= 0 {
		limit = 100
	}

	// Use journalctl to get dnsmasq logs
	cmd := exec.Command("journalctl", "-u", "dnsmasq", "-n", fmt.Sprintf("%d", limit), "--no-pager", "-o", "short-iso")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	logs := []DNSQueryLog{}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	// Regex patterns for parsing dnsmasq query logs
	queryPattern := regexp.MustCompile(`^(\S+)\s+\S+\s+dnsmasq\[\d+\]:\s+query\[(\w+)\]\s+(\S+)\s+from\s+(\S+)`)
	forwardedPattern := regexp.MustCompile(`^(\S+)\s+\S+\s+dnsmasq\[\d+\]:\s+forwarded\s+(\S+)\s+to\s+(\S+)`)
	replyPattern := regexp.MustCompile(`^(\S+)\s+\S+\s+dnsmasq\[\d+\]:\s+reply\s+(\S+)\s+is\s+(\S+)`)
	cachedPattern := regexp.MustCompile(`^(\S+)\s+\S+\s+dnsmasq\[\d+\]:\s+cached\s+(\S+)\s+is\s+(\S+)`)

	for scanner.Scan() {
		line := scanner.Text()

		// Parse query
		if matches := queryPattern.FindStringSubmatch(line); matches != nil {
			logs = append(logs, DNSQueryLog{
				Timestamp: matches[1],
				QueryType: "query",
				Domain:    matches[3],
				QueryFrom: matches[4],
				Result:    matches[2], // Record type (A, AAAA, etc.)
			})
		}

		// Parse forwarded
		if matches := forwardedPattern.FindStringSubmatch(line); matches != nil {
			if len(logs) > 0 && logs[len(logs)-1].Domain == matches[2] {
				logs[len(logs)-1].Result = "forwarded to " + matches[3]
			}
		}

		// Parse reply
		if matches := replyPattern.FindStringSubmatch(line); matches != nil {
			if len(logs) > 0 && logs[len(logs)-1].Domain == matches[2] {
				logs[len(logs)-1].Result = matches[3]
			}
		}

		// Parse cached
		if matches := cachedPattern.FindStringSubmatch(line); matches != nil {
			logs = append(logs, DNSQueryLog{
				Timestamp: matches[1],
				QueryType: "cached",
				Domain:    matches[2],
				QueryFrom: "cache",
				Result:    matches[3],
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse logs: %w", err)
	}

	return logs, nil
}

// GetDNSStats returns DNS statistics
type DNSStats struct {
	CacheHits     int `json:"cache_hits"`
	CacheMisses   int `json:"cache_misses"`
	TotalQueries  int `json:"total_queries"`
	UniqueClients int `json:"unique_clients"`
}

// GetDNSStatistics retrieves DNS query statistics
func (s *DnsmasqService) GetDNSStatistics() (*DNSStats, error) {
	logs, err := s.GetDNSQueryLogs(1000)
	if err != nil {
		return nil, err
	}

	stats := &DNSStats{}
	clientMap := make(map[string]bool)

	for _, log := range logs {
		stats.TotalQueries++

		if log.QueryType == "cached" {
			stats.CacheHits++
		} else if log.QueryType == "query" {
			stats.CacheMisses++
		}

		if log.QueryFrom != "cache" && log.QueryFrom != "" {
			clientMap[log.QueryFrom] = true
		}
	}

	stats.UniqueClients = len(clientMap)

	return stats, nil
}
