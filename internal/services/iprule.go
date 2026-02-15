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

type IPRuleService struct {
	configDir string
}

func NewIPRuleService(configDir string) *IPRuleService {
	return &IPRuleService{configDir: configDir}
}

func (s *IPRuleService) ListRules() ([]models.IPRule, error) {
	cmd := exec.Command("ip", "rule", "show")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list rules: %w", err)
	}

	return s.parseRuleOutput(string(output))
}

func (s *IPRuleService) parseRuleOutput(output string) ([]models.IPRule, error) {
	var rules []models.IPRule
	scanner := bufio.NewScanner(strings.NewReader(output))

	// Pattern: priority: selector action
	// Example: 0:	from all lookup local
	// Example: 32766:	from all lookup main
	re := regexp.MustCompile(`^(\d+):\s+(.+)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		priority, _ := strconv.Atoi(matches[1])
		rest := matches[2]

		rule := models.IPRule{
			Priority: priority,
			Selector: rest,
		}

		// Parse the rest of the rule
		parts := strings.Fields(rest)
		for i := 0; i < len(parts); i++ {
			switch parts[i] {
			case "from":
				if i+1 < len(parts) {
					rule.From = parts[i+1]
					i++
				}
			case "to":
				if i+1 < len(parts) {
					rule.To = parts[i+1]
					i++
				}
			case "fwmark":
				if i+1 < len(parts) {
					rule.FWMark = parts[i+1]
					i++
				}
			case "iif":
				if i+1 < len(parts) {
					rule.IIF = parts[i+1]
					i++
				}
			case "oif":
				if i+1 < len(parts) {
					rule.OIF = parts[i+1]
					i++
				}
			case "lookup":
				if i+1 < len(parts) {
					rule.Table = parts[i+1]
					rule.Action = "lookup"
					i++
				}
			case "unreachable":
				rule.Action = "unreachable"
			case "blackhole":
				rule.Action = "blackhole"
			case "prohibit":
				rule.Action = "prohibit"
			case "not":
				rule.Not = true
			}
		}

		rules = append(rules, rule)
	}

	return rules, nil
}

func (s *IPRuleService) AddRule(input models.IPRuleInput) error {
	args := []string{"rule", "add"}

	if input.Priority > 0 {
		args = append(args, "priority", strconv.Itoa(input.Priority))
	}

	if input.Not {
		args = append(args, "not")
	}

	if input.From != "" {
		args = append(args, "from", input.From)
	} else {
		args = append(args, "from", "all")
	}

	if input.To != "" {
		args = append(args, "to", input.To)
	}

	if input.FWMark != "" {
		args = append(args, "fwmark", input.FWMark)
	}

	if input.IIF != "" {
		args = append(args, "iif", input.IIF)
	}

	if input.OIF != "" {
		args = append(args, "oif", input.OIF)
	}

	if input.Table != "" {
		args = append(args, "lookup", input.Table)
	}

	cmd := exec.Command("ip", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add rule: %s", string(output))
	}

	return nil
}

func (s *IPRuleService) DeleteRule(priority int, from, to string) error {
	args := []string{"rule", "del"}

	if priority > 0 {
		args = append(args, "priority", strconv.Itoa(priority))
	}

	if from != "" {
		args = append(args, "from", from)
	}

	if to != "" {
		args = append(args, "to", to)
	}

	cmd := exec.Command("ip", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete rule: %s", string(output))
	}

	return nil
}

func (s *IPRuleService) DeleteByPriority(priority int) error {
	// Get the rule details first
	rules, err := s.ListRules()
	if err != nil {
		return err
	}

	for _, rule := range rules {
		if rule.Priority == priority {
			return s.DeleteRule(priority, rule.From, rule.To)
		}
	}

	return fmt.Errorf("rule with priority %d not found", priority)
}

func (s *IPRuleService) SaveRules() error {
	rules, err := s.ListRules()
	if err != nil {
		return err
	}

	var lines []string
	for _, rule := range rules {
		// Skip default rules
		if rule.Priority == 0 || rule.Priority == 32766 || rule.Priority == 32767 {
			continue
		}

		line := fmt.Sprintf("priority %d", rule.Priority)

		if rule.Not {
			line += " not"
		}

		if rule.From != "" && rule.From != "all" {
			line += " from " + rule.From
		}

		if rule.To != "" {
			line += " to " + rule.To
		}

		if rule.FWMark != "" {
			line += " fwmark " + rule.FWMark
		}

		if rule.IIF != "" {
			line += " iif " + rule.IIF
		}

		if rule.OIF != "" {
			line += " oif " + rule.OIF
		}

		if rule.Table != "" {
			line += " lookup " + rule.Table
		} else if rule.Action != "" {
			line += " " + rule.Action
		}

		lines = append(lines, line)
	}

	savePath := filepath.Join(s.configDir, "rules", "ip-rules.conf")
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(savePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to save rules: %w", err)
	}

	return nil
}

func (s *IPRuleService) RestoreRules() error {
	savePath := filepath.Join(s.configDir, "rules", "ip-rules.conf")
	data, err := os.ReadFile(savePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		args := append([]string{"rule", "add"}, strings.Fields(line)...)
		exec.Command("ip", args...).Run()
	}

	return nil
}

// ListRulesInNamespace lists IP rules in a specific network namespace
func (s *IPRuleService) ListRulesInNamespace(namespace string) ([]models.IPRule, error) {
	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", "rule", "show")
	} else {
		cmd = exec.Command("ip", "netns", "exec", namespace, "ip", "rule", "show")
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list rules: %w", err)
	}

	return s.parseRuleOutput(string(output))
}

// AddRuleInNamespace adds an IP rule in a specific network namespace
func (s *IPRuleService) AddRuleInNamespace(input models.IPRuleInput, namespace string) error {
	args := []string{"rule", "add"}

	if input.Priority > 0 {
		args = append(args, "priority", strconv.Itoa(input.Priority))
	}

	if input.Not {
		args = append(args, "not")
	}

	if input.From != "" {
		args = append(args, "from", input.From)
	} else {
		args = append(args, "from", "all")
	}

	if input.To != "" {
		args = append(args, "to", input.To)
	}

	if input.FWMark != "" {
		args = append(args, "fwmark", input.FWMark)
	}

	if input.IIF != "" {
		args = append(args, "iif", input.IIF)
	}

	if input.OIF != "" {
		args = append(args, "oif", input.OIF)
	}

	if input.Table != "" {
		args = append(args, "lookup", input.Table)
	}

	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", args...)
	} else {
		nsArgs := append([]string{"netns", "exec", namespace, "ip"}, args...)
		cmd = exec.Command("ip", nsArgs...)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add rule: %s", string(output))
	}

	return nil
}

// DeleteRuleInNamespace deletes an IP rule by priority in a specific network namespace
func (s *IPRuleService) DeleteRuleInNamespace(priority int, namespace string) error {
	// Get the rule details first
	rules, err := s.ListRulesInNamespace(namespace)
	if err != nil {
		return err
	}

	var targetRule *models.IPRule
	for _, rule := range rules {
		if rule.Priority == priority {
			targetRule = &rule
			break
		}
	}

	if targetRule == nil {
		return fmt.Errorf("rule with priority %d not found", priority)
	}

	args := []string{"rule", "del", "priority", strconv.Itoa(priority)}

	if targetRule.From != "" {
		args = append(args, "from", targetRule.From)
	}

	if targetRule.To != "" {
		args = append(args, "to", targetRule.To)
	}

	var cmd *exec.Cmd
	if namespace == "" {
		cmd = exec.Command("ip", args...)
	} else {
		nsArgs := append([]string{"netns", "exec", namespace, "ip"}, args...)
		cmd = exec.Command("ip", nsArgs...)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete rule: %s", string(output))
	}

	return nil
}
