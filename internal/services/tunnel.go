package services

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"linuxtorouter/internal/models"
)

type TunnelService struct {
	configDir string
}

func NewTunnelService(configDir string) *TunnelService {
	return &TunnelService{configDir: configDir}
}

// ==================== GRE Tunnels ====================

func (s *TunnelService) ListGRETunnels() ([]models.GRETunnel, error) {
	// Get GRE tunnels using ip tunnel show
	cmd := exec.Command("ip", "tunnel", "show")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list GRE tunnels: %w", err)
	}

	var tunnels []models.GRETunnel
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		tunnel := s.parseGRETunnelLine(line)
		if tunnel != nil {
			// Get interface state
			state, _ := s.getInterfaceState(tunnel.Name)
			tunnel.State = state
			// Get interface addresses
			tunnel.Addresses = s.getInterfaceAddresses(tunnel.Name)
			tunnels = append(tunnels, *tunnel)
		}
	}

	return tunnels, nil
}

func (s *TunnelService) parseGRETunnelLine(line string) *models.GRETunnel {
	// Example: gre0: gre/ip remote any local any ttl inherit nopmtudisc
	// Example: mytunnel: gre/ip remote 192.168.1.1 local 192.168.1.2 ttl 64
	if !strings.Contains(line, "gre/ip") {
		return nil
	}

	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil
	}

	name := strings.TrimSuffix(parts[0], ":")
	// Skip default gre0 interface
	if name == "gre0" || name == "ip6gre0" {
		return nil
	}

	tunnel := &models.GRETunnel{
		Name: name,
		Mode: "gre",
	}

	for i := 1; i < len(parts); i++ {
		switch parts[i] {
		case "remote":
			if i+1 < len(parts) {
				tunnel.Remote = parts[i+1]
				i++
			}
		case "local":
			if i+1 < len(parts) {
				tunnel.Local = parts[i+1]
				i++
			}
		case "ttl":
			if i+1 < len(parts) {
				tunnel.TTL, _ = strconv.Atoi(parts[i+1])
				i++
			}
		case "key":
			if i+1 < len(parts) {
				tunnel.Key = parts[i+1]
				i++
			}
		}
	}

	return tunnel
}

func (s *TunnelService) CreateGRETunnel(input models.GRETunnelInput) error {
	if input.Name == "" {
		return fmt.Errorf("tunnel name is required")
	}
	if input.Local == "" || input.Remote == "" {
		return fmt.Errorf("local and remote addresses are required")
	}

	args := []string{"tunnel", "add", input.Name, "mode", "gre",
		"local", input.Local, "remote", input.Remote}

	if input.Key != "" {
		args = append(args, "key", input.Key)
	}
	if input.TTL > 0 {
		args = append(args, "ttl", strconv.Itoa(input.TTL))
	}

	cmd := exec.Command("ip", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create GRE tunnel: %s", string(output))
	}

	// Bring the interface up
	if err := s.setInterfaceUp(input.Name); err != nil {
		return fmt.Errorf("tunnel created but failed to bring up: %w", err)
	}

	// Configure IP address if provided
	if input.Address != "" {
		if err := s.addInterfaceAddress(input.Name, input.Address); err != nil {
			return fmt.Errorf("tunnel created but failed to add address: %w", err)
		}
	}

	return nil
}

func (s *TunnelService) DeleteGRETunnel(name string) error {
	cmd := exec.Command("ip", "tunnel", "del", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete GRE tunnel: %s", string(output))
	}
	return nil
}

// ==================== VXLAN Tunnels ====================

func (s *TunnelService) ListVXLANTunnels() ([]models.VXLANTunnel, error) {
	cmd := exec.Command("ip", "-d", "link", "show", "type", "vxlan")
	output, err := cmd.Output()
	if err != nil {
		// No VXLAN interfaces is not an error
		return []models.VXLANTunnel{}, nil
	}

	var tunnels []models.VXLANTunnel
	lines := strings.Split(string(output), "\n")

	var currentTunnel *models.VXLANTunnel
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Interface line: "4: vxlan100: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1450..."
		if match := regexp.MustCompile(`^\d+:\s+(\S+):`).FindStringSubmatch(line); match != nil {
			if currentTunnel != nil {
				tunnels = append(tunnels, *currentTunnel)
			}
			currentTunnel = &models.VXLANTunnel{
				Name:    strings.TrimSuffix(match[1], "@NONE"),
				DstPort: 4789, // default
			}
			if strings.Contains(line, "UP") {
				currentTunnel.State = "UP"
			} else {
				currentTunnel.State = "DOWN"
			}
			// Extract MTU
			if mtuMatch := regexp.MustCompile(`mtu\s+(\d+)`).FindStringSubmatch(line); mtuMatch != nil {
				currentTunnel.MTU, _ = strconv.Atoi(mtuMatch[1])
			}
		} else if currentTunnel != nil {
			if strings.Contains(line, "vxlan") {
				// VXLAN details line
				s.parseVXLANDetails(line, currentTunnel)
			} else if strings.Contains(line, "link/ether") {
				// MAC address line: "link/ether 5a:3b:2c:1d:0e:ff brd ff:ff:ff:ff:ff:ff"
				if macMatch := regexp.MustCompile(`link/ether\s+([0-9a-f:]+)`).FindStringSubmatch(line); macMatch != nil {
					currentTunnel.MAC = macMatch[1]
				}
			}
		}
	}

	if currentTunnel != nil {
		currentTunnel.Addresses = s.getInterfaceAddresses(currentTunnel.Name)
		tunnels = append(tunnels, *currentTunnel)
	}

	// Fetch addresses for all tunnels
	for i := range tunnels {
		if tunnels[i].Addresses == nil {
			tunnels[i].Addresses = s.getInterfaceAddresses(tunnels[i].Name)
		}
	}

	return tunnels, nil
}

