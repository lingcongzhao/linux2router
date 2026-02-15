# Network Namespace (netns) Feature Design

## Overview
Network namespaces provide isolated network stacks, each with its own interfaces, routing tables, firewall rules, and more. This feature will allow users to manage multiple isolated network environments from a single GUI.

## Core Functionality

### 1. Namespace Management
- **List namespaces**: Show all existing network namespaces
- **Create namespace**: Create new network namespace with optional configuration
- **Delete namespace**: Remove namespace (with confirmation for non-empty namespaces)
- **Namespace details**: View detailed info about a specific namespace

### 2. Integration with Existing Features

Each existing feature should be "namespace-aware" with a namespace selector:

| Feature | Integration |
|---------|-------------|
| Network Interfaces | Move interfaces between namespaces, create veth pairs |
| GRE Tunnels | Create tunnels within specific namespace |
| VXLAN Tunnels | Create tunnels within specific namespace |
| WireGuard Tunnels | Create tunnels within specific namespace |
| Firewall (iptables) | View/manage rules per namespace |
| Routing Tables | View/manage routes per namespace |
| IP Rules | View/manage policy routing per namespace |

## Data Models

### Namespace Model
```go
// internal/models/netns.go

type NetworkNamespace struct {
    Name        string      `json:"name"`
    ID          int64       `json:"id,omitempty"`      // inode number
    Interfaces  []string    `json:"interfaces"`        // interfaces in this namespace
    Created     time.Time   `json:"created,omitempty"`
    Persistent  bool        `json:"persistent"`        // survives reboot
}

type NetnsCreateInput struct {
    Name       string `json:"name"`
    Persistent bool   `json:"persistent"` // create file in /etc/netns/
}

type VethPairInput struct {
    Name1     string `json:"name1"`      // first interface name
    Name2     string `json:"name2"`      // second interface name
    Namespace string `json:"namespace"`  // namespace for name2 (name1 stays in default)
    Address1  string `json:"address1"`   // optional IP for name1
    Address2  string `json:"address2"`   // optional IP for name2
}

type MoveInterfaceInput struct {
    Interface string `json:"interface"`
    Namespace string `json:"namespace"` // empty = default namespace
}
```

## Service Layer

### NetnsService Methods
```go
// internal/services/netns.go

type NetnsService struct {
    configDir string
}

// Namespace CRUD
func (s *NetnsService) ListNamespaces() ([]NetworkNamespace, error)
func (s *NetnsService) GetNamespace(name string) (*NetworkNamespace, error)
func (s *NetnsService) CreateNamespace(input NetnsCreateInput) error
func (s *NetnsService) DeleteNamespace(name string) error

// Interface operations
func (s *NetnsService) MoveInterface(input MoveInterfaceInput) error
func (s *NetnsService) CreateVethPair(input VethPairInput) error
func (s *NetnsService) GetNamespaceInterfaces(namespace string) ([]string, error)

// Execute in namespace (for other services to use)
func (s *NetnsService) ExecInNamespace(namespace string, cmd string, args ...string) ([]byte, error)

// Persistence
func (s *NetnsService) SaveNamespaces() error
func (s *NetnsService) RestoreNamespaces() error
```

### Modified Existing Services

Each existing service needs a namespace parameter:

```go
// Example: IPTablesService modifications
func (s *IPTablesService) ListChains(table string, namespace string) ([]ChainInfo, error)
func (s *IPTablesService) AddRule(input FirewallRuleInput, namespace string) error

// Example: IPRouteService modifications
func (s *IPRouteService) ListRoutes(table string, namespace string) ([]Route, error)
func (s *IPRouteService) AddRoute(input RouteInput, namespace string) error

// Example: TunnelService modifications
func (s *TunnelService) CreateGRETunnel(input GRETunnelInput, namespace string) error
func (s *TunnelService) ListGRETunnels(namespace string) ([]GRETunnel, error)
```

## Handler Layer

