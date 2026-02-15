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

type IPSetService struct {
	configDir string
}

func NewIPSetService(configDir string) *IPSetService {
	return &IPSetService{configDir: configDir}
}

// ListSets returns all IP sets with their metadata (no entries)
func (s *IPSetService) ListSets() ([]models.IPSet, error) {
	cmd := exec.Command("ipset", "list", "-t")
	output, err := cmd.Output()
	if err != nil {
		// If ipset list returns error, it might mean no sets exist
		if exitErr, ok := err.(*exec.ExitError); ok {
			if len(exitErr.Stderr) > 0 && strings.Contains(string(exitErr.Stderr), "does not exist") {
				return []models.IPSet{}, nil
			}
		}
		return nil, fmt.Errorf("failed to list sets: %w", err)
	}

	return s.parseSetList(string(output))
}

// GetSet returns a single IP set with all entries
func (s *IPSetService) GetSet(name string) (*models.IPSet, error) {
	cmd := exec.Command("ipset", "list", name)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get set %s: %w", name, err)
	}

	sets, err := s.parseSetList(string(output))
	if err != nil {
		return nil, err
	}

	if len(sets) == 0 {
		return nil, fmt.Errorf("set %s not found", name)
	}

	return &sets[0], nil
}

// parseSetList parses ipset list output
func (s *IPSetService) parseSetList(output string) ([]models.IPSet, error) {
	var sets []models.IPSet
	var currentSet *models.IPSet
	inMembers := false

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		// New set starts with "Name:"
		if strings.HasPrefix(line, "Name:") {
			if currentSet != nil {
				sets = append(sets, *currentSet)
			}
			currentSet = &models.IPSet{
				Name:    strings.TrimSpace(strings.TrimPrefix(line, "Name:")),
				Entries: []string{},
			}
			inMembers = false
			continue
		}

		if currentSet == nil {
			continue
		}

		// Parse set properties
		if strings.HasPrefix(line, "Type:") {
			currentSet.Type = strings.TrimSpace(strings.TrimPrefix(line, "Type:"))
		} else if strings.HasPrefix(line, "Header:") {
			s.parseHeader(currentSet, strings.TrimPrefix(line, "Header:"))
		} else if strings.HasPrefix(line, "Size in memory:") {
			sizeStr := strings.TrimSpace(strings.TrimPrefix(line, "Size in memory:"))
			currentSet.MemSize, _ = strconv.Atoi(sizeStr)
		} else if strings.HasPrefix(line, "References:") {
			refStr := strings.TrimSpace(strings.TrimPrefix(line, "References:"))
			currentSet.References, _ = strconv.Atoi(refStr)
		} else if strings.HasPrefix(line, "Number of entries:") {
			numStr := strings.TrimSpace(strings.TrimPrefix(line, "Number of entries:"))
			currentSet.NumEntries, _ = strconv.Atoi(numStr)
		} else if strings.HasPrefix(line, "Members:") {
			inMembers = true
		} else if inMembers && strings.TrimSpace(line) != "" {
			currentSet.Entries = append(currentSet.Entries, strings.TrimSpace(line))
		}
	}

	if currentSet != nil {
		sets = append(sets, *currentSet)
	}

	return sets, nil
}

// parseHeader extracts properties from the Header line
func (s *IPSetService) parseHeader(set *models.IPSet, header string) {
	// Example: "family inet hashsize 1024 maxelem 65536 timeout 300"
	parts := strings.Fields(header)
	for i := 0; i < len(parts)-1; i++ {
		switch parts[i] {
		case "family":
			set.Family = parts[i+1]
		case "hashsize":
			set.HashSize, _ = strconv.Atoi(parts[i+1])
		case "maxelem":
			set.MaxElem, _ = strconv.Atoi(parts[i+1])
		case "timeout":
			set.Timeout, _ = strconv.Atoi(parts[i+1])
		}
	}
}

// CreateSet creates a new IP set
func (s *IPSetService) CreateSet(input models.IPSetInput) error {
	if input.Name == "" || input.Type == "" {
		return fmt.Errorf("name and type are required")
	}

	// Validate name (alphanumeric, underscore, hyphen)
	validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validName.MatchString(input.Name) {
		return fmt.Errorf("invalid set name: must contain only alphanumeric characters, underscores, and hyphens")
	}

	args := []string{"create", input.Name, input.Type}

	if input.Family != "" {
		args = append(args, "family", input.Family)
	}

	if input.Timeout > 0 {
		args = append(args, "timeout", strconv.Itoa(input.Timeout))
	}

	if input.Comment {
		args = append(args, "comment")
	}

	cmd := exec.Command("ipset", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create set: %s", string(output))
	}

	return nil
}

// DestroySet deletes an IP set
func (s *IPSetService) DestroySet(name string) error {
	cmd := exec.Command("ipset", "destroy", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to destroy set: %s", string(output))
	}
	return nil
}

// AddEntry adds an entry to an IP set
func (s *IPSetService) AddEntry(input models.IPSetEntryInput) error {
	if input.SetName == "" || input.Entry == "" {
		return fmt.Errorf("set name and entry are required")
	}

	args := []string{"add", input.SetName, input.Entry}

	if input.Timeout > 0 {
		args = append(args, "timeout", strconv.Itoa(input.Timeout))
	}

	if input.Comment != "" {
		args = append(args, "comment", input.Comment)
	}

	cmd := exec.Command("ipset", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add entry: %s", string(output))
	}

	return nil
}

// DeleteEntry removes an entry from an IP set
func (s *IPSetService) DeleteEntry(setName, entry string) error {
	cmd := exec.Command("ipset", "del", setName, entry)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete entry: %s", string(output))
	}
	return nil
}

// FlushSet removes all entries from an IP set
func (s *IPSetService) FlushSet(name string) error {
	cmd := exec.Command("ipset", "flush", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to flush set: %s", string(output))
	}
	return nil
}

// SaveSets saves all IP sets to a file
func (s *IPSetService) SaveSets() error {
	cmd := exec.Command("ipset", "save")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to save sets: %w", err)
	}

	// Ensure directory exists
	ipsetDir := filepath.Join(s.configDir, "ipset")
	if err := os.MkdirAll(ipsetDir, 0755); err != nil {
		return fmt.Errorf("failed to create ipset config dir: %w", err)
	}

	savePath := filepath.Join(ipsetDir, "ipset.save")
	if err := os.WriteFile(savePath, output, 0644); err != nil {
		return fmt.Errorf("failed to write ipset file: %w", err)
	}

	return nil
}

// RestoreSets restores IP sets from file
func (s *IPSetService) RestoreSets() error {
	savePath := filepath.Join(s.configDir, "ipset", "ipset.save")
	data, err := os.ReadFile(savePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No saved sets
		}
		return fmt.Errorf("failed to read ipset file: %w", err)
	}

	// Use -exist to avoid errors for already existing sets
	cmd := exec.Command("ipset", "restore", "-exist")
	cmd.Stdin = bytes.NewReader(data)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restore sets: %s", string(output))
	}

	return nil
}

// GetSetTypes returns supported ipset types
func (s *IPSetService) GetSetTypes() []string {
	return []string{
		"hash:ip",
		"hash:net",
		"hash:ip,port",
		"hash:net,port",
		"hash:ip,port,ip",
		"hash:ip,port,net",
		"hash:net,port,net",
		"hash:net,iface",
		"hash:mac",
		"list:set",
	}
}