func (s *TunnelService) parseVXLANDetails(line string, tunnel *models.VXLANTunnel) {
	// Example: vxlan id 100 local 192.168.1.1 dev eth0 srcport 0 0 dstport 4789
	parts := strings.Fields(line)
	for i := 0; i < len(parts); i++ {
		switch parts[i] {
		case "id":
			if i+1 < len(parts) {
				tunnel.VNI, _ = strconv.Atoi(parts[i+1])
				i++
			}
		case "local":
			if i+1 < len(parts) {
				tunnel.Local = parts[i+1]
				i++
			}
		case "remote":
			if i+1 < len(parts) {
				tunnel.Remote = parts[i+1]
				i++
			}
		case "group":
			if i+1 < len(parts) {
				tunnel.Group = parts[i+1]
				i++
			}
		case "dev":
			if i+1 < len(parts) {
				tunnel.Dev = parts[i+1]
				i++
			}
		case "dstport":
			if i+1 < len(parts) {
				tunnel.DstPort, _ = strconv.Atoi(parts[i+1])
				i++
			}
		}
	}
}

func (s *TunnelService) CreateVXLANTunnel(input models.VXLANTunnelInput) error {
	if input.Name == "" {
		return fmt.Errorf("tunnel name is required")
	}
	if input.VNI <= 0 {
		return fmt.Errorf("VNI is required and must be positive")
	}

	args := []string{"link", "add", input.Name, "type", "vxlan", "id", strconv.Itoa(input.VNI)}

	if input.Local != "" {
		args = append(args, "local", input.Local)
	}
	if input.Remote != "" {
		args = append(args, "remote", input.Remote)
	}
	if input.Group != "" {
		args = append(args, "group", input.Group)
	}
	if input.Dev != "" {
		args = append(args, "dev", input.Dev)
	}
	if input.DstPort > 0 {
		args = append(args, "dstport", strconv.Itoa(input.DstPort))
	} else {
		args = append(args, "dstport", "4789")
	}

	cmd := exec.Command("ip", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create VXLAN tunnel: %s", string(output))
	}

	// Set MAC address (generate random one if not provided)
	mac := input.MAC
	if mac == "" {
		mac = s.generateRandomMAC()
	}
	macCmd := exec.Command("ip", "link", "set", input.Name, "address", mac)
	if output, err := macCmd.CombinedOutput(); err != nil {
		// Don't fail, just log warning
		fmt.Printf("Warning: failed to set MAC address: %s\n", string(output))
	}

	// Bring the interface up
	if err := s.setInterfaceUp(input.Name); err != nil {
		return fmt.Errorf("tunnel created but failed to bring up: %w", err)
	}

	// Configure IP address if provided
	if input.Address != "" {
		if err := s.addInterfaceAddress(input.Name, input.Address); err != nil {
			return fmt.Errorf("tunnel created but failed to add address: %w", err)
		}
	}

	return nil
}

// generateRandomMAC generates a random locally administered unicast MAC address
func (s *TunnelService) generateRandomMAC() string {
	mac := make([]byte, 6)
	rand.Read(mac)
	// Set locally administered bit (bit 1 of first byte) and clear multicast bit (bit 0)
	mac[0] = (mac[0] | 0x02) & 0xfe
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

func (s *TunnelService) DeleteVXLANTunnel(name string) error {
	cmd := exec.Command("ip", "link", "del", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete VXLAN tunnel: %s", string(output))
	}
	return nil
}

// ==================== WireGuard Tunnels ====================

func (s *TunnelService) ListWireGuardTunnels() ([]models.WireGuardTunnel, error) {
	// First get list of wireguard interfaces
	cmd := exec.Command("ip", "link", "show", "type", "wireguard")
	output, err := cmd.Output()
	if err != nil {
		return []models.WireGuardTunnel{}, nil
	}

	var tunnels []models.WireGuardTunnel
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if match := regexp.MustCompile(`^\d+:\s+(\S+):`).FindStringSubmatch(line); match != nil {
			name := strings.TrimSuffix(match[1], "@NONE")
			tunnel := models.WireGuardTunnel{
				Name: name,
			}
			if strings.Contains(line, "UP") {
				tunnel.State = "UP"
			} else {
				tunnel.State = "DOWN"
			}

			// Get WireGuard details
			s.getWireGuardDetails(&tunnel)

			// Get addresses
			tunnel.Addresses = s.getInterfaceAddresses(name)

			tunnels = append(tunnels, tunnel)
		}
	}

	return tunnels, nil
}

