package services

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type PersistService struct {
	configDir string
}

func NewPersistService(configDir string) *PersistService {
	return &PersistService{configDir: configDir}
}

func (s *PersistService) ExportConfig() ([]byte, error) {
	// Create a temporary file for the archive
	tmpFile, err := os.CreateTemp("", "router-config-*.tar.gz")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(tmpFile)
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Walk through config directory and add files
	err = filepath.Walk(s.configDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(s.configDir, path)
		if err != nil {
			return err
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// If it's a file, write the content
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create archive: %w", err)
	}

	// Close writers to flush data
	tarWriter.Close()
	gzWriter.Close()

	// Read the file content
	tmpFile.Seek(0, 0)
	return io.ReadAll(tmpFile)
}

func (s *PersistService) ImportConfig(reader io.Reader) error {
	// Create gzip reader
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Get absolute config dir for comparison
	absConfigDir, err := filepath.Abs(s.configDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute config dir: %w", err)
	}

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		// Skip root directory entry "."
		if header.Name == "." || header.Name == "./" {
			continue
		}

		// Clean the path to prevent directory traversal
		cleanName := filepath.Clean(header.Name)
		if cleanName == ".." || strings.HasPrefix(cleanName, "../") {
			return fmt.Errorf("invalid path in archive: %s", header.Name)
		}

		// Construct full path
		targetPath := filepath.Join(s.configDir, cleanName)

		// Ensure the path is within config directory (security check)
		absTargetPath, err := filepath.Abs(targetPath)
		if err != nil || !strings.HasPrefix(absTargetPath, absConfigDir) {
			return fmt.Errorf("invalid path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Create file
			file, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				return fmt.Errorf("failed to write file: %w", err)
			}
			file.Close()

			// Set file permissions
			if err := os.Chmod(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to set permissions: %w", err)
			}
		}
	}

	return nil
}

func (s *PersistService) RestoreAll(
	iptables *IPTablesService,
	routes *IPRouteService,
	rules *IPRuleService,
	ipset *IPSetService,
	netns *NetnsService,
) error {
	var errors []string

	// Restore namespaces first (before other configs that might reference them)
	if netns != nil {
		if err := netns.RestoreNamespaces(); err != nil {
			errors = append(errors, "netns: "+err.Error())
		}
	}

	// Restore ipsets since iptables rules may reference them
	if err := ipset.RestoreSets(); err != nil {
		errors = append(errors, "ipset: "+err.Error())
	}

	if err := iptables.RestoreRules(); err != nil {
		errors = append(errors, "iptables: "+err.Error())
	}

	if err := routes.RestoreRoutes(); err != nil {
		errors = append(errors, "routes: "+err.Error())
	}

	if err := routes.RestoreIPForwarding(); err != nil {
		errors = append(errors, "ip_forward: "+err.Error())
	}

	if err := rules.RestoreRules(); err != nil {
		errors = append(errors, "rules: "+err.Error())
	}

	// Restore tunnels
	tunnelService := NewTunnelService(s.configDir)
	if err := tunnelService.RestoreTunnels(); err != nil {
		errors = append(errors, "tunnels: "+err.Error())
	}

	if len(errors) > 0 {
		return fmt.Errorf("some configurations failed to restore: %v", errors)
	}

	return nil
}

// GenerateSystemdService generates a systemd service file content
func (s *PersistService) GenerateSystemdService(binaryPath string) string {
	return fmt.Sprintf(`[Unit]
Description=Linux Router GUI
After=network.target

[Service]
Type=simple
ExecStart=%s
Restart=always
RestartSec=5
User=root
WorkingDirectory=%s

# Security settings
NoNewPrivileges=false
ProtectSystem=false
ProtectHome=false
PrivateTmp=false

[Install]
WantedBy=multi-user.target
`, binaryPath, filepath.Dir(binaryPath))
}

