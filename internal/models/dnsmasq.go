package models

// DnsmasqConfig represents the complete Dnsmasq configuration
type DnsmasqConfig struct {
	DHCP DHCPConfig `json:"dhcp"`
	DNS  DNSConfig  `json:"dns"`
}

// DHCPConfig represents DHCP server configuration
type DHCPConfig struct {
	Enabled      bool          `json:"enabled"`
	Interface    string        `json:"interface"`
	StartIP      string        `json:"start_ip"`
	EndIP        string        `json:"end_ip"`
	LeaseTime    string        `json:"lease_time"` // e.g., "12h", "24h"
	Gateway      string        `json:"gateway"`
	DNSServers   []string      `json:"dns_servers"`
	StaticLeases []StaticLease `json:"static_leases"`
}

// StaticLease represents a DHCP static lease (reservation)
type StaticLease struct {
	MAC      string `json:"mac"`
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
}

// DNSConfig represents DNS server configuration
type DNSConfig struct {
	Enabled         bool         `json:"enabled"`
	Port            int          `json:"port"`
	UpstreamServers []string     `json:"upstream_servers"`
	CacheSize       int          `json:"cache_size"`
	DomainRules     []DomainRule `json:"domain_rules"`
	CustomHosts     []DNSHost    `json:"custom_hosts"`
}

// DomainRule represents domain-specific DNS routing with optional IPSet integration
type DomainRule struct {
	Domain    string `json:"domain"`     // e.g., "example.com"
	CustomDNS string `json:"custom_dns"` // e.g., "8.8.8.8"
	IPSetName string `json:"ipset_name"` // optional: IPSet to populate with domain IPs
}

// DNSHost represents a custom DNS host entry
type DNSHost struct {
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
}

// DHCPLease represents an active DHCP lease
type DHCPLease struct {
	ExpiryTime int64  `json:"expiry_time"` // Unix timestamp
	MAC        string `json:"mac"`
	IP         string `json:"ip"`
	Hostname   string `json:"hostname"`
	ClientID   string `json:"client_id"`
}

// Input structs for API operations

// DHCPConfigInput is used to update DHCP configuration
type DHCPConfigInput struct {
	Enabled    bool     `json:"enabled"`
	Interface  string   `json:"interface"`
	StartIP    string   `json:"start_ip"`
	EndIP      string   `json:"end_ip"`
	LeaseTime  string   `json:"lease_time"`
	Gateway    string   `json:"gateway"`
	DNSServers []string `json:"dns_servers"`
}

// StaticLeaseInput is used to add a static DHCP lease
type StaticLeaseInput struct {
	MAC      string `json:"mac"`
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
}

// DNSConfigInput is used to update DNS configuration
type DNSConfigInput struct {
	Enabled         bool     `json:"enabled"`
	Port            int      `json:"port"`
	UpstreamServers []string `json:"upstream_servers"`
	CacheSize       int      `json:"cache_size"`
}

// DomainRuleInput is used to add a domain-specific DNS rule
type DomainRuleInput struct {
	Domain    string `json:"domain"`
	CustomDNS string `json:"custom_dns"`
	IPSetName string `json:"ipset_name"` // optional
}

// DNSHostInput is used to add a custom DNS host entry
type DNSHostInput struct {
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
}
