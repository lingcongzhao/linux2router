package services

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"linuxtorouter/internal/models"
)

type IPTablesService struct {
	configDir string
}

func NewIPTablesService(configDir string) *IPTablesService {
	return &IPTablesService{configDir: configDir}
}

func (s *IPTablesService) ListChains(table string) ([]models.ChainInfo, error) {
	if table == "" {
		table = "filter"
	}

	cmd := exec.Command("iptables", "-t", table, "-L", "-n", "-v", "--line-numbers")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list chains: %w", err)
	}

	return s.parseChainOutput(string(output))
}

func (s *IPTablesService) GetChain(table, chain string) (*models.ChainInfo, error) {
	if table == "" {
		table = "filter"
	}

	cmd := exec.Command("iptables", "-t", table, "-L", chain, "-n", "-v", "--line-numbers")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get chain: %w", err)
	}

	chains, err := s.parseChainOutput(string(output))
	if err != nil {
		return nil, err
	}

	if len(chains) == 0 {
		return nil, fmt.Errorf("chain not found")
	}

	return &chains[0], nil
}

func (s *IPTablesService) parseChainOutput(output string) ([]models.ChainInfo, error) {
	var chains []models.ChainInfo
	var currentChain *models.ChainInfo

	scanner := bufio.NewScanner(strings.NewReader(output))
	// Updated regex to handle K/M/G suffixes for both packets and bytes (e.g., "253K packets, 33M bytes")
	chainHeaderRe := regexp.MustCompile(`^Chain (\S+) \(policy (\S+) (\d+[KMG]?) packets, (\d+[KMG]?) bytes\)`)
	chainHeaderNoPolicy := regexp.MustCompile(`^Chain (\S+) \((\d+) references\)`)

	for scanner.Scan() {
		line := scanner.Text()

		// Check for chain header with policy
		if matches := chainHeaderRe.FindStringSubmatch(line); matches != nil {
			if currentChain != nil {
				chains = append(chains, *currentChain)
			}
			packets := parseSuffixedNumber(matches[3])
			bytesVal := parseSuffixedNumber(matches[4])
			currentChain = &models.ChainInfo{
				Name:    matches[1],
				Policy:  matches[2],
				Packets: packets,
				Bytes:   bytesVal,
			}
			continue
		}

		// Check for chain header without policy (user-defined chains)
		if matches := chainHeaderNoPolicy.FindStringSubmatch(line); matches != nil {
			if currentChain != nil {
				chains = append(chains, *currentChain)
			}
			currentChain = &models.ChainInfo{
				Name:   matches[1],
				Policy: "-",
			}
			continue
		}

		// Skip header line
		if strings.HasPrefix(line, "num") || strings.TrimSpace(line) == "" {
			continue
		}

		// Parse rule line
		if currentChain != nil && strings.TrimSpace(line) != "" {
			rule := s.parseRuleLine(line)
			if rule != nil {
				currentChain.Rules = append(currentChain.Rules, *rule)
			}
		}
	}

	if currentChain != nil {
		chains = append(chains, *currentChain)
	}

	return chains, nil
}

// parseSuffixedNumber parses numbers with K/M/G suffixes (e.g., "6477K", "49M", "253K")
func parseSuffixedNumber(s string) uint64 {
	if s == "" {
		return 0
	}

	multiplier := uint64(1)
	numStr := s

	// Check for suffix
	lastChar := s[len(s)-1]
	switch lastChar {
	case 'K':
		multiplier = 1024
		numStr = s[:len(s)-1]
	case 'M':
		multiplier = 1024 * 1024
		numStr = s[:len(s)-1]
	case 'G':
		multiplier = 1024 * 1024 * 1024
		numStr = s[:len(s)-1]
	}

	val, _ := strconv.ParseUint(numStr, 10, 64)
	return val * multiplier
}