func (s *TunnelService) getWireGuardDetails(tunnel *models.WireGuardTunnel) {
	cmd := exec.Command("wg", "show", tunnel.Name)
	output, err := cmd.Output()
	if err != nil {
		return
	}

	var currentPeer *models.WireGuardPeer
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "public key:") {
			tunnel.PublicKey = strings.TrimSpace(strings.TrimPrefix(line, "public key:"))
		} else if strings.HasPrefix(line, "listening port:") {
			port := strings.TrimSpace(strings.TrimPrefix(line, "listening port:"))
			tunnel.ListenPort, _ = strconv.Atoi(port)
		} else if strings.HasPrefix(line, "fwmark:") {
			tunnel.FwMark = strings.TrimSpace(strings.TrimPrefix(line, "fwmark:"))
		} else if strings.HasPrefix(line, "peer:") {
			if currentPeer != nil {
				tunnel.Peers = append(tunnel.Peers, *currentPeer)
			}
			currentPeer = &models.WireGuardPeer{
				PublicKey: strings.TrimSpace(strings.TrimPrefix(line, "peer:")),
			}
		} else if currentPeer != nil {
			if strings.HasPrefix(line, "endpoint:") {
				currentPeer.Endpoint = strings.TrimSpace(strings.TrimPrefix(line, "endpoint:"))
			} else if strings.HasPrefix(line, "allowed ips:") {
				ips := strings.TrimSpace(strings.TrimPrefix(line, "allowed ips:"))
				currentPeer.AllowedIPs = strings.Split(ips, ", ")
			} else if strings.HasPrefix(line, "latest handshake:") {
				currentPeer.LatestHandshake = strings.TrimSpace(strings.TrimPrefix(line, "latest handshake:"))
			} else if strings.HasPrefix(line, "transfer:") {
				s.parseWireGuardTransfer(line, currentPeer)
			} else if strings.HasPrefix(line, "persistent keepalive:") {
				ka := strings.TrimSpace(strings.TrimPrefix(line, "persistent keepalive:"))
				ka = strings.TrimSuffix(ka, " seconds")
				currentPeer.PersistentKeepalive, _ = strconv.Atoi(ka)
			}
		}
	}

	if currentPeer != nil {
		tunnel.Peers = append(tunnel.Peers, *currentPeer)
	}
}

func (s *TunnelService) parseWireGuardTransfer(line string, peer *models.WireGuardPeer) {
	// Example: transfer: 1.23 KiB received, 4.56 MiB sent
	line = strings.TrimPrefix(line, "transfer:")
	parts := strings.Split(line, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "received") {
			peer.TransferRx = s.parseDataSize(strings.TrimSuffix(part, " received"))
		} else if strings.Contains(part, "sent") {
			peer.TransferTx = s.parseDataSize(strings.TrimSuffix(part, " sent"))
		}
	}
}

func (s *TunnelService) parseDataSize(size string) uint64 {
	size = strings.TrimSpace(size)
	parts := strings.Fields(size)
	if len(parts) != 2 {
		return 0
	}
	value, _ := strconv.ParseFloat(parts[0], 64)
	unit := parts[1]
	switch unit {
	case "B":
		return uint64(value)
	case "KiB":
		return uint64(value * 1024)
	case "MiB":
		return uint64(value * 1024 * 1024)
	case "GiB":
		return uint64(value * 1024 * 1024 * 1024)
	}
	return 0
}

func (s *TunnelService) GetWireGuardTunnel(name string) (*models.WireGuardTunnel, error) {
	tunnels, err := s.ListWireGuardTunnels()
	if err != nil {
		return nil, err
	}
	for _, t := range tunnels {
		if t.Name == name {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("tunnel not found: %s", name)
}

func (s *TunnelService) CreateWireGuardTunnel(input models.WireGuardTunnelInput) error {
	if input.Name == "" {
		return fmt.Errorf("tunnel name is required")
	}

	// Create the interface
	cmd := exec.Command("ip", "link", "add", input.Name, "type", "wireguard")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create WireGuard interface: %s", string(output))
	}

	// Generate or set private key
	privateKey := input.PrivateKey
	if privateKey == "" {
		genCmd := exec.Command("wg", "genkey")
		keyOutput, err := genCmd.Output()
		if err != nil {
			s.DeleteWireGuardTunnel(input.Name)
			return fmt.Errorf("failed to generate private key: %w", err)
		}
		privateKey = strings.TrimSpace(string(keyOutput))
	}

	// Set the private key
	setCmd := exec.Command("wg", "set", input.Name, "private-key", "/dev/stdin")
	setCmd.Stdin = strings.NewReader(privateKey)
	if output, err := setCmd.CombinedOutput(); err != nil {
		s.DeleteWireGuardTunnel(input.Name)
		return fmt.Errorf("failed to set private key: %s", string(output))
	}

	// Set listen port if specified
	if input.ListenPort > 0 {
		setCmd := exec.Command("wg", "set", input.Name, "listen-port", strconv.Itoa(input.ListenPort))
		if output, err := setCmd.CombinedOutput(); err != nil {
			s.DeleteWireGuardTunnel(input.Name)
			return fmt.Errorf("failed to set listen port: %s", string(output))
		}
	}

	// Add address if specified
	if input.Address != "" {
		addrCmd := exec.Command("ip", "addr", "add", input.Address, "dev", input.Name)
		if output, err := addrCmd.CombinedOutput(); err != nil {
			// Don't fail, just log
			fmt.Printf("Warning: failed to add address: %s\n", string(output))
		}
	}

	// Bring the interface up
	if err := s.setInterfaceUp(input.Name); err != nil {
		return fmt.Errorf("tunnel created but failed to bring up: %w", err)
	}

	return nil
}

func (s *TunnelService) DeleteWireGuardTunnel(name string) error {
	cmd := exec.Command("ip", "link", "del", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete WireGuard tunnel: %s", string(output))
	}
	return nil
}

func (s *TunnelService) AddWireGuardPeer(input models.WireGuardPeerInput) error {
	if input.Interface == "" || input.PublicKey == "" {
		return fmt.Errorf("interface and public key are required")
	}

	args := []string{"set", input.Interface, "peer", input.PublicKey}

	if input.Endpoint != "" {
		args = append(args, "endpoint", input.Endpoint)
	}
	if input.AllowedIPs != "" {
		args = append(args, "allowed-ips", input.AllowedIPs)
	}
	if input.PersistentKeepalive > 0 {
		args = append(args, "persistent-keepalive", strconv.Itoa(input.PersistentKeepalive))
	}

	cmd := exec.Command("wg", args...)
	if input.PresharedKey != "" {
		// Need to use preshared-key with /dev/stdin
		args = append(args, "preshared-key", "/dev/stdin")
		cmd = exec.Command("wg", args...)
		cmd.Stdin = strings.NewReader(input.PresharedKey)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add peer: %s", string(output))
	}

	return nil
}

func (s *TunnelService) RemoveWireGuardPeer(interfaceName, publicKey string) error {
	cmd := exec.Command("wg", "set", interfaceName, "peer", publicKey, "remove")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove peer: %s", string(output))
	}
	return nil
}