### NetnsHandler
```go
// internal/handlers/netns.go

type NetnsHandler struct {
    templates     TemplateExecutor
    netnsService  *NetnsService
    userService   *auth.UserService
}

// Routes
// GET  /netns                    - List all namespaces
// GET  /netns/list               - HTMX partial for namespace list
// POST /netns                    - Create namespace
// GET  /netns/{name}             - Namespace detail page
// DELETE /netns/{name}           - Delete namespace
// POST /netns/{name}/interfaces  - Move interface to namespace
// POST /netns/veth               - Create veth pair
// POST /netns/save               - Save namespace config
```

## API Routes

```go
// cmd/server/main.go additions

// Network Namespaces
r.Get("/netns", netnsHandler.List)
r.Get("/netns/list", netnsHandler.GetNamespaces)
r.Post("/netns", netnsHandler.CreateNamespace)
r.Get("/netns/{name}", netnsHandler.ViewNamespace)
r.Delete("/netns/{name}", netnsHandler.DeleteNamespace)
r.Post("/netns/{name}/interfaces", netnsHandler.MoveInterface)
r.Delete("/netns/{name}/interfaces", netnsHandler.RemoveInterface)
r.Post("/netns/veth", netnsHandler.CreateVethPair)
r.Post("/netns/save", netnsHandler.SaveNamespaces)

// Namespace-specific routes for existing features
r.Get("/netns/{name}/interfaces", netnsHandler.ListInterfaces)
r.Get("/netns/{name}/firewall", netnsHandler.ListFirewallRules)
r.Get("/netns/{name}/routes", netnsHandler.ListRoutes)
r.Get("/netns/{name}/rules", netnsHandler.ListIPRules)
r.Get("/netns/{name}/tunnels/gre", netnsHandler.ListGRETunnels)
r.Get("/netns/{name}/tunnels/vxlan", netnsHandler.ListVXLANTunnels)
r.Get("/netns/{name}/tunnels/wireguard", netnsHandler.ListWireGuardTunnels)
```

## UI Design

### 1. Main Namespace Page (`/netns`)

```
+----------------------------------------------------------+
| Network Namespaces                    [+ New] [Save All] |
+----------------------------------------------------------+
| Name        | Interfaces | Routes | Rules | Actions      |
|-------------|------------|--------|-------|--------------|
| ns-customer1| 3          | 5      | 2     | [View] [Del] |
| ns-customer2| 2          | 3      | 1     | [View] [Del] |
| ns-isolated | 1          | 1      | 0     | [View] [Del] |
+----------------------------------------------------------+
```

### 2. Namespace Detail Page (`/netns/{name}`)

```
+----------------------------------------------------------+
| <- Back    Namespace: ns-customer1              [Delete] |
+----------------------------------------------------------+
| Tabs: [Interfaces] [Firewall] [Routes] [Rules] [Tunnels] |
+----------------------------------------------------------+
|                                                          |
| Interfaces in this namespace:                            |
| +------------------------------------------------------+ |
| | Name    | Type  | State | IP Address      | Actions  | |
| |---------|-------|-------|-----------------|----------| |
| | veth1   | veth  | UP    | 10.0.1.1/24    | [Remove] | |
| | gre-cust| gre   | UP    | 192.168.1.1/30 | [Remove] | |
| +------------------------------------------------------+ |
|                                                          |
| [+ Add Interface] [+ Create Veth Pair] [+ Create Tunnel] |
+----------------------------------------------------------+
```

### 3. Namespace Selector Component

Add a namespace selector dropdown to existing pages:

```
+----------------------------------------------------------+
| Firewall (iptables)                                      |
| Namespace: [Default (root)] [v]     [+ New Chain] [Save] |
+----------------------------------------------------------+
```

## Templates

### New Templates Required
```
web/templates/pages/
├── netns.html              # Main namespace list page
├── netns_detail.html       # Namespace detail page

web/templates/partials/
├── netns_table.html        # Namespace list table (HTMX)
├── netns_interfaces.html   # Interfaces in namespace (HTMX)
├── netns_selector.html     # Namespace dropdown component
```

## Shell Commands Reference

