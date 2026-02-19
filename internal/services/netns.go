package services

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"linuxtorouter/internal/models"
)

type NetnsService struct {
	configDir string
}

func NewNetnsService(configDir string) *NetnsService {
	return &NetnsService{configDir: configDir}
}

// ListNamespaces returns all network namespaces
func (s *NetnsService) ListNamespaces() ([]models.NetworkNamespace, error) {
	cmd := exec.Command("ip", "netns", "list")
	output, err := cmd.Output()
	if err != nil {
		// No namespaces is not an error
		return []models.NetworkNamespace{}, nil
	}

	var namespaces []models.NetworkNamespace
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Format: "name (id: xxx)" or just "name"
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		ns := models.NetworkNamespace{
			Name: parts[0],
		}

		// Extract ID if present
		if len(parts) >= 3 && parts[1] == "(id:" {
			ns.ID = strings.TrimSuffix(parts[2], ")")
		}

		// Get interfaces in this namespace
		ns.Interfaces = s.getNamespaceInterfaces(ns.Name)

		// Check if persistent config exists
		ns.Persistent = s.hasPersistedConfig(ns.Name)

		namespaces = append(namespaces, ns)
	}

	return namespaces, nil
}

// GetNamespace returns details for a specific namespace
func (s *NetnsService) GetNamespace(name string) (*models.NetworkNamespace, error) {
	namespaces, err := s.ListNamespaces()
	if err != nil {
		return nil, err
	}

	for _, ns := range namespaces {
		if ns.Name == name {
			return &ns, nil
		}
	}

	return nil, fmt.Errorf("namespace not found: %s", name)
}

// CreateNamespace creates a new network namespace
func (s *NetnsService) CreateNamespace(input models.NetnsCreateInput) error {
	if input.Name == "" {
		return fmt.Errorf("namespace name is required")
	}

	// Validate name (alphanumeric, dash, underscore)
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(input.Name) {
		return fmt.Errorf("invalid namespace name: only alphanumeric, dash, and underscore allowed")
	}

	cmd := exec.Command("ip", "netns", "add", input.Name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create namespace: %s", string(output))
	}

	// Enable loopback interface in the new namespace
	loCmd := exec.Command("ip", "netns", "exec", input.Name, "ip", "link", "set", "lo", "up")
	loCmd.Run() // Ignore error

	return nil
}

// DeleteNamespace deletes a network namespace
func (s *NetnsService) DeleteNamespace(name string) error {
	if name == "" {
		return fmt.Errorf("namespace name is required")
	}

	cmd := exec.Command("ip", "netns", "delete", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete namespace: %s", string(output))
	}

	// Remove persisted config if exists
	configPath := filepath.Join(s.configDir, "netns", name)
	os.RemoveAll(configPath)

	return nil
}

// getNamespaceInterfaces returns interfaces in a namespace
func (s *NetnsService) getNamespaceInterfaces(namespace string) []string {
	cmd := exec.Command("ip", "netns", "exec", namespace, "ip", "-o", "link", "show")
	output, err := cmd.Output()
	if err != nil {
		return []string{}
	}

	var interfaces []string
	re := regexp.MustCompile(`^\d+:\s+([^:@]+)`)

	for _, line := range strings.Split(string(output), "\n") {
		if matches := re.FindStringSubmatch(line); matches != nil {
			ifName := strings.TrimSpace(matches[1])
			if ifName != "lo" { // Skip loopback
				interfaces = append(interfaces, ifName)
			}
		}
	}

	return interfaces
}