func (s *TunnelService) UpdateWireGuardInterface(name string, listenPort int, address string) error {
	// Update listen port if specified
	if listenPort > 0 {
		cmd := exec.Command("wg", "set", name, "listen-port", strconv.Itoa(listenPort))
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to set listen port: %s", string(output))
		}
	}

	// Handle address changes
	if address != "" {
		// First flush existing addresses
		flushCmd := exec.Command("ip", "addr", "flush", "dev", name)
		flushCmd.CombinedOutput() // Ignore errors

		// Add new address
		addCmd := exec.Command("ip", "addr", "add", address, "dev", name)
		if output, err := addCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to add address: %s", string(output))
		}
	}

	return nil
}

func (s *TunnelService) UpdateWireGuardPeer(input models.WireGuardPeerInput) error {
	if input.Interface == "" || input.PublicKey == "" {
		return fmt.Errorf("interface and public key are required")
	}

	args := []string{"set", input.Interface, "peer", input.PublicKey}

	if input.Endpoint != "" {
		args = append(args, "endpoint", input.Endpoint)
	}
	if input.AllowedIPs != "" {
		args = append(args, "allowed-ips", input.AllowedIPs)
	}
	if input.PersistentKeepalive >= 0 {
		args = append(args, "persistent-keepalive", strconv.Itoa(input.PersistentKeepalive))
	}

	cmd := exec.Command("wg", args...)
	if input.PresharedKey != "" {
		args = append(args, "preshared-key", "/dev/stdin")
		cmd = exec.Command("wg", args...)
		cmd.Stdin = strings.NewReader(input.PresharedKey)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update peer: %s", string(output))
	}

	return nil
}

func (s *TunnelService) AddWireGuardAddress(name, address string) error {
	cmd := exec.Command("ip", "addr", "add", address, "dev", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add address: %s", string(output))
	}
	return nil
}

func (s *TunnelService) RemoveWireGuardAddress(name, address string) error {
	cmd := exec.Command("ip", "addr", "del", address, "dev", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove address: %s", string(output))
	}
	return nil
}

func (s *TunnelService) GenerateWireGuardKeyPair() (privateKey, publicKey string, err error) {
	genCmd := exec.Command("wg", "genkey")
	privOutput, err := genCmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}
	privateKey = strings.TrimSpace(string(privOutput))

	pubCmd := exec.Command("wg", "pubkey")
	pubCmd.Stdin = strings.NewReader(privateKey)
	pubOutput, err := pubCmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to derive public key: %w", err)
	}
	publicKey = strings.TrimSpace(string(pubOutput))

	return privateKey, publicKey, nil
}

// ==================== Common Helpers ====================

func (s *TunnelService) getInterfaceState(name string) (string, error) {
	cmd := exec.Command("ip", "link", "show", name)
	output, err := cmd.Output()
	if err != nil {
		return "UNKNOWN", err
	}
	if strings.Contains(string(output), "UP") {
		return "UP", nil
	}
	return "DOWN", nil
}

func (s *TunnelService) setInterfaceUp(name string) error {
	cmd := exec.Command("ip", "link", "set", name, "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface up: %s", string(output))
	}
	return nil
}

func (s *TunnelService) addInterfaceAddress(name, address string) error {
	cmd := exec.Command("ip", "addr", "add", address, "dev", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add address: %s", string(output))
	}
	return nil
}

func (s *TunnelService) SetInterfaceDown(name string) error {
	cmd := exec.Command("ip", "link", "set", name, "down")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface down: %s", string(output))
	}
	return nil
}

func (s *TunnelService) SetInterfaceUp(name string) error {
	return s.setInterfaceUp(name)
}

