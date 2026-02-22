# Configuration Save/Export/Import Test Plan

## Overview
This test plan covers all configuration save, export, and import features across the application.

## Test Environment
- Platform: Linux
- Prerequisites: Root access (for network operations), Go 1.21+

---

## Test Cases

### 1. Save All Configurations (/settings)

**Purpose**: Verify that clicking "Save All Configurations" saves all network configurations to persistent storage.

**Test Steps**:
1. Start the application: `sudo ./router-gui`
2. Navigate to http://localhost:8090/settings
3. Login with admin credentials
4. Click "Save All Configurations" button
5. Verify success message appears

**Expected Results**:
- Success alert: "All configurations saved successfully"
- Files created in config directory:
  - `configs/ipset/ipset.save`
  - `configs/iptables/rules.v4`
  - `configs/routes/*.conf` (per table)
  - `configs/sysctl/ip_forward.conf`
  - `configs/rules/ip-rules.conf`
  - `configs/tunnels/gre.conf`
  - `configs/tunnels/vxlan.conf`
  - `configs/tunnels/wireguard/*.conf`
  - `configs/netns/namespaces.conf`
  - `configs/dnsmasq/config.json`

**Verification Commands**:
```bash
ls -la /var/lib/router-gui/configs/
```

---

### 2. Export Config (/settings)

**Purpose**: Verify that clicking "Export Config" downloads a compressed archive of all configurations.

**Test Steps**:
1. Navigate to http://localhost:8090/settings
2. Click "Export Config" button
3. Verify file download starts

**Expected Results**:
- File downloads as `router-config.tar.gz`
- Archive contains all config files

**Verification Commands**:
```bash
# Extract and verify archive contents
tar -tzf router-config.tar.gz
```

---

### 3. Import Config (/settings)

**Purpose**: Verify that importing a configuration archive restores all network configurations.

**Test Steps**:
1. First, create a backup by exporting: Click "Export Config"
2. Make some network changes (add routes, firewall rules, etc.)
3. Click "Import Config" button
4. Upload the previously exported archive
5. Verify success message

**Expected Results**:
- Success alert: "Configuration imported and applied successfully"
- All network configurations are restored

**Verification Commands**:
```bash
# Check iptables rules are restored
sudo iptables -L -n

# Check routes are restored
ip route show

# Check IP sets are restored
ipset list

# Check tunnels are restored
ip tunnel show
ip link show type vxlan
ip link show type wireguard

# Check Dnsmasq is running with restored config
sudo systemctl status dnsmasq
```

---

### 4. Save Config (/netns)

**Purpose**: Verify that clicking "Save Config" on the Network Namespaces page saves namespace configurations.

**Test Steps**:
1. Navigate to http://localhost:8090/netns
2. Create a network namespace (optional)
3. Click "Save Config" button
4. Verify success message

**Expected Results**:
- Success alert: "Namespaces saved successfully"
- Files created:
  - `configs/netns/namespaces.conf`
  - `configs/netns/{namespace}/iptables.rules` (per namespace)
  - `configs/netns/{namespace}/routes.conf` (per namespace)
  - `configs/netns/{namespace}/rules.conf` (per namespace)

**Verification Commands**:
```bash
ls -la /var/lib/router-gui/configs/netns/
```

---

### 5. Save Tunnels (/tunnels/gre, /tunnels/vxlan, /tunnels/wireguard)

**Purpose**: Verify that clicking "Save Tunnels" saves tunnel configurations.

**Test Steps**:
1. Navigate to http://localhost:8090/tunnels/gre (or vxlan, wireguard)
2. Create a tunnel (optional)
3. Click "Save Tunnels" button
4. Verify success message

**Expected Results**:
- Success alert: "Tunnels saved successfully"
- Files created:
  - `configs/tunnels/gre.conf`
  - `configs/tunnels/vxlan.conf`
  - `configs/tunnels/wireguard/*.conf`

**Verification Commands**:
```bash
ls -la /var/lib/router-gui/configs/tunnels/
```

---

### 6. Save Tunnels in Namespace (/netns/{name}/tunnels/gre, vxlan, wireguard)

**Purpose**: Verify that clicking "Save Tunnels" in a namespace saves namespace-specific tunnel configurations.

**Test Steps**:
1. Navigate to a namespace's tunnel page (e.g., http://localhost:8090/netns/myns/tunnels/gre)
2. Create a tunnel in the namespace (optional)
3. Click "Save Tunnels" button
4. Verify success message

**Expected Results**:
- Success alert: "Tunnels saved successfully"
- Files created in `configs/netns/{namespace}/`:
  - `gre.conf`
  - `vxlan.conf`
  - `{tunnel}.conf` (for WireGuard)

**Verification Commands**:
```bash
ls -la /var/lib/router-gui/configs/netns/{namespace}/
```

---

### 7. Partial Failure Test