```bash
# List namespaces
ip netns list

# Create namespace
ip netns add <name>

# Delete namespace
ip netns delete <name>

# Execute command in namespace
ip netns exec <name> <command>

# Move interface to namespace
ip link set <interface> netns <namespace>

# Create veth pair
ip link add <name1> type veth peer name <name2>
ip link set <name2> netns <namespace>

# List interfaces in namespace
ip netns exec <name> ip link show

# Firewall in namespace
ip netns exec <name> iptables -L -n -v

# Routes in namespace
ip netns exec <name> ip route show

# IP rules in namespace
ip netns exec <name> ip rule show
```

## Persistence Strategy

### Configuration File Structure
```
configs/
├── netns/
│   ├── namespaces.conf      # List of namespaces to create
│   ├── veth-pairs.conf      # Veth pairs configuration
│   └── ns-customer1/        # Per-namespace configs
│       ├── interfaces.conf  # Interface assignments
│       ├── iptables.rules   # Firewall rules
│       ├── routes.conf      # Static routes
│       └── rules.conf       # IP rules
```

### namespaces.conf format
```
# name:persistent
ns-customer1:true
ns-customer2:true
ns-isolated:false
```

### veth-pairs.conf format
```
# name1:name2:namespace:address1:address2
veth-root:veth-ns1:ns-customer1:10.0.1.1/24:10.0.1.2/24
veth-root2:veth-ns2:ns-customer2:10.0.2.1/24:10.0.2.2/24
```

## Implementation Order

### Phase 1: Core Namespace Management ✓ COMPLETE
1. Create models/netns.go
2. Create services/netns.go (basic CRUD)
3. Create handlers/netns.go
4. Create templates (netns.html, netns_table.html)
5. Add routes to main.go
6. Add navigation link

### Phase 2: Interface Integration ✓ COMPLETE
1. Add MoveInterface, CreateVethPair to service
2. Add namespace parameter to NetlinkService
3. Create netns_interfaces.html partial
4. Add interface management UI to namespace detail

### Phase 3: Firewall Integration ✓ COMPLETE
1. Modify IPTablesService to accept namespace (ListChainsInNamespace, AddRuleInNamespace, etc.)
2. Create namespace-specific firewall page (/netns/{name}/firewall)
3. Create netns_firewall.html and netns_firewall_table.html templates

### Phase 4: Routing Integration ✓ COMPLETE
1. Modify IPRouteService to accept namespace (ListRoutesInNamespace, AddRouteInNamespace, etc.)
2. Modify IPRuleService to accept namespace (ListRulesInNamespace, AddRuleInNamespace, etc.)
3. Create namespace-specific routes/rules pages
4. Create netns_routes.html, netns_routes_table.html, netns_rules.html, netns_rules_table.html

### Phase 5: Tunnel Integration ✓ COMPLETE
1. Modify TunnelService to accept namespace (CreateGRETunnelInNamespace, etc.)
2. Create namespace-specific tunnel page (/netns/{name}/tunnels)
3. Create netns_tunnels.html and netns_tunnels_table.html templates
4. Support GRE, VXLAN, and WireGuard tunnels within namespaces

### Phase 6: Persistence ✓ COMPLETE
1. Implement SaveNamespaces/RestoreNamespaces with per-namespace configs
2. Add to persist.go RestoreAll with netns parameter
3. Update restore script to include namespace restoration
4. Save iptables rules, routes, and IP rules per namespace

## Navigation Update

Add to nav.html under a new "Isolation" or "Namespaces" section:
```
Namespaces
├── Network Namespaces    (/netns)
```

Or add as a top-level item in the main navigation.

## Security Considerations

1. **Namespace names**: Validate names (alphanumeric, dash, underscore only)
2. **Interface isolation**: Warn when moving critical interfaces
3. **Audit logging**: Log all namespace operations
4. **Default namespace protection**: Prevent accidental deletion of critical interfaces from root namespace

## Future Enhancements

1. **Namespace templates**: Pre-configured namespace setups
2. **Traffic monitoring**: Per-namespace bandwidth stats
3. **Namespace cloning**: Duplicate namespace configuration
4. **Inter-namespace routing**: Visual routing topology
5. **Container integration**: Link with Docker/LXC namespaces