func (s *TunnelService) getInterfaceAddresses(name string) []string {
	cmd := exec.Command("ip", "-o", "addr", "show", name)
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

// ==================== Persistence ====================

func (s *TunnelService) SaveTunnels() error {
	tunnelDir := filepath.Join(s.configDir, "tunnels")
	if err := os.MkdirAll(tunnelDir, 0755); err != nil {
		return fmt.Errorf("failed to create tunnel config dir: %w", err)
	}

	// Save GRE tunnels
	greTunnels, _ := s.ListGRETunnels()
	if len(greTunnels) > 0 {
		var greLines []string
		for _, t := range greTunnels {
			line := fmt.Sprintf("%s %s %s", t.Name, t.Local, t.Remote)
			if t.Key != "" {
				line += " key " + t.Key
			}
			if t.TTL > 0 {
				line += fmt.Sprintf(" ttl %d", t.TTL)
			}
			greLines = append(greLines, line)
		}
		os.WriteFile(filepath.Join(tunnelDir, "gre.conf"), []byte(strings.Join(greLines, "\n")), 0644)
	}

	// Save VXLAN tunnels
	vxlanTunnels, _ := s.ListVXLANTunnels()
	if len(vxlanTunnels) > 0 {
		var vxlanLines []string
		for _, t := range vxlanTunnels {
			line := fmt.Sprintf("%s %d", t.Name, t.VNI)
			if t.Local != "" {
				line += " local " + t.Local
			}
			if t.Remote != "" {
				line += " remote " + t.Remote
			}
			if t.Group != "" {
				line += " group " + t.Group
			}
			if t.Dev != "" {
				line += " dev " + t.Dev
			}
			line += fmt.Sprintf(" dstport %d", t.DstPort)
			if t.MAC != "" {
				line += " mac " + t.MAC
			}
			vxlanLines = append(vxlanLines, line)
		}
		os.WriteFile(filepath.Join(tunnelDir, "vxlan.conf"), []byte(strings.Join(vxlanLines, "\n")), 0644)
	}

	// Save WireGuard configs using wg-quick format
	wgTunnels, _ := s.ListWireGuardTunnels()
	for _, t := range wgTunnels {
		s.saveWireGuardConfig(t)
	}

	return nil
}

func (s *TunnelService) saveWireGuardConfig(tunnel models.WireGuardTunnel) error {
	tunnelDir := filepath.Join(s.configDir, "tunnels", "wireguard")
	if err := os.MkdirAll(tunnelDir, 0700); err != nil {
		return err
	}

	// Get private key
	cmd := exec.Command("wg", "show", tunnel.Name, "private-key")
	privKey, _ := cmd.Output()

	var config strings.Builder
	config.WriteString("[Interface]\n")
	config.WriteString(fmt.Sprintf("PrivateKey = %s\n", strings.TrimSpace(string(privKey))))
	if tunnel.ListenPort > 0 {
		config.WriteString(fmt.Sprintf("ListenPort = %d\n", tunnel.ListenPort))
	}
	for _, addr := range tunnel.Addresses {
		config.WriteString(fmt.Sprintf("Address = %s\n", addr))
	}

	for _, peer := range tunnel.Peers {
		config.WriteString("\n[Peer]\n")
		config.WriteString(fmt.Sprintf("PublicKey = %s\n", peer.PublicKey))
		if peer.Endpoint != "" {
			config.WriteString(fmt.Sprintf("Endpoint = %s\n", peer.Endpoint))
		}
		if len(peer.AllowedIPs) > 0 {
			config.WriteString(fmt.Sprintf("AllowedIPs = %s\n", strings.Join(peer.AllowedIPs, ", ")))
		}
		if peer.PersistentKeepalive > 0 {
			config.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", peer.PersistentKeepalive))
		}
	}

	return os.WriteFile(filepath.Join(tunnelDir, tunnel.Name+".conf"), []byte(config.String()), 0600)
}

func (s *TunnelService) RestoreTunnels() error {
	tunnelDir := filepath.Join(s.configDir, "tunnels")

	// Restore GRE tunnels
	greData, err := os.ReadFile(filepath.Join(tunnelDir, "gre.conf"))
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(greData)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				input := models.GRETunnelInput{
					Name:   parts[0],
					Local:  parts[1],
					Remote: parts[2],
				}
				for i := 3; i < len(parts); i++ {
					if parts[i] == "key" && i+1 < len(parts) {
						input.Key = parts[i+1]
					} else if parts[i] == "ttl" && i+1 < len(parts) {
						input.TTL, _ = strconv.Atoi(parts[i+1])
					}
				}
				s.CreateGRETunnel(input)
			}
		}
	}

	// Restore VXLAN tunnels
	vxlanData, err := os.ReadFile(filepath.Join(tunnelDir, "vxlan.conf"))
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(vxlanData)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				vni, _ := strconv.Atoi(parts[1])
				input := models.VXLANTunnelInput{
					Name: parts[0],
					VNI:  vni,
				}
				for i := 2; i < len(parts); i++ {
					switch parts[i] {
					case "local":
						if i+1 < len(parts) {
							input.Local = parts[i+1]
						}
					case "remote":
						if i+1 < len(parts) {
							input.Remote = parts[i+1]
						}
					case "group":
						if i+1 < len(parts) {
							input.Group = parts[i+1]
						}
					case "dev":
						if i+1 < len(parts) {
							input.Dev = parts[i+1]
						}
					case "dstport":
						if i+1 < len(parts) {
							input.DstPort, _ = strconv.Atoi(parts[i+1])
						}
					case "mac":
						if i+1 < len(parts) {
							input.MAC = parts[i+1]
						}
					}
				}
				s.CreateVXLANTunnel(input)
			}
		}
	}

	// Restore WireGuard tunnels from wg-quick style configs
	wgDir := filepath.Join(tunnelDir, "wireguard")
	files, err := os.ReadDir(wgDir)
	if err == nil {
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".conf") {
				name := strings.TrimSuffix(file.Name(), ".conf")
				configPath := filepath.Join(wgDir, file.Name())
				exec.Command("wg-quick", "up", configPath).Run()
				_ = name // Interface will be created by wg-quick
			}
		}
	}

	return nil
}