// GetNamespaceInterfaceDetails returns detailed interface info for a namespace
func (s *NetnsService) GetNamespaceInterfaceDetails(namespace string) ([]models.NetnsInterfaceInfo, error) {
	cmd := exec.Command("ip", "netns", "exec", namespace, "ip", "-d", "link", "show")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}

	var interfaces []models.NetnsInterfaceInfo
	lines := strings.Split(string(output), "\n")

	var current *models.NetnsInterfaceInfo
	for _, line := range lines {
		// Interface line: "2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500..."
		if match := regexp.MustCompile(`^\d+:\s+([^:@]+)[@:]`).FindStringSubmatch(line); match != nil {
			if current != nil && current.Name != "lo" {
				interfaces = append(interfaces, *current)
			}
			current = &models.NetnsInterfaceInfo{
				Name: strings.TrimSpace(match[1]),
			}
			if strings.Contains(line, "UP") {
				current.State = "UP"
			} else {
				current.State = "DOWN"
			}
			// Extract MTU
			if mtuMatch := regexp.MustCompile(`mtu\s+(\d+)`).FindStringSubmatch(line); mtuMatch != nil {
				current.MTU, _ = strconv.Atoi(mtuMatch[1])
			}
		} else if current != nil {
			// Type detection
			if strings.Contains(line, "link/ether") {
				if macMatch := regexp.MustCompile(`link/ether\s+([0-9a-f:]+)`).FindStringSubmatch(line); macMatch != nil {
					current.MAC = macMatch[1]
				}
			}
			// Detect type from line
			if strings.Contains(line, "veth") {
				current.Type = "veth"
			} else if strings.Contains(line, "vxlan") {
				current.Type = "vxlan"
			} else if strings.Contains(line, "gre") {
				current.Type = "gre"
			} else if strings.Contains(line, "wireguard") {
				current.Type = "wireguard"
			} else if strings.Contains(line, "bridge") {
				current.Type = "bridge"
			} else if current.Type == "" {
				current.Type = "ethernet"
			}
		}
	}

	if current != nil && current.Name != "lo" {
		interfaces = append(interfaces, *current)
	}

	// Get addresses for each interface
	for i := range interfaces {
		interfaces[i].Addresses = s.getInterfaceAddresses(namespace, interfaces[i].Name)
	}

	return interfaces, nil
}

// getInterfaceAddresses returns IP addresses for an interface in a namespace
func (s *NetnsService) getInterfaceAddresses(namespace, ifName string) []string {
	cmd := exec.Command("ip", "netns", "exec", namespace, "ip", "-o", "addr", "show", ifName)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var addresses []string
	re := regexp.MustCompile(`inet6?\s+([^\s]+)`)
	for _, match := range re.FindAllStringSubmatch(string(output), -1) {
		addresses = append(addresses, match[1])
	}
	return addresses
}

// MoveInterface moves an interface to a namespace
func (s *NetnsService) MoveInterface(input models.MoveInterfaceInput) error {
	if input.Interface == "" {
		return fmt.Errorf("interface name is required")
	}

	var cmd *exec.Cmd
	if input.Namespace == "" {
		// Move back to default namespace using pid 1 (init/default netns)
		cmd = exec.Command("ip", "link", "set", input.Interface, "netns", "1")
	} else {
		cmd = exec.Command("ip", "link", "set", input.Interface, "netns", input.Namespace)
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to move interface: %s", string(output))
	}

	return nil
}

// CreateVethPair creates a veth pair with one end in a namespace
func (s *NetnsService) CreateVethPair(input models.VethPairInput) error {
	if input.Name1 == "" || input.Name2 == "" {
		return fmt.Errorf("both interface names are required")
	}
	if input.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}

	// Create veth pair
	cmd := exec.Command("ip", "link", "add", input.Name1, "type", "veth", "peer", "name", input.Name2)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create veth pair: %s", string(output))
	}

	// Move second interface to namespace
	moveCmd := exec.Command("ip", "link", "set", input.Name2, "netns", input.Namespace)
	if output, err := moveCmd.CombinedOutput(); err != nil {
		// Cleanup on failure
		exec.Command("ip", "link", "delete", input.Name1).Run()
		return fmt.Errorf("failed to move interface to namespace: %s", string(output))
	}

	// Bring up both interfaces
	exec.Command("ip", "link", "set", input.Name1, "up").Run()
	exec.Command("ip", "netns", "exec", input.Namespace, "ip", "link", "set", input.Name2, "up").Run()

	// Add addresses if provided
	if input.Address1 != "" {
		exec.Command("ip", "addr", "add", input.Address1, "dev", input.Name1).Run()
	}
	if input.Address2 != "" {
		exec.Command("ip", "netns", "exec", input.Namespace, "ip", "addr", "add", input.Address2, "dev", input.Name2).Run()
	}

	return nil
}

// DeleteVethPair deletes a veth pair (deleting one end deletes both)
func (s *NetnsService) DeleteVethPair(name string) error {
	cmd := exec.Command("ip", "link", "delete", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete veth pair: %s", string(output))
	}
	return nil
}