// GenerateRestoreScript generates a script to restore network configuration on boot
func (s *PersistService) GenerateRestoreScript() string {
	return fmt.Sprintf(`#!/bin/bash
# Linux Router Configuration Restore Script
# This script restores saved network configuration on boot

CONFIG_DIR="%s"

# Restore network namespaces first
if [ -f "$CONFIG_DIR/netns/namespaces.conf" ]; then
    echo "Restoring network namespaces..."
    while IFS= read -r ns; do
        [ -z "$ns" ] && continue
        [[ "$ns" =~ ^# ]] && continue
        if ! ip netns list | grep -q "^${ns}\s"; then
            ip netns add "$ns" 2>/dev/null || true
            ip netns exec "$ns" ip link set lo up 2>/dev/null || true
            echo "Created namespace: $ns"
        fi

        # Restore per-namespace iptables rules
        if [ -f "$CONFIG_DIR/netns/$ns/iptables.rules" ]; then
            ip netns exec "$ns" iptables-restore < "$CONFIG_DIR/netns/$ns/iptables.rules" 2>/dev/null || true
        fi

        # Restore per-namespace routes
        if [ -f "$CONFIG_DIR/netns/$ns/routes.conf" ]; then
            while IFS= read -r route; do
                [ -z "$route" ] && continue
                [[ "$route" =~ ^# ]] && continue
                [[ "$route" =~ ^default ]] && continue
                ip netns exec "$ns" ip route add $route 2>/dev/null || true
            done < "$CONFIG_DIR/netns/$ns/routes.conf"
        fi

        # Restore per-namespace IP rules
        if [ -f "$CONFIG_DIR/netns/$ns/rules.conf" ]; then
            while IFS= read -r rule; do
                [ -z "$rule" ] && continue
                [[ "$rule" =~ ^# ]] && continue
                ip netns exec "$ns" ip rule add $rule 2>/dev/null || true
            done < "$CONFIG_DIR/netns/$ns/rules.conf"
        fi
    done < "$CONFIG_DIR/netns/namespaces.conf"
fi

# Restore IP forwarding setting
if [ -f "$CONFIG_DIR/sysctl/ip_forward.conf" ]; then
    echo "Restoring IP forwarding setting..."
    while IFS= read -r line; do
        [ -z "$line" ] && continue
        [[ "$line" =~ ^# ]] && continue
        if [[ "$line" =~ ^net\.ipv4\.ip_forward= ]]; then
            value="${line#*=}"
            echo "$value" > /proc/sys/net/ipv4/ip_forward
            echo "IP forwarding set to $value"
        fi
    done < "$CONFIG_DIR/sysctl/ip_forward.conf"
fi

# Restore IPSets first (before iptables, since rules may reference them)
if [ -f "$CONFIG_DIR/ipset/ipset.save" ]; then
    echo "Restoring IPSets..."
    ipset restore -exist < "$CONFIG_DIR/ipset/ipset.save"
fi

# Restore iptables rules
if [ -f "$CONFIG_DIR/iptables/rules.v4" ]; then
    echo "Restoring iptables rules..."
    iptables-restore < "$CONFIG_DIR/iptables/rules.v4"
fi

# Restore routes
for table_file in "$CONFIG_DIR/routes"/*.conf; do
    if [ -f "$table_file" ]; then
        table=$(basename "$table_file" .conf)
        echo "Restoring routes for table: $table"
        while IFS= read -r line; do
            [ -z "$line" ] && continue
            [[ "$line" =~ ^# ]] && continue
            if [ "$table" = "main" ]; then
                ip route add $line 2>/dev/null || true
            else
                ip route add $line table "$table" 2>/dev/null || true
            fi
        done < "$table_file"
    fi
done

# Restore IP rules
if [ -f "$CONFIG_DIR/rules/ip-rules.conf" ]; then
    echo "Restoring IP rules..."
    while IFS= read -r line; do
        [ -z "$line" ] && continue
        [[ "$line" =~ ^# ]] && continue
        ip rule add $line 2>/dev/null || true
    done < "$CONFIG_DIR/rules/ip-rules.conf"
fi

# Restore GRE tunnels
if [ -f "$CONFIG_DIR/tunnels/gre.conf" ]; then
    echo "Restoring GRE tunnels..."
    while IFS= read -r line; do
        [ -z "$line" ] && continue
        [[ "$line" =~ ^# ]] && continue
        read -r name mode local remote rest <<< "$line"
        ip tunnel add "$name" mode "$mode" local "$local" remote "$remote" $rest 2>/dev/null || true
        ip link set "$name" up 2>/dev/null || true
    done < "$CONFIG_DIR/tunnels/gre.conf"
fi

# Restore VXLAN tunnels
if [ -f "$CONFIG_DIR/tunnels/vxlan.conf" ]; then
    echo "Restoring VXLAN tunnels..."
    while IFS= read -r line; do
        [ -z "$line" ] && continue
        [[ "$line" =~ ^# ]] && continue
        read -r name vni rest <<< "$line"
        ip link add "$name" type vxlan id "$vni" $rest 2>/dev/null || true
        ip link set "$name" up 2>/dev/null || true
    done < "$CONFIG_DIR/tunnels/vxlan.conf"
fi

# Restore WireGuard tunnels
if [ -d "$CONFIG_DIR/tunnels/wireguard" ]; then
    echo "Restoring WireGuard tunnels..."
    for conf in "$CONFIG_DIR/tunnels/wireguard"/*.conf; do
        [ -f "$conf" ] || continue
        wg-quick up "$conf" 2>/dev/null || true
    done
fi

echo "Network configuration restored."
`, s.configDir)
}
