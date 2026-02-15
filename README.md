# Linux Router GUI

A comprehensive web-based GUI application for managing Linux networking on Ubuntu 24.04. Built with Go, it provides an intuitive interface to manage firewall rules, routing, tunnels, and network namespaces.

## Features

### Network Management
- **Network Interfaces** - View status, configure IP addresses, set MTU, bring up/down
- **Routing Tables** - Manage static routes across multiple tables (main, local, custom)
- **IP Rules** - Policy-based routing with custom routing tables
- **IP Forwarding** - Toggle IPv4 packet forwarding

### Firewall (iptables)
- **Tables**: filter, nat, mangle, raw
- **Chain Management**: Built-in chains (INPUT, OUTPUT, FORWARD, etc.) + custom chains
- **Rule Operations**: Add, delete, reorder rules with full parameter support
- **Advanced Features**:
  - IPSet matching for efficient rule matching
  - MARK target for packet marking (mangle table)
  - NAT configuration (Source/Destination NAT)
  - Connection state matching (NEW, ESTABLISHED, RELATED, etc.)
  - Chain policy configuration (ACCEPT/DROP)

### Tunnel Support
- **GRE Tunnels** - GRE tunnels with keys and TTL
- **VXLAN Tunnels** - VNI-based Layer 2 virtualization with MAC configuration
- **WireGuard Tunnels** - Modern VPN with peer management and key generation

### Network Namespaces
- Full isolated network stack management
- Per-namespace:
  - Interface management (veth pairs, move interfaces)
  - Firewall rules
  - Routing tables
  - IP rules
  - Tunnels (GRE, VXLAN, WireGuard)
- Persistence across reboots

### IP Sets
- Create/delete IP sets (hash:ip, hash:net, hash:ip,port, etc.)
- Add/remove entries
- Flush sets
- Use in firewall rules with match direction

### System
- **Authentication** - Session-based login with bcrypt hashing
- **User Management** - Admin-only user CRUD
- **Configuration**:
  - Save/restore all settings
  - Export/import configuration archives
  - Boot-time restoration via systemd
- **Audit Logging** - Track all administrative actions

## Requirements

- Ubuntu 24.04 or compatible Linux
- Go 1.22+
- Root privileges (required for network operations)
- CGO enabled

## Installation

### Build from Source

```bash
# Clone and build
cd linuxtorouter
make build

# Or development build
make build-dev
```

### Run the Application

```bash
# Run directly (requires root)
sudo ./build/router-gui

# Development mode
sudo make dev
```

### Install as System Service

```bash
# Install binary and create directories
sudo make install

# Install systemd service
sudo make systemd

# Start service
sudo systemctl start router-gui
sudo systemctl enable router-gui
```

## Default Configuration

| Setting | Default Value |
|---------|---------------|
| Port | 8090 |
| Username | admin |
| Password | admin |
| Data Directory | ./data |
| Config Directory | ./configs |

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| ROUTER_PORT | HTTP server port | 8090 |
| ROUTER_DATA_DIR | Database directory | ./data |
| ROUTER_CONFIG_DIR | Configuration directory | ./configs |
| ROUTER_SESSION_SECRET | Session secret key | (auto-generated) |
| ROUTER_SESSION_MAX_AGE | Session lifetime (seconds) | 86400 |
| ROUTER_DEFAULT_ADMIN | Default admin username | admin |
| ROUTER_DEFAULT_PASSWORD | Default admin password | admin |

## Usage

1. Open browser: `http://localhost:8090`
2. Login with default credentials (admin/admin)
3. Navigate through the menu:

### Navigation Structure

```
Dashboard          - System overview and statistics
Network
├── Interfaces     - Network interface management
├── Namespaces    - Network namespace management
Tunnels
├── GRE           - GRE tunnel configuration
├── VXLAN         - VXLAN tunnel configuration
├── WireGuard     - WireGuard VPN management
Firewall
├── Iptables      - Firewall rule management
├── IP Sets       - IP set management
Routing
├── Routes        - Routing table management
├── Policy        - Policy routing rules
Settings          - User management and configuration
```

## Project Structure

```
linuxtorouter/
├── cmd/server/main.go           # Application entry point
├── internal/
│   ├── auth/                   # Authentication & sessions
│   │   ├── session.go
│   │   └── user.go
│   ├── config/                 # Configuration
│   │   └── config.go
│   ├── database/               # SQLite database
│   │   └── sqlite.go
│   ├── handlers/                # HTTP request handlers
│   │   ├── auth.go
│   │   ├── dashboard.go
│   │   ├── firewall.go
│   │   ├── interfaces.go
│   │   ├── ipset.go
│   │   ├── netns.go
│   │   ├── routes.go
│   │   ├── rules.go
│   │   ├── settings.go
│   │   ├── templates.go
│   │   └── tunnel.go
│   ├── middleware/             # Auth middleware
│   │   └── auth.go
│   ├── models/                  # Data models
│   │   ├── firewall.go
│   │   ├── interface.go
│   │   ├── ipset.go
│   │   ├── netns.go
│   │   ├── route.go
│   │   ├── rule.go
│   │   ├── tunnel.go
│   │   └── user.go
│   └── services/                # System command wrappers
│       ├── iptables.go
│       ├── ipset.go
│       ├── iproute.go
│       ├── iprule.go
│       ├── netlink.go
│       ├── netns.go
│       ├── persist.go
│       └── tunnel.go
├── web/
│   ├── templates/               # HTML templates
│   ├── static/                  # CSS & JavaScript
├── docs/                        # Documentation
├── build/                       # Build output
├── Makefile
└── README.md
```

## Technology Stack

- **Backend**: Go with chi router
- **Frontend**: Server-side rendered templates with HTMX
- **Database**: SQLite (users, audit logs)
- **System APIs**: netlink (interfaces), iproute2 (routes, rules), iptables

## Dependencies

```
github.com/go-chi/chi/v5         - HTTP router
github.com/gorilla/sessions       - Session management
github.com/mattn/go-sqlite3       - SQLite driver
github.com/vishvananda/netlink    - Netlink API
golang.org/x/crypto               - Bcrypt password hashing
```

## Security Notes

- Change default admin password on first login
- The application requires root privileges - use carefully
- Use HTTPS in production (via reverse proxy)
- Session secrets should be customized for production

## License

MIT