// ExecInNamespace executes a command in a namespace
func (s *NetnsService) ExecInNamespace(namespace string, command string, args ...string) ([]byte, error) {
	fullArgs := append([]string{"netns", "exec", namespace, command}, args...)
	cmd := exec.Command("ip", fullArgs...)
	return cmd.CombinedOutput()
}

// hasPersistedConfig checks if namespace has saved configuration
func (s *NetnsService) hasPersistedConfig(name string) bool {
	configPath := filepath.Join(s.configDir, "netns", name)
	_, err := os.Stat(configPath)
	return err == nil
}

// SaveNamespaces saves namespace configurations including per-namespace configs
func (s *NetnsService) SaveNamespaces() error {
	netnsDir := filepath.Join(s.configDir, "netns")
	if err := os.MkdirAll(netnsDir, 0755); err != nil {
		return fmt.Errorf("failed to create netns config dir: %w", err)
	}

	namespaces, err := s.ListNamespaces()
	if err != nil {
		return err
	}

	// Save namespace list
	var nsLines []string
	for _, ns := range namespaces {
		nsLines = append(nsLines, ns.Name)
	}

	if err := os.WriteFile(filepath.Join(netnsDir, "namespaces.conf"), []byte(strings.Join(nsLines, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to save namespaces list: %w", err)
	}

	// Save per-namespace configurations
	for _, ns := range namespaces {
		if err := s.saveNamespaceConfig(ns.Name); err != nil {
			return fmt.Errorf("failed to save config for namespace %s: %w", ns.Name, err)
		}
	}

	// Save veth pairs configuration
	if err := s.saveVethPairs(); err != nil {
		return fmt.Errorf("failed to save veth pairs: %w", err)
	}

	return nil
}

// saveNamespaceConfig saves configuration for a specific namespace
func (s *NetnsService) saveNamespaceConfig(namespace string) error {
	nsDir := filepath.Join(s.configDir, "netns", namespace)
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		return err
	}

	// Save iptables rules
	if err := s.saveNamespaceIPTables(namespace, nsDir); err != nil {
		return fmt.Errorf("iptables: %w", err)
	}

	// Save routes
	if err := s.saveNamespaceRoutes(namespace, nsDir); err != nil {
		return fmt.Errorf("routes: %w", err)
	}

	// Save IP rules
	if err := s.saveNamespaceIPRules(namespace, nsDir); err != nil {
		return fmt.Errorf("ip rules: %w", err)
	}

	return nil
}

// saveNamespaceIPTables saves iptables rules for a namespace
func (s *NetnsService) saveNamespaceIPTables(namespace, nsDir string) error {
	cmd := exec.Command("ip", "netns", "exec", namespace, "iptables-save")
	output, err := cmd.Output()
	if err != nil {
		// Empty rules is not an error
		return nil
	}
	return os.WriteFile(filepath.Join(nsDir, "iptables.rules"), output, 0644)
}

// saveNamespaceRoutes saves routes for a namespace
func (s *NetnsService) saveNamespaceRoutes(namespace, nsDir string) error {
	cmd := exec.Command("ip", "netns", "exec", namespace, "ip", "route", "show")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	return os.WriteFile(filepath.Join(nsDir, "routes.conf"), output, 0644)
}

// saveNamespaceIPRules saves IP rules for a namespace
func (s *NetnsService) saveNamespaceIPRules(namespace, nsDir string) error {
	cmd := exec.Command("ip", "netns", "exec", namespace, "ip", "rule", "show")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	// Filter out default rules (priority 0 and 32766/32767)
	var rules []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip default rules
		if strings.HasPrefix(line, "0:") || strings.HasPrefix(line, "32766:") || strings.HasPrefix(line, "32767:") {
			continue
		}
		// Extract the rule without the priority prefix for easier restoration
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			rules = append(rules, strings.TrimSpace(parts[1]))
		}
	}

	if len(rules) == 0 {
		return nil
	}
	return os.WriteFile(filepath.Join(nsDir, "rules.conf"), []byte(strings.Join(rules, "\n")+"\n"), 0644)
}

