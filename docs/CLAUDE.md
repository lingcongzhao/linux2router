# Linux Router GUI - Implementation Plan

## Overview
A web-based GUI application to manage Linux networking components (iptables, ip rules, routing tables, network interfaces) on Ubuntu 24.04.

## Technology Stack

### Backend (Go)
- **Web Framework**: `chi` (lightweight, idiomatic Go router)
- **Session Management**: `gorilla/sessions` with secure cookies
- **Template Engine**: Go's `html/template` with custom functions
- **Database**: SQLite for user accounts and audit logs
- **System Interaction**: Direct command execution via `os/exec` + `vishvananda/netlink` for interface management

### Frontend
- **Rendering**: Go templates (server-side rendered)
- **Interactivity**: HTMX for dynamic updates without full page reloads
- **Styling**: Tailwind CSS (via CDN for simplicity)
- **Icons**: Heroicons (SVG)

## Project Structure

```
linuxtorouter/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── internal/
│   ├── auth/
│   │   ├── session.go           # Session management
│   │   └── user.go              # User CRUD operations
│   ├── handlers/
│   │   ├── auth.go              # Login/logout handlers
│   │   ├── dashboard.go         # Dashboard overview
│   │   ├── firewall.go          # iptables management
│   │   ├── routes.go            # Routing table management
│   │   ├── rules.go             # IP rules (policy routing)
│   │   ├── interfaces.go        # Network interface management
│   │   └── settings.go          # User/system settings
│   ├── middleware/
│   │   └── auth.go              # Authentication middleware
│   ├── models/
│   │   ├── user.go              # User model
│   │   ├── firewall.go          # iptables rule model
│   │   ├── route.go             # Route model
│   │   ├── rule.go              # IP rule model
│   │   └── interface.go         # Network interface model
│   ├── services/
│   │   ├── iptables.go          # iptables command wrapper
│   │   ├── iproute.go           # ip route command wrapper
│   │   ├── iprule.go            # ip rule command wrapper
│   │   ├── netlink.go           # Network interface via netlink
│   │   └── persist.go           # Configuration persistence
│   ├── database/
│   │   └── sqlite.go            # SQLite connection and migrations
│   └── config/
│       └── config.go            # Application configuration
├── web/
│   ├── templates/
│   │   ├── layouts/
│   │   │   └── base.html        # Base layout with nav
│   │   ├── pages/
│   │   │   ├── login.html
│   │   │   ├── dashboard.html
│   │   │   ├── firewall.html
│   │   │   ├── routes.html
│   │   │   ├── rules.html
│   │   │   ├── interfaces.html
│   │   │   └── settings.html
│   │   └── partials/
│   │       ├── nav.html
│   │       ├── firewall_table.html
│   │       ├── route_table.html
│   │       └── alert.html
│   └── static/
│       ├── css/
│       │   └── custom.css
│       └── js/
│           └── app.js           # Minimal JS for HTMX extensions
├── configs/
│   ├── iptables/                # Persisted iptables rules
│   ├── routes/                  # Persisted routes
│   └── rules/                   # Persisted ip rules
├── data/
│   └── router.db                # SQLite database
├── go.mod
├── go.sum
└── Makefile
```

## Core Features

### 1. Authentication System
- Username/password login with bcrypt hashing
- Session-based auth with secure cookies
- Default admin account created on first run
- User management (add/edit/delete users)
- Session timeout and "remember me" option

### 2. Dashboard
- System overview (hostname, uptime, kernel version)
- Network statistics summary
- Quick links to management sections
- Active connections count
- Interface status overview

### 3. Firewall (iptables) Management
- **Tables**: filter, nat, mangle
- **Chains**: INPUT, OUTPUT, FORWARD, PREROUTING, POSTROUTING, custom chains
- **Operations**:
  - List rules with line numbers
  - Add new rules (with form builder for common scenarios)
  - Edit existing rules
  - Delete rules
  - Reorder rules (move up/down)
  - Create/delete custom chains
  - Set chain policy (ACCEPT/DROP)
- **Persistence**: Save to `/etc/iptables/rules.v4` format

### 4. Routing Table Management
- View all routes (main table and custom tables)
- Add static routes (destination, gateway, interface, metric)
- Delete routes
- Support for multiple routing tables
- **Persistence**: Save to config files, restore via systemd service

### 5. IP Rules (Policy Routing)
- List all ip rules with priorities
- Add rules (from/to source, fwmark, table, priority)
- Delete rules
- Support for custom routing tables
- **Persistence**: Save to config files

### 6. Network Interface Management
- List all interfaces with status
- View interface details (IP, MAC, MTU, state, statistics)
- Bring interface up/down
- Configure IP addresses (add/remove)
- Set MTU
- View real-time traffic statistics

### 7. Configuration Persistence
- Save current configuration to files
- Load configuration on system boot
- Systemd service for automatic restoration
- Export/import configuration backup

## API Endpoints