// ==================== Namespace-Aware Methods ====================

// ListGRETunnelsInNamespace lists GRE tunnels in a specific namespace
func (s *TunnelService) ListGRETunnelsInNamespace(namespace string) ([]models.GRETunnel, error) {
	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", "tunnel", "show")
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "ip", "tunnel", "show")
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list GRE tunnels: %w", err)
	}

	var tunnels []models.GRETunnel
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		tunnel := s.parseGRETunnelLine(line)
		if tunnel != nil {
			state, _ := s.getInterfaceStateInNamespace(tunnel.Name, namespace)
			tunnel.State = state
			tunnel.Addresses = s.getInterfaceAddressesInNamespace(tunnel.Name, namespace)
			tunnels = append(tunnels, *tunnel)
		}
	}

	return tunnels, nil
}

// CreateGRETunnelInNamespace creates a GRE tunnel in a specific namespace
func (s *TunnelService) CreateGRETunnelInNamespace(input models.GRETunnelInput, namespace string) error {
	if input.Name == "" {
		return fmt.Errorf("tunnel name is required")
	}
	if input.Local == "" || input.Remote == "" {
		return fmt.Errorf("local and remote addresses are required")
	}

	args := []string{"tunnel", "add", input.Name, "mode", "gre",
		"local", input.Local, "remote", input.Remote}

	if input.Key != "" {
		args = append(args, "key", input.Key)
	}
	if input.TTL > 0 {
		args = append(args, "ttl", strconv.Itoa(input.TTL))
	}

	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", args...)
	} else {
		nsArgs := append([]string{"netns", "exec", namespace, "ip"}, args...)
		cmd = exec.Command("ip", nsArgs...)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create GRE tunnel: %s", string(output))
	}

	// Bring up
	if err := s.setInterfaceUpInNamespace(input.Name, namespace); err != nil {
		return fmt.Errorf("tunnel created but failed to bring up: %w", err)
	}

	// Add address
	if input.Address != "" {
		if err := s.addInterfaceAddressInNamespace(input.Name, input.Address, namespace); err != nil {
			return fmt.Errorf("tunnel created but failed to add address: %w", err)
		}
	}

	return nil
}

// DeleteGRETunnelInNamespace deletes a GRE tunnel in a specific namespace
func (s *TunnelService) DeleteGRETunnelInNamespace(name, namespace string) error {
	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", "tunnel", "del", name)
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "ip", "tunnel", "del", name)
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete GRE tunnel: %s", string(output))
	}
	return nil
}

// ListVXLANTunnelsInNamespace lists VXLAN tunnels in a specific namespace
func (s *TunnelService) ListVXLANTunnelsInNamespace(namespace string) ([]models.VXLANTunnel, error) {
	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", "-d", "link", "show", "type", "vxlan")
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "ip", "-d", "link", "show", "type", "vxlan")
	}

	output, err := cmd.Output()
	if err != nil {
		return []models.VXLANTunnel{}, nil
	}

	var tunnels []models.VXLANTunnel
	lines := strings.Split(string(output), "\n")

	var currentTunnel *models.VXLANTunnel
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if match := regexp.MustCompile(`^\d+:\s+(\S+):`).FindStringSubmatch(line); match != nil {
			if currentTunnel != nil {
				tunnels = append(tunnels, *currentTunnel)
			}
			currentTunnel = &models.VXLANTunnel{
				Name:    strings.TrimSuffix(match[1], "@NONE"),
				DstPort: 4789,
			}
			if strings.Contains(line, "UP") {
				currentTunnel.State = "UP"
			} else {
				currentTunnel.State = "DOWN"
			}
			if mtuMatch := regexp.MustCompile(`mtu\s+(\d+)`).FindStringSubmatch(line); mtuMatch != nil {
				currentTunnel.MTU, _ = strconv.Atoi(mtuMatch[1])
			}
		} else if currentTunnel != nil {
			if strings.Contains(line, "vxlan") {
				s.parseVXLANDetails(line, currentTunnel)
			} else if strings.Contains(line, "link/ether") {
				if macMatch := regexp.MustCompile(`link/ether\s+([0-9a-f:]+)`).FindStringSubmatch(line); macMatch != nil {
					currentTunnel.MAC = macMatch[1]
				}
			}
		}
	}

	if currentTunnel != nil {
		currentTunnel.Addresses = s.getInterfaceAddressesInNamespace(currentTunnel.Name, namespace)
		tunnels = append(tunnels, *currentTunnel)
	}

	for i := range tunnels {
		if tunnels[i].Addresses == nil {
			tunnels[i].Addresses = s.getInterfaceAddressesInNamespace(tunnels[i].Name, namespace)
		}
	}

	return tunnels, nil
}