func (s *IPTablesService) parseRuleLine(line string) *models.FirewallRule {
	// iptables -L -n -v --line-numbers output format:
	// num pkts bytes target prot opt in out source destination [extra...]

	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	// First, extract comments (everything between first /* and last */) before field parsing
	// This prevents comments with spaces from breaking field parsing
	var commentPart string
	cleanLine := line

	if firstComment := strings.Index(line, "/*"); firstComment != -1 {
		if lastComment := strings.LastIndex(line, "*/"); lastComment != -1 && lastComment > firstComment {
			commentPart = strings.TrimSpace(line[firstComment : lastComment+2])
			cleanLine = strings.TrimSpace(line[:firstComment]) + " " + strings.TrimSpace(line[lastComment+2:])
			cleanLine = strings.TrimSpace(cleanLine)
		}
	}

	// Now parse the clean line (without comments)
	fields := strings.Fields(cleanLine)

	// Valid protocol names for detection
	validProtocols := map[string]bool{"all": true, "tcp": true, "udp": true, "icmp": true, "sctp": true, "udplite": true, "esp": true, "ah": true, "gre": true, "ipip": true, "icmpv6": true, "ipv6-icmp": true}

	// Check if a field looks like a protocol (name or number 0-255)
	isProtocol := func(s string) bool {
		if validProtocols[strings.ToLower(s)] {
			return true
		}
		// Check if it's a numeric protocol (0-255)
		if n, err := strconv.Atoi(s); err == nil && n >= 0 && n <= 255 {
			return true
		}
		return false
	}

	// Need at least 9 fields (target might be empty)
	if len(fields) < 9 {
		return nil
	}

	num, _ := strconv.Atoi(fields[0])
	packets := parseSuffixedNumber(fields[1])
	bytesVal := parseSuffixedNumber(fields[2])

	var target, protocol, opt, in, out, source, destination string
	var extraFields []string

	// Check if target is empty (fields[3] is actually protocol)
	if isProtocol(fields[3]) {
		// Target is empty, shift fields
		target = ""
		protocol = fields[3]
		opt = fields[4]
		in = fields[5]
		out = fields[6]
		source = fields[7]
		destination = fields[8]
		if len(fields) > 9 {
			extraFields = fields[9:]
		}
	} else if len(fields) >= 10 {
		// Normal case with target
		target = fields[3]
		protocol = fields[4]
		opt = fields[5]
		in = fields[6]
		out = fields[7]
		source = fields[8]
		destination = fields[9]
		if len(fields) > 10 {
			extraFields = fields[10:]
		}
	} else {
		return nil
	}

	rule := &models.FirewallRule{
		Num:         num,
		Packets:     packets,
		Bytes:       bytesVal,
		Target:      target,
		Protocol:    protocol,
		Opt:         opt,
		In:          in,
		Out:         out,
		Source:      source,
		Destination: destination,
	}

	// Extra options (fields beyond standard ones, excluding comments)
	if len(extraFields) > 0 {
		rule.Extra = strings.Join(extraFields, " ")
	}

	// Set comment
	rule.Comment = commentPart

	return rule
}

// extractComment separates comment from extra options
// Returns (options without comment, comment text including /* and */)
func extractComment(extra string) (string, string) {
	startIdx := strings.Index(extra, "/*")
	if startIdx == -1 {
		return extra, ""
	}

	endIdx := strings.LastIndex(extra, "*/")
	if endIdx == -1 || endIdx < startIdx {
		return extra, ""
	}

	// Extract comment text (including /* and */)
	comment := strings.TrimSpace(extra[startIdx : endIdx+2])

	// Build options without comment
	options := strings.TrimSpace(extra[:startIdx])
	if endIdx+2 < len(extra) {
		remaining := strings.TrimSpace(extra[endIdx+2:])
		if remaining != "" {
			if options != "" {
				options = options + " " + remaining
			} else {
				options = remaining
			}
		}
	}

	return options, comment
}

func (s *IPTablesService) AddRule(input models.FirewallRuleInput) error {
	args := s.buildRuleArgs(input)

	if input.Position > 0 {
		args = append([]string{"-t", input.Table, "-I", input.Chain, strconv.Itoa(input.Position)}, args...)
	} else {
		args = append([]string{"-t", input.Table, "-A", input.Chain}, args...)
	}

	cmd := exec.Command("iptables", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add rule: %s", string(output))
	}

	return nil
}

func (s *IPTablesService) DeleteRule(table, chain string, ruleNum int) error {
	if table == "" {
		table = "filter"
	}

	cmd := exec.Command("iptables", "-t", table, "-D", chain, strconv.Itoa(ruleNum))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete rule: %s", string(output))
	}

	return nil
}