### Authentication
- `GET  /login` - Login page
- `POST /login` - Authenticate
- `POST /logout` - Logout
- `GET  /settings/users` - User management page
- `POST /settings/users` - Create user
- `DELETE /settings/users/{id}` - Delete user

### Dashboard
- `GET /` - Dashboard page
- `GET /api/stats` - System statistics (HTMX partial)

### Firewall
- `GET  /firewall` - Firewall management page
- `GET  /firewall/rules?table=filter&chain=INPUT` - Get rules (HTMX partial)
- `POST /firewall/rules` - Add rule
- `PUT  /firewall/rules/{num}` - Update rule
- `DELETE /firewall/rules/{num}` - Delete rule
- `POST /firewall/rules/{num}/move` - Move rule up/down
- `POST /firewall/chains` - Create chain
- `DELETE /firewall/chains/{name}` - Delete chain
- `PUT  /firewall/chains/{name}/policy` - Set policy
- `POST /firewall/save` - Persist rules to file

### Routes
- `GET  /routes` - Routes page
- `GET  /routes/list?table=main` - Get routes (HTMX partial)
- `POST /routes` - Add route
- `DELETE /routes` - Delete route
- `POST /routes/save` - Persist routes

### IP Rules
- `GET  /rules` - IP rules page
- `GET  /rules/list` - Get rules (HTMX partial)
- `POST /rules` - Add rule
- `DELETE /rules/{priority}` - Delete rule
- `POST /rules/save` - Persist rules

### Interfaces
- `GET  /interfaces` - Interfaces page
- `GET  /interfaces/{name}` - Interface details
- `POST /interfaces/{name}/up` - Bring up
- `POST /interfaces/{name}/down` - Bring down
- `POST /interfaces/{name}/addr` - Add IP address
- `DELETE /interfaces/{name}/addr` - Remove IP address
- `PUT  /interfaces/{name}/mtu` - Set MTU

## Implementation Phases

### Phase 1: Project Setup & Authentication
1. Initialize Go module and project structure
2. Set up chi router and middleware
3. Implement SQLite database with user table
4. Create session management
5. Build login page and auth handlers
6. Create base template layout with navigation

### Phase 2: Dashboard & System Info
1. Create dashboard handler and template
2. Implement system info gathering (uptime, hostname, etc.)
3. Add basic network statistics display

### Phase 3: Network Interfaces
1. Implement netlink-based interface listing
2. Create interface detail view
3. Add up/down, IP address, MTU operations
4. Build interface management UI

### Phase 4: Firewall (iptables)
1. Implement iptables command parser/executor
2. Create rule listing by table/chain
3. Build rule add/edit/delete functionality
4. Implement chain management
5. Add persistence (save/restore)
6. Build complete firewall UI

### Phase 5: Routing Tables
1. Implement ip route command wrapper
2. Create route listing (all tables)
3. Build add/delete route functionality
4. Add persistence
5. Build routes UI

### Phase 6: IP Rules (Policy Routing)
1. Implement ip rule command wrapper
2. Create rule listing
3. Build add/delete functionality
4. Add persistence
5. Build rules UI

### Phase 7: Polish & Persistence
1. Create systemd service for boot restoration
2. Implement configuration export/import
3. Add audit logging
4. Error handling improvements
5. UI polish and responsive design

## Security Considerations

1. **Privilege Escalation**: The server must run as root or with appropriate capabilities to manage networking
2. **Input Validation**: Strict validation of all user inputs before executing system commands
3. **Command Injection Prevention**: Use parameterized commands, never string concatenation
4. **CSRF Protection**: Include CSRF tokens in all forms
5. **Session Security**: Secure, HttpOnly cookies with proper expiration
6. **Rate Limiting**: Protect login endpoint from brute force

## Dependencies (go.mod)

```go
require (
    github.com/go-chi/chi/v5 v5.0.12
    github.com/gorilla/sessions v1.2.2
    github.com/mattn/go-sqlite3 v1.14.22
    github.com/vishvananda/netlink v1.1.0
    golang.org/x/crypto v0.21.0  // bcrypt
)
```

## Verification Plan

1. **Build & Run**: `go build -o router-gui ./cmd/server && sudo ./router-gui`
2. **Test Authentication**:
   - Access http://localhost:8080, verify redirect to login
   - Login with default admin credentials
   - Verify session persistence across page loads
3. **Test Firewall**:
   - View existing iptables rules
   - Add a test rule, verify it appears in `iptables -L`
   - Delete the rule, verify removal
4. **Test Routes**:
   - View routing table, compare with `ip route`
   - Add/delete test routes
5. **Test Interfaces**:
   - View interface list, compare with `ip link`
   - Test up/down operations on a test interface
6. **Test Persistence**:
   - Save configuration
   - Verify files created in configs/
   - Restart service, verify restoration

## Notes

- The application requires root privileges to manage network settings
- Consider running behind a reverse proxy (nginx) for production
- HTTPS should be configured for production use
- Default port: 8090 (configurable via config file or environment variable)
