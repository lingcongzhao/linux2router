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

type IPRouteService struct {
	configDir string
}

func NewIPRouteService(configDir string) *IPRouteService {
	return &IPRouteService{configDir: configDir}
}

func (s *IPRouteService) ListRoutes(table string) ([]models.Route, error) {
	args := []string{"route", "show"}
	if table != "" && table != "main" {
		args = append(args, "table", table)
	}

	cmd := exec.Command("ip", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list routes: %w", err)
	}

	return s.parseRouteOutput(string(output), table)
}

func (s *IPRouteService) ListAllRoutes() ([]models.Route, error) {
	cmd := exec.Command("ip", "route", "show", "table", "all")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list all routes: %w", err)
	}

	return s.parseRouteOutput(string(output), "")
}

func (s *IPRouteService) parseRouteOutput(output, defaultTable string) ([]models.Route, error) {
	var routes []models.Route
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		route := s.parseRouteLine(line, defaultTable)
		if route != nil {
			routes = append(routes, *route)
		}
	}

	return routes, nil
}

func (s *IPRouteService) parseRouteLine(line, defaultTable string) *models.Route {
	route := &models.Route{
		Table: defaultTable,
	}

	parts := strings.Fields(line)
	if len(parts) < 1 {
		return nil
	}

	// First element is usually destination or "default"
	route.Destination = parts[0]

	// Parse key-value pairs
	for i := 1; i < len(parts); i++ {
		switch parts[i] {
		case "via":
			if i+1 < len(parts) {
				route.Gateway = parts[i+1]
				i++
			}
		case "dev":
			if i+1 < len(parts) {
				route.Interface = parts[i+1]
				i++
			}
		case "proto":
			if i+1 < len(parts) {
				route.Protocol = parts[i+1]
				i++
			}
		case "scope":
			if i+1 < len(parts) {
				route.Scope = parts[i+1]
				i++
			}
		case "src":
			if i+1 < len(parts) {
				route.Source = parts[i+1]
				i++
			}
		case "metric":
			if i+1 < len(parts) {
				route.Metric, _ = strconv.Atoi(parts[i+1])
				i++
			}
		case "table":
			if i+1 < len(parts) {
				route.Table = parts[i+1]
				i++
			}
		}
	}

	// Handle route type
	if strings.HasPrefix(route.Destination, "broadcast") ||
		strings.HasPrefix(route.Destination, "local") ||
		strings.HasPrefix(route.Destination, "unreachable") {
		typeParts := strings.SplitN(route.Destination, " ", 2)
		route.Type = typeParts[0]
		if len(typeParts) > 1 {
			route.Destination = typeParts[1]
		}
	}

	return route
}

func (s *IPRouteService) AddRoute(input models.RouteInput) error {
	args := []string{"route", "add"}

	if input.Destination == "" {
		return fmt.Errorf("destination is required")
	}
	args = append(args, input.Destination)

	if input.Gateway != "" {
		args = append(args, "via", input.Gateway)
	}

	if input.Interface != "" {
		args = append(args, "dev", input.Interface)
	}

	if input.Metric > 0 {
		args = append(args, "metric", strconv.Itoa(input.Metric))
	}

	if input.Table != "" && input.Table != "main" {
		args = append(args, "table", input.Table)
	}

	cmd := exec.Command("ip", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add route: %s", string(output))
	}

	return nil
}

func (s *IPRouteService) DeleteRoute(destination, gateway, iface, table string) error {
	args := []string{"route", "del", destination}

	if gateway != "" {
		args = append(args, "via", gateway)
	}

	if iface != "" {
		args = append(args, "dev", iface)
	}

	if table != "" && table != "main" {
		args = append(args, "table", table)
	}

	cmd := exec.Command("ip", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete route: %s", string(output))
	}

	return nil
}

func (s *IPRouteService) GetRoutingTables() ([]models.RoutingTable, error) {
	// Read /etc/iproute2/rt_tables
	file, err := os.Open("/etc/iproute2/rt_tables")
	if err != nil {
		// Return default tables if file doesn't exist
		return []models.RoutingTable{
			{ID: 255, Name: "local"},
			{ID: 254, Name: "main"},
			{ID: 253, Name: "default"},
		}, nil
	}
	defer file.Close()

	var tables []models.RoutingTable
	scanner := bufio.NewScanner(file)
	re := regexp.MustCompile(`^\s*(\d+)\s+(\S+)`)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "#") || strings.TrimSpace(line) == "" {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if matches != nil {
			id, _ := strconv.Atoi(matches[1])
			tables = append(tables, models.RoutingTable{
				ID:   id,
				Name: matches[2],
			})
		}
	}

	return tables, nil
}

