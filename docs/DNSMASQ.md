# Dnsmasq DNS & DHCP Server Documentation

## Table of Contents
1. [Overview](#overview)
2. [Installation](#installation)
3. [DHCP Server Configuration](#dhcp-server-configuration)
4. [DNS Server Configuration](#dns-server-configuration)
5. [Advanced Features](#advanced-features)
6. [Troubleshooting](#troubleshooting)
7. [API Reference](#api-reference)

---

## Overview

The Dnsmasq module provides integrated DNS and DHCP services for your Linux Router. It offers:

- **DHCP Server**: Automatic IP address assignment with static reservations
- **DNS Server**: DNS resolution with caching and custom entries
- **Advanced Domain Routing**: Route specific domains to custom DNS servers
- **IPSet Integration**: Automatically populate IPSets with resolved domain IPs for firewall rules

### Architecture

```
┌─────────────────────────────────────────────┐
│         Linux Router Web GUI                │
├─────────────────────────────────────────────┤
│  DHCP Handler  │  DNS Handler  │  Monitoring│
├─────────────────────────────────────────────┤
│         Dnsmasq Service Layer               │
├─────────────────────────────────────────────┤
│  Config Gen  │  Validation  │  Persistence  │
├─────────────────────────────────────────────┤
│           Dnsmasq Daemon                    │
└─────────────────────────────────────────────┘
```

---

## Installation

### Prerequisites

1. **Install Dnsmasq**:
   ```bash
   sudo apt-get update
   sudo apt-get install dnsmasq
   ```

2. **Stop the default Dnsmasq service** (managed by the router GUI):
   ```bash
   sudo systemctl stop dnsmasq
   sudo systemctl disable dnsmasq
   ```

3. **Ensure the router GUI can manage Dnsmasq**:
   The GUI will start/stop Dnsmasq as needed through systemd.

### Verification

Check that Dnsmasq is installed:
```bash
dnsmasq --version
```

---

## DHCP Server Configuration

### Accessing the DHCP Page

Navigate to **Services > DHCP** in the web interface.

### Basic Configuration

1. **Enable DHCP Server**: Toggle the "Enable DHCP Server" checkbox
2. **Select Interface**: Choose the network interface to serve DHCP on (e.g., `eth0`, `br0`)
3. **Set IP Range**:
   - **Start IP**: First IP address in the DHCP pool (e.g., `192.168.1.100`)
   - **End IP**: Last IP address in the DHCP pool (e.g., `192.168.1.200`)
4. **Lease Time**: How long clients keep their IP addresses (e.g., `12h`, `24h`, `7d`)
5. **Gateway** (Optional): Default gateway for DHCP clients (e.g., `192.168.1.1`)
6. **DNS Servers** (Optional): Comma-separated DNS server IPs for clients (e.g., `8.8.8.8,8.8.4.4`)

### Static DHCP Leases (Reservations)

Reserve specific IP addresses for devices based on MAC address:

1. Click **"+ Add Reservation"**
2. Enter:
   - **MAC Address**: Device MAC address (e.g., `00:11:22:33:44:55`)
   - **IP Address**: Reserved IP (e.g., `192.168.1.50`)
   - **Hostname** (Optional): Friendly name for the device
3. Click **"Add Reservation"**

**Use Cases**:
- Servers that need consistent IP addresses
- Network printers
- IoT devices
- Security cameras

### Viewing Active Leases

The **Active DHCP Leases** table shows:
- MAC Address
- IP Address
- Hostname (if provided by client)
- Lease Expiration Time

Updates automatically every 10 seconds.

### Example Configuration

```
Interface: eth0
Start IP: 192.168.1.100
End IP: 192.168.1.200
Lease Time: 12h
Gateway: 192.168.1.1
DNS Servers: 8.8.8.8,8.8.4.4

Static Leases:
- 00:11:22:33:44:55 → 192.168.1.50 (server1)
- aa:bb:cc:dd:ee:ff → 192.168.1.51 (printer)
```

---

## DNS Server Configuration

### Accessing the DNS Page

Navigate to **Services > DNS** in the web interface.

### Basic Configuration

1. **Enable DNS Server**: Toggle the "Enable DNS Server" checkbox
2. **Listen Port**: DNS port (default: `53`)
3. **Cache Size**: Number of DNS entries to cache (default: `1000`)
4. **Upstream DNS Servers**: Comma-separated DNS servers for external resolution (e.g., `8.8.8.8,1.1.1.1`)

### Custom DNS Hosts

Create local DNS records for internal services:

1. Click **"+ Add Host"**
2. Enter:
   - **Hostname**: Domain name (e.g., `server.local`, `nas.home`)
   - **IP Address**: IP to resolve to (e.g., `192.168.1.100`)
3. Click **"Add Host"**

**Use Cases**:
- Internal web servers
- NAS devices
- Local development environments
- Custom TLDs (e.g., `.local`, `.home`)

### Example Configuration

```
Port: 53
Cache Size: 1000
Upstream Servers: 8.8.8.8,1.1.1.1,9.9.9.9

Custom Hosts:
- server.local → 192.168.1.100
- nas.local → 192.168.1.101
- dev.local → 192.168.1.102
```

---

## Advanced Features

### Domain-Specific DNS Routing

Route specific domains to custom DNS servers and optionally populate IPSets.

#### Use Cases

1. **Corporate VPN DNS**: Route `company.local` to corporate DNS server
2. **Ad Blocking**: Route ad domains to a black hole DNS
3. **Geo-specific Routing**: Use region-specific DNS for certain domains
4. **Content Filtering**: Route domains to filtered DNS servers
5. **Traffic Routing**: Populate IPSets for policy-based routing

#### Configuration

1. Navigate to **Services > DNS**
2. Scroll to **"Advanced Domain Rules"**
3. Click **"+ Add Rule"**
4. Enter:
   - **Domain**: Domain name (e.g., `example.com`, `netflix.com`)
   - **Custom DNS Server** (Optional): DNS server IP to use for this domain
   - **IPSet Name** (Optional): Existing IPSet to populate with resolved IPs
5. Click **"Add Rule"**

#### Example: Route Corporate Traffic

**Scenario**: Route all `company.local` domains to corporate DNS server `10.0.0.1`

```
Domain: company.local
Custom DNS: 10.0.0.1
IPSet Name: (none)
```

All DNS queries for `*.company.local` will be forwarded to `10.0.0.1`.

#### Example: Traffic Routing with IPSet

**Scenario**: Route all Netflix traffic through a specific gateway

**Step 1**: Create an IPSet
```
Navigate to Firewall > IP Sets
Create set: "netflix_ips" (type: hash:ip)
```

**Step 2**: Create Domain Rule
```
Domain: netflix.com
Custom DNS: (leave empty to use default)
IPSet Name: netflix_ips
```

**Step 3**: Create Firewall/Routing Rules
```
Navigate to Routing > Policies
Add rule: "Match IPSet netflix_ips → Route via table vpn"
```

Now all IPs resolved for `netflix.com` automatically populate `netflix_ips` and get routed accordingly!

### Configuration File Location

Dnsmasq configuration is generated at:
```
/etc/dnsmasq.d/linux2router.conf
```

Configuration is saved in JSON format at:
```
<configDir>/dnsmasq/config.json
```

---

## Troubleshooting

### DHCP Issues

#### Clients not getting IP addresses

1. **Check service status**:
   ```bash
   systemctl status dnsmasq
   ```

2. **Verify interface is correct**:
   ```bash
   ip addr show <interface>
   ```

3. **Check for IP conflicts**:
   Ensure DHCP range doesn't overlap with static IPs

4. **View logs**:
   ```bash
   journalctl -u dnsmasq -f
   ```

#### Static leases not working

1. Verify MAC address format (use colons or dashes consistently)
2. Ensure client releases old lease: `sudo dhclient -r <interface>`
3. Check Dnsmasq logs for errors

### DNS Issues

#### DNS not resolving

1. **Test DNS server**:
   ```bash
   nslookup example.com <router-ip>
   ```

2. **Check upstream connectivity**:
   ```bash
   ping 8.8.8.8
   ```

3. **Verify DNS port is listening**:
   ```bash
   sudo netstat -tulpn | grep :53
   ```

#### Domain rules not working

1. Verify IPSet exists: Navigate to **Firewall > IP Sets**
2. Check domain format (should be `example.com`, not `*.example.com`)
3. View Dnsmasq configuration: `cat /etc/dnsmasq.d/linux2router.conf`

### Service Control

**Restart Dnsmasq**:
```bash
sudo systemctl restart dnsmasq
```

**Check configuration syntax**:
```bash
dnsmasq --test
```

**View detailed logs**:
```bash
sudo journalctl -u dnsmasq -n 100 --no-pager
```

---

## API Reference

### DHCP Endpoints

#### Get DHCP Configuration
```http
GET /dhcp/config
```

#### Update DHCP Configuration
```http
POST /dhcp/config
Content-Type: application/x-www-form-urlencoded

enabled=on&interface=eth0&start_ip=192.168.1.100&end_ip=192.168.1.200&lease_time=12h&gateway=192.168.1.1&dns_servers=8.8.8.8
```

#### Add Static Lease
```http
POST /dhcp/static-leases
Content-Type: application/x-www-form-urlencoded

mac=00:11:22:33:44:55&ip=192.168.1.50&hostname=server1
```

#### Remove Static Lease
```http
DELETE /dhcp/static-leases/{mac}
```

#### Get Active DHCP Leases
```http
GET /dhcp/leases
```

### DNS Endpoints

#### Get DNS Configuration
```http
GET /dns/config
```

#### Update DNS Configuration
```http
POST /dns/config
Content-Type: application/x-www-form-urlencoded

enabled=on&port=53&cache_size=1000&upstream_servers=8.8.8.8,1.1.1.1
```

#### Add Custom Host
```http
POST /dns/custom-hosts
Content-Type: application/x-www-form-urlencoded

hostname=server.local&ip=192.168.1.100
```

#### Remove Custom Host
```http
DELETE /dns/custom-hosts/{hostname}
```

#### Add Domain Rule
```http
POST /dns/domain-rules
Content-Type: application/x-www-form-urlencoded

domain=example.com&custom_dns=10.0.0.1&ipset_name=example_ips
```

#### Remove Domain Rule
```http
DELETE /dns/domain-rules/{domain}
```

### Service Control Endpoints

#### Start Service
```http
POST /dhcp/start
POST /dns/start
```

#### Stop Service
```http
POST /dhcp/stop
POST /dns/stop
```

#### Restart Service
```http
POST /dhcp/restart
POST /dns/restart
```

---

## Best Practices

### DHCP

1. **Reserve static IPs outside DHCP range**: If using 192.168.1.100-200 for DHCP, use 192.168.1.2-99 for static IPs
2. **Use meaningful hostnames**: Helps identify devices in lease tables
3. **Set appropriate lease times**: 
   - Short (1-4h) for public WiFi or guest networks
   - Medium (12-24h) for office networks
   - Long (7d+) for stable home networks
4. **Document reservations**: Keep track of which devices have reserved IPs

### DNS

1. **Use multiple upstream servers**: Improves reliability
2. **Adjust cache size based on network size**:
   - Small home network: 150-500
   - Small office: 500-1000
   - Medium office: 1000-5000
3. **Use local TLDs**: `.local`, `.home`, `.lan` for internal services
4. **Test domain rules**: Use `nslookup` to verify rules work as expected

### IPSet Integration

1. **Create IPSets before domain rules**: Validation will fail if IPSet doesn't exist
2. **Use hash:ip type**: Best performance for domain IP lists
3. **Monitor IPSet size**: Large domains (Google, Facebook) can have many IPs
4. **Combine with firewall rules**: IPSets alone don't do anything without firewall/routing rules

---

## Advanced Scenarios

### Scenario 1: Split-Horizon DNS

Resolve internal domains differently than external:

```
Custom Hosts:
- app.example.com → 192.168.1.100 (internal)

Domain Rules:
- example.com → 8.8.8.8 (external resolution)
```

Internal clients get `192.168.1.100` for `app.example.com`, but `example.com` resolves via public DNS.

### Scenario 2: Guest Network Isolation

DHCP configuration for guest network:
```
Interface: guest0
Start IP: 10.20.30.100
End IP: 10.20.30.200
Lease Time: 1h
Gateway: 10.20.30.1
DNS Servers: 8.8.8.8,8.8.4.4
```

No static leases, short lease time, external DNS only.

### Scenario 3: Content Routing

Route streaming services through specific gateway:

```
1. Create IPSets: netflix_ips, youtube_ips, hulu_ips
2. Create Domain Rules:
   - netflix.com → netflix_ips
   - youtube.com → youtube_ips
   - hulu.com → hulu_ips
3. Create Routing Rules:
   - Match netflix_ips → Table streaming_vpn
   - Match youtube_ips → Table streaming_vpn
   - Match hulu_ips → Table streaming_vpn
4. Configure streaming_vpn table with VPN gateway
```

All streaming traffic automatically routes through VPN!

---

## Security Considerations

1. **Disable DHCP on untrusted interfaces**: Only enable on LAN interfaces
2. **Use DNS filtering**: Route ad/malware domains to `0.0.0.0` or filtering DNS
3. **Limit DNS recursion**: Dnsmasq is configured for local network only
4. **Monitor DHCP leases**: Watch for unknown devices
5. **Enable DNSSEC** (if needed): Configure in upstream servers

---

## Performance Tuning

### For Small Networks (<50 devices)
```
Cache Size: 500
Lease Time: 24h
```

### For Medium Networks (50-200 devices)
```
Cache Size: 1000-2000
Lease Time: 12h
```

### For Large Networks (200+ devices)
```
Cache Size: 5000+
Lease Time: 4-12h
Consider multiple DHCP servers with failover
```

---

## Migration from Other DHCP/DNS Servers

### From ISC DHCP Server

1. Export existing static leases from `/etc/dhcp/dhcpd.conf`
2. Convert to static lease format in web GUI
3. Disable ISC DHCP: `sudo systemctl stop isc-dhcp-server`
4. Enable Dnsmasq DHCP in web GUI

### From BIND DNS

1. Export zone files
2. Convert A records to custom hosts in web GUI
3. Configure forwarders as upstream DNS servers
4. Disable BIND: `sudo systemctl stop bind9`
5. Enable Dnsmasq DNS in web GUI

---

## Support

For issues or questions:
- Check logs: `journalctl -u dnsmasq -f`
- Review this documentation
- Check GitHub issues
- Consult Dnsmasq documentation: http://www.thekelleys.org.uk/dnsmasq/doc.html