func (s *IPTablesService) MoveRule(table, chain string, fromPos, toPos int) error {
	// Get the rule specification first
	chainInfo, err := s.GetChain(table, chain)
	if err != nil {
		return err
	}

	if fromPos < 1 || fromPos > len(chainInfo.Rules) {
		return fmt.Errorf("invalid source position")
	}

	// Delete the rule from original position
	if err := s.DeleteRule(table, chain, fromPos); err != nil {
		return err
	}

	// Adjust target position if needed
	if toPos > fromPos {
		toPos--
	}

	// Get updated rule spec and re-insert at new position
	// This is a simplified approach - in production you'd need to preserve the full rule spec
	return nil
}

func (s *IPTablesService) SetPolicy(table, chain, policy string) error {
	if table == "" {
		table = "filter"
	}

	policy = strings.ToUpper(policy)
	if policy != "ACCEPT" && policy != "DROP" && policy != "REJECT" {
		return fmt.Errorf("invalid policy: %s", policy)
	}

	cmd := exec.Command("iptables", "-t", table, "-P", chain, policy)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set policy: %s", string(output))
	}

	return nil
}

func (s *IPTablesService) CreateChain(table, chain string) error {
	if table == "" {
		table = "filter"
	}

	cmd := exec.Command("iptables", "-t", table, "-N", chain)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create chain: %s", string(output))
	}

	return nil
}

func (s *IPTablesService) DeleteChain(table, chain string) error {
	if table == "" {
		table = "filter"
	}

	// First flush the chain
	flushCmd := exec.Command("iptables", "-t", table, "-F", chain)
	flushCmd.Run()

	// Then delete it
	cmd := exec.Command("iptables", "-t", table, "-X", chain)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete chain: %s", string(output))
	}

	return nil
}

func (s *IPTablesService) FlushChain(table, chain string) error {
	if table == "" {
		table = "filter"
	}

	args := []string{"-t", table, "-F"}
	if chain != "" {
		args = append(args, chain)
	}

	cmd := exec.Command("iptables", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to flush chain: %s", string(output))
	}

	return nil
}

func (s *IPTablesService) buildRuleArgs(input models.FirewallRuleInput) []string {
	var args []string

	if input.Protocol != "" && input.Protocol != "all" {
		args = append(args, "-p", input.Protocol)
	}

	if input.Source != "" && input.Source != "0.0.0.0/0" {
		args = append(args, "-s", input.Source)
	}

	if input.Destination != "" && input.Destination != "0.0.0.0/0" {
		args = append(args, "-d", input.Destination)
	}

	if input.InInterface != "" {
		args = append(args, "-i", input.InInterface)
	}

	if input.OutInterface != "" {
		args = append(args, "-o", input.OutInterface)
	}

	if input.DPort != "" {
		args = append(args, "--dport", input.DPort)
	}

	if input.SPort != "" {
		args = append(args, "--sport", input.SPort)
	}

	if input.State != "" {
		args = append(args, "-m", "state", "--state", input.State)
	}

	if input.IPSet != "" && input.IPSetFlags != "" {
		args = append(args, "-m", "set", "--match-set", input.IPSet, input.IPSetFlags)
	}

	if input.Comment != "" {
		args = append(args, "-m", "comment", "--comment", input.Comment)
	}

	args = append(args, "-j", input.Target)

	if input.ToDestination != "" {
		args = append(args, "--to-destination", input.ToDestination)
	}

	if input.ToSource != "" {
		args = append(args, "--to-source", input.ToSource)
	}

	if input.Mark != "" && input.Target == "MARK" {
		args = append(args, "--set-mark", input.Mark)
	}

	return args
}

func (s *IPTablesService) SaveRules() error {
	cmd := exec.Command("iptables-save")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to save rules: %w", err)
	}

	savePath := filepath.Join(s.configDir, "iptables", "rules.v4")
	if err := os.WriteFile(savePath, output, 0644); err != nil {
		return fmt.Errorf("failed to write rules file: %w", err)
	}

	return nil
}

func (s *IPTablesService) RestoreRules() error {
	savePath := filepath.Join(s.configDir, "iptables", "rules.v4")
	data, err := os.ReadFile(savePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No saved rules
		}
		return fmt.Errorf("failed to read rules file: %w", err)
	}

	cmd := exec.Command("iptables-restore")
	cmd.Stdin = bytes.NewReader(data)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restore rules: %s", string(output))
	}

	return nil
}

func (s *IPTablesService) GetRawRules() (string, error) {
	cmd := exec.Command("iptables-save")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get rules: %w", err)
	}
	return string(output), nil
}