func (s *IPRouteService) CreateRoutingTable(id int, name string) error {
	// Validate ID range (1-252, 0 and 253-255 are reserved)
	if id < 1 || id > 252 {
		return fmt.Errorf("table ID must be between 1 and 252")
	}

	// Validate name
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("table name is required")
	}
	if strings.ContainsAny(name, " \t\n") {
		return fmt.Errorf("table name cannot contain whitespace")
	}

	// Check if table already exists
	tables, err := s.GetRoutingTables()
	if err != nil {
		return err
	}

	for _, t := range tables {
		if t.ID == id {
			return fmt.Errorf("table ID %d already exists", id)
		}
		if t.Name == name {
			return fmt.Errorf("table name '%s' already exists", name)
		}
	}

	// Append to /etc/iproute2/rt_tables
	f, err := os.OpenFile("/etc/iproute2/rt_tables", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open rt_tables: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(fmt.Sprintf("%d\t%s\n", id, name))
	if err != nil {
		return fmt.Errorf("failed to write to rt_tables: %w", err)
	}

	return nil
}

func (s *IPRouteService) DeleteRoutingTable(name string) error {
	// Reserved tables that cannot be deleted
	reserved := map[string]bool{
		"local":   true,
		"main":    true,
		"default": true,
		"unspec":  true,
	}

	if reserved[name] {
		return fmt.Errorf("cannot delete reserved table '%s'", name)
	}

	// Read current file
	data, err := os.ReadFile("/etc/iproute2/rt_tables")
	if err != nil {
		return fmt.Errorf("failed to read rt_tables: %w", err)
	}

	// Filter out the table to delete
	var newLines []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	re := regexp.MustCompile(`^\s*(\d+)\s+(\S+)`)
	found := false

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if matches != nil && matches[2] == name {
			found = true
			continue // Skip this line
		}
		newLines = append(newLines, line)
	}

	if !found {
		return fmt.Errorf("table '%s' not found", name)
	}

	// Write back
	if err := os.WriteFile("/etc/iproute2/rt_tables", []byte(strings.Join(newLines, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write rt_tables: %w", err)
	}

	// Flush routes in the deleted table (optional, best effort)
	exec.Command("ip", "route", "flush", "table", name).Run()

	return nil
}

func (s *IPRouteService) GetNextAvailableTableID() int {
	tables, _ := s.GetRoutingTables()
	usedIDs := make(map[int]bool)
	for _, t := range tables {
		usedIDs[t.ID] = true
	}

	// Find first available ID between 1 and 252
	for i := 100; i <= 252; i++ {
		if !usedIDs[i] {
			return i
		}
	}
	// Try lower range
	for i := 1; i < 100; i++ {
		if !usedIDs[i] {
			return i
		}
	}
	return 0 // No available ID
}

func (s *IPRouteService) FlushTable(table string) error {
	args := []string{"route", "flush"}
	if table != "" {
		args = append(args, "table", table)
	}

	cmd := exec.Command("ip", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to flush routes: %s", string(output))
	}

	return nil
}

func (s *IPRouteService) SaveRoutes() error {
	// Get all routes
	routes, err := s.ListAllRoutes()
	if err != nil {
		return err
	}

	// Ensure routes directory exists
	routesDir := filepath.Join(s.configDir, "routes")
	if err := os.MkdirAll(routesDir, 0755); err != nil {
		return fmt.Errorf("failed to create routes directory: %w", err)
	}

	// Group by table
	tableRoutes := make(map[string][]string)
	for _, route := range routes {
		table := route.Table
		if table == "" {
			table = "main"
		}

		// Skip the "local" table entirely (contains auto-generated local/broadcast routes)
		if table == "local" {
			continue
		}

		// Skip local and broadcast route types
		if route.Type == "local" || route.Type == "broadcast" {
			continue
		}

		// Skip kernel-generated interface routes (proto kernel with scope link/host)
		// but only for the main table - custom tables should save all routes
		if table == "main" && route.Protocol == "kernel" && (route.Scope == "link" || route.Scope == "host") {
			continue
		}

		// Build route command
		cmd := route.Destination
		if route.Gateway != "" {
			cmd += " via " + route.Gateway
		}
		if route.Interface != "" {
			cmd += " dev " + route.Interface
		}
		if route.Metric > 0 {
			cmd += " metric " + strconv.Itoa(route.Metric)
		}

		tableRoutes[table] = append(tableRoutes[table], cmd)
	}

	// Save to files
	for table, cmds := range tableRoutes {
		savePath := filepath.Join(routesDir, table+".conf")
		content := strings.Join(cmds, "\n")
		if err := os.WriteFile(savePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to save routes for table %s: %w", table, err)
		}
	}

	return nil
}

func (s *IPRouteService) RestoreRoutes() error {
	routesDir := filepath.Join(s.configDir, "routes")
	files, err := os.ReadDir(routesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".conf") {
			continue
		}

		table := strings.TrimSuffix(file.Name(), ".conf")
		data, err := os.ReadFile(filepath.Join(routesDir, file.Name()))
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			args := []string{"route", "add"}
			args = append(args, strings.Fields(line)...)
			if table != "main" {
				args = append(args, "table", table)
			}

			exec.Command("ip", args...).Run()
		}
	}

	return nil
}