// saveVethPairs saves veth pair configuration
func (s *NetnsService) saveVethPairs() error {
	// Find veth pairs by looking at interfaces with veth type
	cmd := exec.Command("ip", "-d", "link", "show", "type", "veth")
	output, err := cmd.Output()
	if err != nil {
		// No veth pairs
		return nil
	}

	var pairs []string
	lines := strings.Split(string(output), "\n")

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		// Match interface line: "2: veth0@if3: ..."
		match := regexp.MustCompile(`^\d+:\s+([^@]+)@`).FindStringSubmatch(line)
		if match == nil {
			continue
		}

		name := match[1]
		// Check if this veth has a peer in a namespace
		// This is complex because we need to track which pairs we've already recorded
		// For now, just save the veth name that exists in the default namespace
		pairs = append(pairs, name)
	}

	if len(pairs) == 0 {
		return nil
	}

	netnsDir := filepath.Join(s.configDir, "netns")
	return os.WriteFile(filepath.Join(netnsDir, "veth-pairs.conf"), []byte("# Veth interfaces in default namespace\n"+strings.Join(pairs, "\n")+"\n"), 0644)
}

// SetInterfaceState brings an interface up or down in a namespace
func (s *NetnsService) SetInterfaceState(namespace, ifName string, up bool) error {
	state := "down"
	if up {
		state = "up"
	}
	cmd := exec.Command("ip", "netns", "exec", namespace, "ip", "link", "set", ifName, state)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set interface %s: %s", state, string(output))
	}
	return nil
}

// RemoveInterface moves an interface from a namespace back to the default namespace
func (s *NetnsService) RemoveInterface(namespace, ifName string) error {
	cmd := exec.Command("ip", "netns", "exec", namespace, "ip", "link", "set", ifName, "netns", "1")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove interface: %s", string(output))
	}
	return nil
}

// RestoreNamespaces restores namespace configurations including per-namespace configs
func (s *NetnsService) RestoreNamespaces() error {
	netnsDir := filepath.Join(s.configDir, "netns")

	data, err := os.ReadFile(filepath.Join(netnsDir, "namespaces.conf"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var restoredNamespaces []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}

		// Check if namespace already exists
		if _, err := s.GetNamespace(name); err != nil {
			// Create if not exists
			if err := s.CreateNamespace(models.NetnsCreateInput{Name: name}); err != nil {
				continue
			}
		}
		restoredNamespaces = append(restoredNamespaces, name)
	}

	// Restore per-namespace configurations
	for _, name := range restoredNamespaces {
		s.restoreNamespaceConfig(name)
	}

	return nil
}

// restoreNamespaceConfig restores configuration for a specific namespace
func (s *NetnsService) restoreNamespaceConfig(namespace string) error {
	nsDir := filepath.Join(s.configDir, "netns", namespace)

	// Restore iptables rules
	s.restoreNamespaceIPTables(namespace, nsDir)

	// Restore routes
	s.restoreNamespaceRoutes(namespace, nsDir)

	// Restore IP rules
	s.restoreNamespaceIPRules(namespace, nsDir)

	return nil
}

// restoreNamespaceIPTables restores iptables rules for a namespace
func (s *NetnsService) restoreNamespaceIPTables(namespace, nsDir string) error {
	rulesFile := filepath.Join(nsDir, "iptables.rules")
	if _, err := os.Stat(rulesFile); os.IsNotExist(err) {
		return nil
	}

	cmd := exec.Command("ip", "netns", "exec", namespace, "iptables-restore")
	file, err := os.Open(rulesFile)
	if err != nil {
		return err
	}
	defer file.Close()
	cmd.Stdin = file

	return cmd.Run()
}

// restoreNamespaceRoutes restores routes for a namespace
func (s *NetnsService) restoreNamespaceRoutes(namespace, nsDir string) error {
	routesFile := filepath.Join(nsDir, "routes.conf")
	data, err := os.ReadFile(routesFile)
	if err != nil {
		return nil
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Skip default route if present (will be added by DHCP or config)
		if strings.HasPrefix(line, "default") {
			continue
		}
		args := append([]string{"netns", "exec", namespace, "ip", "route", "add"}, strings.Fields(line)...)
		exec.Command("ip", args...).Run()
	}

	return nil
}

// restoreNamespaceIPRules restores IP rules for a namespace
func (s *NetnsService) restoreNamespaceIPRules(namespace, nsDir string) error {
	rulesFile := filepath.Join(nsDir, "rules.conf")
	data, err := os.ReadFile(rulesFile)
	if err != nil {
		return nil
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		args := append([]string{"netns", "exec", namespace, "ip", "rule", "add"}, strings.Fields(line)...)
		exec.Command("ip", args...).Run()
	}

	return nil
}