// CreateVXLANTunnelInNamespace creates a VXLAN tunnel in a specific namespace
func (s *TunnelService) CreateVXLANTunnelInNamespace(input models.VXLANTunnelInput, namespace string) error {
	if input.Name == "" {
		return fmt.Errorf("tunnel name is required")
	}
	if input.VNI <= 0 {
		return fmt.Errorf("VNI is required and must be positive")
	}

	args := []string{"link", "add", input.Name, "type", "vxlan", "id", strconv.Itoa(input.VNI)}

	if input.Local != "" {
		args = append(args, "local", input.Local)
	}
	if input.Remote != "" {
		args = append(args, "remote", input.Remote)
	}
	if input.Group != "" {
		args = append(args, "group", input.Group)
	}
	if input.Dev != "" {
		args = append(args, "dev", input.Dev)
	}
	if input.DstPort > 0 {
		args = append(args, "dstport", strconv.Itoa(input.DstPort))
	} else {
		args = append(args, "dstport", "4789")
	}

	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", args...)
	} else {
		nsArgs := append([]string{"netns", "exec", namespace, "ip"}, args...)
		cmd = exec.Command("ip", nsArgs...)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create VXLAN tunnel: %s", string(output))
	}

	// Set MAC
	mac := input.MAC
	if mac == "" {
		mac = s.generateRandomMAC()
	}
	var macCmd *exec.Cmd
	if namespace == "" {
		macCmd = exec.Command("ip", "link", "set", input.Name, "address", mac)
	} else {
		macCmd = exec.Command("ip", "netns", "exec", namespace, "ip", "link", "set", input.Name, "address", mac)
	}
	macCmd.CombinedOutput()

	// Bring up
	if err := s.setInterfaceUpInNamespace(input.Name, namespace); err != nil {
		return fmt.Errorf("tunnel created but failed to bring up: %w", err)
	}

	// Add address
	if input.Address != "" {
		if err := s.addInterfaceAddressInNamespace(input.Name, input.Address, namespace); err != nil {
			return fmt.Errorf("tunnel created but failed to add address: %w", err)
		}
	}

	return nil
}

// DeleteVXLANTunnelInNamespace deletes a VXLAN tunnel in a specific namespace
func (s *TunnelService) DeleteVXLANTunnelInNamespace(name, namespace string) error {
	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", "link", "del", name)
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "ip", "link", "del", name)
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete VXLAN tunnel: %s", string(output))
	}
	return nil
}

// ListWireGuardTunnelsInNamespace lists WireGuard tunnels in a specific namespace
func (s *TunnelService) ListWireGuardTunnelsInNamespace(namespace string) ([]models.WireGuardTunnel, error) {
	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", "link", "show", "type", "wireguard")
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "ip", "link", "show", "type", "wireguard")
	}

	output, err := cmd.Output()
	if err != nil {
		return []models.WireGuardTunnel{}, nil
	}

	var tunnels []models.WireGuardTunnel
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if match := regexp.MustCompile(`^\d+:\s+(\S+):`).FindStringSubmatch(line); match != nil {
			name := strings.TrimSuffix(match[1], "@NONE")
			tunnel := models.WireGuardTunnel{
				Name: name,
			}
			if strings.Contains(line, "UP") {
				tunnel.State = "UP"
			} else {
				tunnel.State = "DOWN"
			}

			s.getWireGuardDetailsInNamespace(&tunnel, namespace)
			tunnel.Addresses = s.getInterfaceAddressesInNamespace(name, namespace)
			tunnels = append(tunnels, tunnel)
		}
	}

	return tunnels, nil
}

func (s *TunnelService) getWireGuardDetailsInNamespace(tunnel *models.WireGuardTunnel, namespace string) {
	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("wg", "show", tunnel.Name)
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "wg", "show", tunnel.Name)
	}

	output, err := cmd.Output()
	if err != nil {
		return
	}

	var currentPeer *models.WireGuardPeer
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "public key:") {
			tunnel.PublicKey = strings.TrimSpace(strings.TrimPrefix(line, "public key:"))
		} else if strings.HasPrefix(line, "listening port:") {
			port := strings.TrimSpace(strings.TrimPrefix(line, "listening port:"))
			tunnel.ListenPort, _ = strconv.Atoi(port)
		} else if strings.HasPrefix(line, "peer:") {
			if currentPeer != nil {
				tunnel.Peers = append(tunnel.Peers, *currentPeer)
			}
			currentPeer = &models.WireGuardPeer{
				PublicKey: strings.TrimSpace(strings.TrimPrefix(line, "peer:")),
			}
		} else if currentPeer != nil {
			if strings.HasPrefix(line, "endpoint:") {
				currentPeer.Endpoint = strings.TrimSpace(strings.TrimPrefix(line, "endpoint:"))
			} else if strings.HasPrefix(line, "allowed ips:") {
				ips := strings.TrimSpace(strings.TrimPrefix(line, "allowed ips:"))
				currentPeer.AllowedIPs = strings.Split(ips, ", ")
			} else if strings.HasPrefix(line, "latest handshake:") {
				currentPeer.LatestHandshake = strings.TrimSpace(strings.TrimPrefix(line, "latest handshake:"))
			} else if strings.HasPrefix(line, "transfer:") {
				s.parseWireGuardTransfer(line, currentPeer)
			} else if strings.HasPrefix(line, "persistent keepalive:") {
				ka := strings.TrimSpace(strings.TrimPrefix(line, "persistent keepalive:"))
				ka = strings.TrimSuffix(ka, " seconds")
				currentPeer.PersistentKeepalive, _ = strconv.Atoi(ka)
			}
		}
	}

	if currentPeer != nil {
		tunnel.Peers = append(tunnel.Peers, *currentPeer)
	}
}