// GetIPForwarding returns the current state of IPv4 forwarding
func (s *IPRouteService) GetIPForwarding() (bool, error) {
	data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil {
		return false, fmt.Errorf("failed to read ip_forward: %w", err)
	}
	return strings.TrimSpace(string(data)) == "1", nil
}

// SetIPForwarding enables or disables IPv4 forwarding
func (s *IPRouteService) SetIPForwarding(enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}

	if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte(value), 0644); err != nil {
		return fmt.Errorf("failed to set ip_forward: %w", err)
	}

	return nil
}

// SaveIPForwarding saves the current IP forwarding state to config file
func (s *IPRouteService) SaveIPForwarding() error {
	enabled, err := s.GetIPForwarding()
	if err != nil {
		return err
	}

	// Ensure directory exists
	sysDir := filepath.Join(s.configDir, "sysctl")
	if err := os.MkdirAll(sysDir, 0755); err != nil {
		return fmt.Errorf("failed to create sysctl config dir: %w", err)
	}

	value := "0"
	if enabled {
		value = "1"
	}

	savePath := filepath.Join(sysDir, "ip_forward.conf")
	content := fmt.Sprintf("# IP Forwarding configuration\nnet.ipv4.ip_forward=%s\n", value)
	if err := os.WriteFile(savePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to save ip_forward config: %w", err)
	}

	return nil
}

// RestoreIPForwarding restores IP forwarding state from config file
func (s *IPRouteService) RestoreIPForwarding() error {
	savePath := filepath.Join(s.configDir, "sysctl", "ip_forward.conf")
	data, err := os.ReadFile(savePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No saved config
		}
		return fmt.Errorf("failed to read ip_forward config: %w", err)
	}

	// Parse the config file
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if strings.HasPrefix(line, "net.ipv4.ip_forward=") {
			value := strings.TrimPrefix(line, "net.ipv4.ip_forward=")
			enabled := value == "1"
			return s.SetIPForwarding(enabled)
		}
	}

	return nil
}

// ListRoutesInNamespace lists routes in a specific network namespace
func (s *IPRouteService) ListRoutesInNamespace(table, namespace string) ([]models.Route, error) {
	args := []string{"route", "show"}
	if table != "" && table != "main" {
		args = append(args, "table", table)
	}

	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", args...)
	} else {
		nsArgs := append([]string{"netns", "exec", namespace, "ip"}, args...)
		cmd = exec.Command("ip", nsArgs...)
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list routes: %w", err)
	}

	return s.parseRouteOutput(string(output), table)
}

// AddRouteInNamespace adds a route in a specific network namespace
func (s *IPRouteService) AddRouteInNamespace(input models.RouteInput, namespace string) error {
	args := []string{"route", "add"}

	if input.Destination == "" {
		return fmt.Errorf("destination is required")
	}
	args = append(args, input.Destination)

	if input.Gateway != "" {
		args = append(args, "via", input.Gateway)
	}

	if input.Interface != "" {
		args = append(args, "dev", input.Interface)
	}

	if input.Metric > 0 {
		args = append(args, "metric", strconv.Itoa(input.Metric))
	}

	if input.Table != "" && input.Table != "main" {
		args = append(args, "table", input.Table)
	}

	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", args...)
	} else {
		nsArgs := append([]string{"netns", "exec", namespace, "ip"}, args...)
		cmd = exec.Command("ip", nsArgs...)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add route: %s", string(output))
	}

	return nil
}

// DeleteRouteInNamespace deletes a route in a specific network namespace
func (s *IPRouteService) DeleteRouteInNamespace(input models.RouteInput, namespace string) error {
	args := []string{"route", "del", input.Destination}

	if input.Gateway != "" {
		args = append(args, "via", input.Gateway)
	}

	if input.Interface != "" {
		args = append(args, "dev", input.Interface)
	}

	if input.Table != "" && input.Table != "main" {
		args = append(args, "table", input.Table)
	}

	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", args...)
	} else {
		nsArgs := append([]string{"netns", "exec", namespace, "ip"}, args...)
		cmd = exec.Command("ip", nsArgs...)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete route: %s", string(output))
	}

	return nil
}