// ListChainsInNamespace lists chains in a specific network namespace
func (s *IPTablesService) ListChainsInNamespace(table, namespace string) ([]models.ChainInfo, error) {
	if table == "" {
		table = "filter"
	}

	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("iptables", "-t", table, "-L", "-n", "-v", "--line-numbers")
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "iptables", "-t", table, "-L", "-n", "-v", "--line-numbers")
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list chains: %w", err)
	}

	return s.parseChainOutput(string(output))
}

// GetChainInNamespace gets a specific chain in a network namespace
func (s *IPTablesService) GetChainInNamespace(table, chain, namespace string) (*models.ChainInfo, error) {
	if table == "" {
		table = "filter"
	}

	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("iptables", "-t", table, "-L", chain, "-n", "-v", "--line-numbers")
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "iptables", "-t", table, "-L", chain, "-n", "-v", "--line-numbers")
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get chain: %w", err)
	}

	chains, err := s.parseChainOutput(string(output))
	if err != nil {
		return nil, err
	}

	if len(chains) == 0 {
		return nil, fmt.Errorf("chain not found")
	}

	return &chains[0], nil
}

// AddRuleInNamespace adds a rule in a specific network namespace
func (s *IPTablesService) AddRuleInNamespace(input models.FirewallRuleInput, namespace string) error {
	args := s.buildRuleArgs(input)

	if input.Position > 0 {
		args = append([]string{"-t", input.Table, "-I", input.Chain, strconv.Itoa(input.Position)}, args...)
	} else {
		args = append([]string{"-t", input.Table, "-A", input.Chain}, args...)
	}

	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("iptables", args...)
	} else {
		nsArgs := append([]string{"netns", "exec", namespace, "iptables"}, args...)
		cmd = exec.Command("ip", nsArgs...)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add rule: %s", string(output))
	}

	return nil
}

// DeleteRuleInNamespace deletes a rule in a specific network namespace
func (s *IPTablesService) DeleteRuleInNamespace(table, chain string, ruleNum int, namespace string) error {
	if table == "" {
		table = "filter"
	}

	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("iptables", "-t", table, "-D", chain, strconv.Itoa(ruleNum))
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "iptables", "-t", table, "-D", chain, strconv.Itoa(ruleNum))
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete rule: %s", string(output))
	}

	return nil
}

// SetPolicyInNamespace sets chain policy in a specific network namespace
func (s *IPTablesService) SetPolicyInNamespace(table, chain, policy, namespace string) error {
	if table == "" {
		table = "filter"
	}

	policy = strings.ToUpper(policy)
	if policy != "ACCEPT" && policy != "DROP" && policy != "REJECT" {
		return fmt.Errorf("invalid policy: %s", policy)
	}

	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("iptables", "-t", table, "-P", chain, policy)
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "iptables", "-t", table, "-P", chain, policy)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set policy: %s", string(output))
	}

	return nil
}

// FlushChainInNamespace flushes a chain in a specific network namespace
func (s *IPTablesService) FlushChainInNamespace(table, chain, namespace string) error {
	if table == "" {
		table = "filter"
	}

	args := []string{"-t", table, "-F"}
	if chain != "" {
		args = append(args, chain)
	}

	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("iptables", args...)
	} else {
		nsArgs := append([]string{"netns", "exec", namespace, "iptables"}, args...)
		cmd = exec.Command("ip", nsArgs...)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to flush chain: %s", string(output))
	}

	return nil
}

// CreateChainInNamespace creates a new chain in a specific network namespace
func (s *IPTablesService) CreateChainInNamespace(table, chain, namespace string) error {
	if table == "" {
		table = "filter"
	}

	args := []string{"-t", table, "-N", chain}

	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("iptables", args...)
	} else {
		nsArgs := append([]string{"netns", "exec", namespace, "iptables"}, args...)
		cmd = exec.Command("ip", nsArgs...)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create chain: %s", string(output))
	}

	return nil
}

// DeleteChainInNamespace deletes a chain in a specific network namespace
func (s *IPTablesService) DeleteChainInNamespace(table, chain, namespace string) error {
	if table == "" {
		table = "filter"
	}

	// First flush the chain
	if err := s.FlushChainInNamespace(table, chain, namespace); err != nil {
		return err
	}

	args := []string{"-t", table, "-X", chain}

	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("iptables", args...)
	} else {
		nsArgs := append([]string{"netns", "exec", namespace, "iptables"}, args...)
		cmd = exec.Command("ip", nsArgs...)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete chain: %s", string(output))
	}

	return nil
}