// CreateWireGuardTunnelInNamespace creates a WireGuard tunnel in a specific namespace
func (s *TunnelService) CreateWireGuardTunnelInNamespace(input models.WireGuardTunnelInput, namespace string) error {
	if input.Name == "" {
		return fmt.Errorf("tunnel name is required")
	}

	// Create the interface
	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", "link", "add", input.Name, "type", "wireguard")
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "ip", "link", "add", input.Name, "type", "wireguard")
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create WireGuard interface: %s", string(output))
	}

	// Generate or set private key
	privateKey := input.PrivateKey
	if privateKey == "" {
		var genCmd *exec.Cmd
		if namespace == "" {
			genCmd = exec.Command("wg", "genkey")
		} else {
			genCmd = exec.Command("ip", "netns", "exec", namespace, "wg", "genkey")
		}
		keyOutput, err := genCmd.Output()
		if err != nil {
			s.DeleteWireGuardTunnelInNamespace(input.Name, namespace)
			return fmt.Errorf("failed to generate private key: %w", err)
		}
		privateKey = strings.TrimSpace(string(keyOutput))
	}

	// Set the private key
	var setCmd *exec.Cmd
	if namespace == "" {
		setCmd = exec.Command("wg", "set", input.Name, "private-key", "/dev/stdin")
	} else {
		setCmd = exec.Command("ip", "netns", "exec", namespace, "wg", "set", input.Name, "private-key", "/dev/stdin")
	}
	setCmd.Stdin = strings.NewReader(privateKey)
	if output, err := setCmd.CombinedOutput(); err != nil {
		s.DeleteWireGuardTunnelInNamespace(input.Name, namespace)
		return fmt.Errorf("failed to set private key: %s", string(output))
	}

	// Set listen port
	if input.ListenPort > 0 {
		var portCmd *exec.Cmd
		if namespace == "" {
			portCmd = exec.Command("wg", "set", input.Name, "listen-port", strconv.Itoa(input.ListenPort))
		} else {
			portCmd = exec.Command("ip", "netns", "exec", namespace, "wg", "set", input.Name, "listen-port", strconv.Itoa(input.ListenPort))
		}
		if output, err := portCmd.CombinedOutput(); err != nil {
			s.DeleteWireGuardTunnelInNamespace(input.Name, namespace)
			return fmt.Errorf("failed to set listen port: %s", string(output))
		}
	}

	// Add address
	if input.Address != "" {
		if err := s.addInterfaceAddressInNamespace(input.Name, input.Address, namespace); err != nil {
			fmt.Printf("Warning: failed to add address: %v\n", err)
		}
	}

	// Bring up
	if err := s.setInterfaceUpInNamespace(input.Name, namespace); err != nil {
		return fmt.Errorf("tunnel created but failed to bring up: %w", err)
	}

	return nil
}

// DeleteWireGuardTunnelInNamespace deletes a WireGuard tunnel in a specific namespace
func (s *TunnelService) DeleteWireGuardTunnelInNamespace(name, namespace string) error {
	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", "link", "del", name)
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "ip", "link", "del", name)
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete WireGuard tunnel: %s", string(output))
	}
	return nil
}

// Namespace helper methods
func (s *TunnelService) getInterfaceStateInNamespace(name, namespace string) (string, error) {
	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", "link", "show", name)
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "ip", "link", "show", name)
	}
	output, err := cmd.Output()
	if err != nil {
		return "UNKNOWN", err
	}
	if strings.Contains(string(output), "UP") {
		return "UP", nil
	}
	return "DOWN", nil
}

func (s *TunnelService) setInterfaceUpInNamespace(name, namespace string) error {
	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", "link", "set", name, "up")
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "ip", "link", "set", name, "up")
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface up: %s", string(output))
	}
	return nil
}

func (s *TunnelService) setInterfaceDownInNamespace(name, namespace string) error {
	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", "link", "set", name, "down")
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "ip", "link", "set", name, "down")
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface down: %s", string(output))
	}
	return nil
}

func (s *TunnelService) addInterfaceAddressInNamespace(name, address, namespace string) error {
	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", "addr", "add", address, "dev", name)
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "ip", "addr", "add", address, "dev", name)
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add address: %s", string(output))
	}
	return nil
}

func (s *TunnelService) getInterfaceAddressesInNamespace(name, namespace string) []string {
	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", "-o", "addr", "show", name)
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "ip", "-o", "addr", "show", name)
	}
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

// SetInterfaceUpInNamespace brings up an interface in a namespace (exported)
func (s *TunnelService) SetInterfaceUpInNamespace(name, namespace string) error {
	return s.setInterfaceUpInNamespace(name, namespace)
}

// SetInterfaceDownInNamespace brings down an interface in a namespace (exported)
func (s *TunnelService) SetInterfaceDownInNamespace(name, namespace string) error {
	return s.setInterfaceDownInNamespace(name, namespace)
}