// GetRoutingTablesForNamespace returns routing tables for a specific namespace
// Tables are stored in configs/netns/{namespace}/rt_tables
func (s *IPRouteService) GetRoutingTablesForNamespace(namespace string) ([]models.RoutingTable, error) {
	// Default tables that always exist
	defaultTables := []models.RoutingTable{
		{ID: 255, Name: "local"},
		{ID: 254, Name: "main"},
		{ID: 253, Name: "default"},
	}

	// Read namespace-specific rt_tables file
	rtTablesPath := filepath.Join(s.configDir, "netns", namespace, "rt_tables")
	file, err := os.Open(rtTablesPath)
	if err != nil {
		// Return default tables if file doesn't exist
		return defaultTables, nil
	}
	defer file.Close()

	var tables []models.RoutingTable
	scanner := bufio.NewScanner(file)
	re := regexp.MustCompile(`^\s*(\d+)\s+(\S+)`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		id, _ := strconv.Atoi(matches[1])
		name := matches[2]

		// Skip reserved tables (they're in defaultTables)
		if id == 0 || id >= 253 {
			continue
		}

		tables = append(tables, models.RoutingTable{
			ID:   id,
			Name: name,
		})
	}

	// Append default tables
	tables = append(tables, defaultTables...)
	return tables, nil
}

// CreateRoutingTableForNamespace creates a routing table for a specific namespace
func (s *IPRouteService) CreateRoutingTableForNamespace(namespace string, id int, name string) error {
	// Validate ID range (1-252, 0 and 253-255 are reserved)
	if id < 1 || id > 252 {
		return fmt.Errorf("table ID must be between 1 and 252")
	}

	// Validate name
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("table name is required")
	}
	if strings.ContainsAny(name, " \t\n") {
		return fmt.Errorf("table name cannot contain whitespace")
	}

	// Ensure namespace directory exists
	nsDir := filepath.Join(s.configDir, "netns", namespace)
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		return fmt.Errorf("failed to create namespace config directory: %w", err)
	}

	rtTablesPath := filepath.Join(nsDir, "rt_tables")

	// Check if table already exists
	tables, _ := s.GetRoutingTablesForNamespace(namespace)
	for _, t := range tables {
		if t.ID == id {
			return fmt.Errorf("table with ID %d already exists", id)
		}
		if t.Name == name {
			return fmt.Errorf("table with name '%s' already exists", name)
		}
	}

	// Append to file
	f, err := os.OpenFile(rtTablesPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open rt_tables: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "%d\t%s\n", id, name); err != nil {
		return fmt.Errorf("failed to write to rt_tables: %w", err)
	}

	return nil
}

// DeleteRoutingTableForNamespace deletes a routing table from a specific namespace
func (s *IPRouteService) DeleteRoutingTableForNamespace(namespace, name string) error {
	// Reserved tables that cannot be deleted
	reserved := map[string]bool{
		"local":   true,
		"main":    true,
		"default": true,
		"unspec":  true,
	}

	if reserved[name] {
		return fmt.Errorf("cannot delete reserved table '%s'", name)
	}

	rtTablesPath := filepath.Join(s.configDir, "netns", namespace, "rt_tables")

	// Read current file
	data, err := os.ReadFile(rtTablesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("table '%s' not found", name)
		}
		return fmt.Errorf("failed to read rt_tables: %w", err)
	}

	// Filter out the table to delete
	var newLines []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	re := regexp.MustCompile(`^\s*(\d+)\s+(\S+)`)
	found := false

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if matches != nil && matches[2] == name {
			found = true
			continue // Skip this line
		}
		newLines = append(newLines, line)
	}

	if !found {
		return fmt.Errorf("table '%s' not found", name)
	}

	// Write back
	content := strings.Join(newLines, "\n")
	if len(newLines) > 0 {
		content += "\n"
	}
	if err := os.WriteFile(rtTablesPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write rt_tables: %w", err)
	}

	// Flush routes in the deleted table (best effort)
	exec.Command("ip", "netns", "exec", namespace, "ip", "route", "flush", "table", name).Run()

	return nil
}

// GetNextAvailableTableIDForNamespace finds the next available table ID for a namespace
func (s *IPRouteService) GetNextAvailableTableIDForNamespace(namespace string) int {
	tables, _ := s.GetRoutingTablesForNamespace(namespace)
	usedIDs := make(map[int]bool)
	for _, t := range tables {
		usedIDs[t.ID] = true
	}

	// Find first available ID between 100 and 252
	for i := 100; i <= 252; i++ {
		if !usedIDs[i] {
			return i
		}
	}
	// Try lower range
	for i := 1; i < 100; i++ {
		if !usedIDs[i] {
			return i
		}
	}
	return 0 // No available ID
}