**Purpose**: Verify graceful handling when some configurations fail to restore.

**Test Steps**:
1. Create a backup
2. Manually corrupt one of the config files in the archive
3. Import the corrupted archive

**Expected Results**:
- Warning alert shows which configurations failed
- Other configurations are restored successfully

---

### 8. Empty Configuration Test

**Purpose**: Verify behavior when importing an empty or minimal configuration.

**Test Steps**:
1. Create a minimal config archive with only some files
2. Import it

**Expected Results**:
- Success or warning depending on what's missing
- No crashes or errors

---

## All Save Functions Summary

| Location | Handler | Endpoint | Status |
|----------|---------|----------|--------|
| /settings - Save All Configurations | `settingsHandler.SaveAll` | POST /settings/save-all | ✓ Fixed |
| /settings - Export Config | `settingsHandler.ExportConfig` | GET /settings/export | ✓ Working |
| /settings - Import Config | `settingsHandler.ImportConfig` | POST /settings/import | ✓ Fixed |
| /netns - Save Config | `netnsHandler.SaveNamespaces` | POST /netns/save | ✓ Working |
| /tunnels/gre - Save Tunnels | `tunnelHandler.SaveTunnels` | POST /tunnels/save | ✓ Working |
| /tunnels/vxlan - Save Tunnels | `tunnelHandler.SaveTunnels` | POST /tunnels/save | ✓ Working |
| /tunnels/wireguard - Save Tunnels | `tunnelHandler.SaveTunnels` | POST /tunnels/save | ✓ Working |
| /netns/{name}/tunnels/gre - Save Tunnels | `netnsHandler.SaveTunnels` | POST /netns/{name}/tunnels/save | ✓ Fixed |
| /netns/{name}/tunnels/vxlan - Save Tunnels | `netnsHandler.SaveTunnels` | POST /netns/{name}/tunnels/save | ✓ Fixed |
| /netns/{name}/tunnels/wireguard - Save Tunnels | `netnsHandler.SaveTunnels` | POST /netns/{name}/tunnels/save | ✓ Fixed |

---

## Configuration Components Coverage

| Component | Save | Export | Import | Restore |
|-----------|------|--------|--------|---------|
| IPSets | ✓ | ✓ | ✓ | ✓ |
| iptables | ✓ | ✓ | ✓ | ✓ |
| Routes | ✓ | ✓ | ✓ | ✓ |
| IP Forwarding | ✓ | ✓ | ✓ | ✓ |
| IP Rules | ✓ | ✓ | ✓ | ✓ |
| Tunnels (GRE/VXLAN/WG) | ✓ | ✓ | ✓ | ✓ |
| Network Namespaces | ✓ | ✓ | ✓ | ✓ |
| Namespace-specific Tunnels | ✓ | - | - | ✓ |
| Dnsmasq (DHCP/DNS) | ✓ | ✓ | ✓ | ✓ |

---

## Known Issues Fixed

1. **Issue**: Save All Configurations did not save network namespaces
   - **Fix**: Added `netnsService.SaveNamespaces()` call in `internal/handlers/settings.go`

2. **Issue**: Save All Configurations did not save Dnsmasq configuration
   - **Fix**: Added `dnsmasqService.SaveConfig()` call in `internal/handlers/settings.go`

3. **Issue**: Import Config did not restore network namespaces
   - **Fix**: Added `netnsService.RestoreNamespaces()` call in `internal/handlers/settings.go`
   - **Note**: Restore order fixed to restore namespaces first (before configs that reference them)

4. **Issue**: Import Config did not restore Dnsmasq configuration
   - **Fix**: Added `dnsmasqService.RestoreConfig()` call in `internal/handlers/settings.go`

5. **Issue**: Missing route for namespace tunnel save buttons
   - **Fix**: Added route `POST /netns/{name}/tunnels/save` in `cmd/server/main.go`
   - **Affected pages**: netns_gre.html, netns_vxlan.html, netns_wireguard.html

6. **Code Cleanup**: Removed unused `extractComment` function from `internal/services/iptables.go`

7. **Bug Fix**: Button stuck on "Loading..." on first click
   - **Root Cause**: HTMX event listeners were not wrapped in `DOMContentLoaded`, causing them to not attach properly on first page load
   - **Fix**: Wrapped HTMX event listeners in `DOMContentLoaded` in `web/static/js/app.js`
   - **Additional Fix**: Added `type="button"` to the Save All Configurations button to prevent submit behavior

---

## Files Modified

- `internal/handlers/settings.go` - Added missing service calls for Save All and Import
- `internal/services/iptables.go` - Removed dead code (extractComment function)
- `cmd/server/main.go` - Added missing route for namespace tunnel save, updated handler initialization
- `web/static/js/app.js` - Fixed HTMX event listener timing issue
- `web/templates/pages/settings.html` - Added type="button" to Save All button
