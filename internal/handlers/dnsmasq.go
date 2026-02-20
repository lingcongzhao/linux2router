package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"linuxtorouter/internal/auth"
	"linuxtorouter/internal/middleware"
	"linuxtorouter/internal/models"
	"linuxtorouter/internal/services"

	"github.com/go-chi/chi/v5"
)

type DnsmasqHandler struct {
	templates      TemplateExecutor
	dnsmasqService *services.DnsmasqService
	ipsetService   *services.IPSetService
	userService    *auth.UserService
}

func NewDnsmasqHandler(
	templates TemplateExecutor,
	dnsmasqService *services.DnsmasqService,
	ipsetService *services.IPSetService,
	userService *auth.UserService,
) *DnsmasqHandler {
	return &DnsmasqHandler{
		templates:      templates,
		dnsmasqService: dnsmasqService,
		ipsetService:   ipsetService,
		userService:    userService,
	}
}

// DHCP Page and Operations

// DHCPPage renders the DHCP configuration page
func (h *DnsmasqHandler) DHCPPage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	config, err := h.dnsmasqService.GetConfig()
	if err != nil {
		log.Printf("Failed to get Dnsmasq config: %v", err)
		config = &models.DnsmasqConfig{}
	}

	interfaces, err := h.dnsmasqService.GetInterfaces()
	if err != nil {
		log.Printf("Failed to get interfaces: %v", err)
		interfaces = []string{}
	}

	status, err := h.dnsmasqService.GetStatus()
	if err != nil {
		log.Printf("Failed to get Dnsmasq status: %v", err)
		status = "unknown"
	}

	data := map[string]interface{}{
		"Title":      "DHCP Server",
		"ActivePage": "dhcp",
		"User":       user,
		"Config":     config.DHCP,
		"Interfaces": interfaces,
		"Status":     status,
	}

	if err := h.templates.ExecuteTemplate(w, "dhcp.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GetDHCPConfig returns the DHCP configuration as a partial
func (h *DnsmasqHandler) GetDHCPConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.dnsmasqService.GetConfig()
	if err != nil {
		h.renderAlert(w, "error", "Failed to get DHCP config: "+err.Error())
		return
	}

	interfaces, err := h.dnsmasqService.GetInterfaces()
	if err != nil {
		log.Printf("Failed to get interfaces: %v", err)
		interfaces = []string{}
	}

	data := map[string]interface{}{
		"Config":     config.DHCP,
		"Interfaces": interfaces,
	}

	if err := h.templates.ExecuteTemplate(w, "dhcp_config.html", data); err != nil {
		log.Printf("Template error: %v", err)
	}
}

// UpdateDHCPConfig updates the DHCP configuration
func (h *DnsmasqHandler) UpdateDHCPConfig(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	enabled := r.FormValue("enabled") == "on"
	dnsServers := []string{}
	if dnsServersStr := r.FormValue("dns_servers"); dnsServersStr != "" {
		for _, s := range strings.Split(dnsServersStr, ",") {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				dnsServers = append(dnsServers, trimmed)
			}
		}
	}

	input := models.DHCPConfigInput{
		Enabled:    enabled,
		Interface:  strings.TrimSpace(r.FormValue("interface")),
		StartIP:    strings.TrimSpace(r.FormValue("start_ip")),
		EndIP:      strings.TrimSpace(r.FormValue("end_ip")),
		LeaseTime:  strings.TrimSpace(r.FormValue("lease_time")),
		Gateway:    strings.TrimSpace(r.FormValue("gateway")),
		DNSServers: dnsServers,
	}

	if err := h.dnsmasqService.UpdateDHCPConfig(input); err != nil {
		h.renderAlert(w, "error", "Failed to update DHCP config: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "dhcp_update_config",
		"Interface: "+input.Interface, getClientIP(r))

	h.renderAlert(w, "success", "DHCP configuration updated successfully")
}

// GetStaticLeases returns the list of static DHCP leases
func (h *DnsmasqHandler) GetStaticLeases(w http.ResponseWriter, r *http.Request) {
	config, err := h.dnsmasqService.GetConfig()
	if err != nil {
		h.renderAlert(w, "error", "Failed to get static leases: "+err.Error())
		return
	}

	data := map[string]interface{}{
		"Leases": config.DHCP.StaticLeases,
	}

	if err := h.templates.ExecuteTemplate(w, "dhcp_static_leases.html", data); err != nil {
		log.Printf("Template error: %v", err)
	}
}

// AddStaticLease adds a static DHCP lease
func (h *DnsmasqHandler) AddStaticLease(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	input := models.StaticLeaseInput{
		MAC:      strings.TrimSpace(r.FormValue("mac")),
		IP:       strings.TrimSpace(r.FormValue("ip")),
		Hostname: strings.TrimSpace(r.FormValue("hostname")),
	}

	if err := h.dnsmasqService.AddStaticLease(input); err != nil {
		h.renderAlert(w, "error", "Failed to add static lease: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "dhcp_add_static_lease",
		"MAC: "+input.MAC+", IP: "+input.IP, getClientIP(r))

	h.renderAlert(w, "success", "Static lease added successfully")
}

// RemoveStaticLease removes a static DHCP lease
func (h *DnsmasqHandler) RemoveStaticLease(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	mac := chi.URLParam(r, "mac")

	if err := h.dnsmasqService.RemoveStaticLease(mac); err != nil {
		h.renderAlert(w, "error", "Failed to remove static lease: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "dhcp_remove_static_lease",
		"MAC: "+mac, getClientIP(r))

	h.renderAlert(w, "success", "Static lease removed successfully")
}

// GetDHCPLeases returns the list of active DHCP leases
func (h *DnsmasqHandler) GetDHCPLeases(w http.ResponseWriter, r *http.Request) {
	leases, err := h.dnsmasqService.GetDHCPLeases()
	if err != nil {
		h.renderAlert(w, "error", "Failed to get DHCP leases: "+err.Error())
		return
	}

	data := map[string]interface{}{
		"Leases": leases,
	}

	if err := h.templates.ExecuteTemplate(w, "dhcp_active_leases.html", data); err != nil {
		log.Printf("Template error: %v", err)
	}
}

// DNS Page and Operations

// DNSPage renders the DNS configuration page
func (h *DnsmasqHandler) DNSPage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	config, err := h.dnsmasqService.GetConfig()
	if err != nil {
		log.Printf("Failed to get Dnsmasq config: %v", err)
		config = &models.DnsmasqConfig{}
	}

	ipsets, err := h.ipsetService.ListSets()
	if err != nil {
		log.Printf("Failed to get IPSets: %v", err)
		ipsets = []models.IPSet{}
	}

	status, err := h.dnsmasqService.GetStatus()
	if err != nil {
		log.Printf("Failed to get Dnsmasq status: %v", err)
		status = "unknown"
	}

	data := map[string]interface{}{
		"Title":      "DNS Server",
		"ActivePage": "dns",
		"User":       user,
		"Config":     config.DNS,
		"IPSets":     ipsets,
		"Status":     status,
	}

	if err := h.templates.ExecuteTemplate(w, "dns.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GetDNSConfig returns the DNS configuration as a partial
func (h *DnsmasqHandler) GetDNSConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.dnsmasqService.GetConfig()
	if err != nil {
		h.renderAlert(w, "error", "Failed to get DNS config: "+err.Error())
		return
	}

	data := map[string]interface{}{
		"Config": config.DNS,
	}

	if err := h.templates.ExecuteTemplate(w, "dns_config.html", data); err != nil {
		log.Printf("Template error: %v", err)
	}
}

// UpdateDNSConfig updates the DNS configuration
func (h *DnsmasqHandler) UpdateDNSConfig(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	enabled := r.FormValue("enabled") == "on"
	port := 53
	if portStr := r.FormValue("port"); portStr != "" {
		if p, err := parseInt(portStr); err == nil {
			port = p
		}
	}

	cacheSize := 1000
	if cacheSizeStr := r.FormValue("cache_size"); cacheSizeStr != "" {
		if cs, err := parseInt(cacheSizeStr); err == nil {
			cacheSize = cs
		}
	}

	upstreamServers := []string{}
	if upstreamStr := r.FormValue("upstream_servers"); upstreamStr != "" {
		for _, s := range strings.Split(upstreamStr, ",") {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				upstreamServers = append(upstreamServers, trimmed)
			}
		}
	}

	input := models.DNSConfigInput{
		Enabled:         enabled,
		Port:            port,
		UpstreamServers: upstreamServers,
		CacheSize:       cacheSize,
	}

	if err := h.dnsmasqService.UpdateDNSConfig(input); err != nil {
		h.renderAlert(w, "error", "Failed to update DNS config: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "dns_update_config",
		"", getClientIP(r))

	h.renderAlert(w, "success", "DNS configuration updated successfully")
}

// GetDomainRules returns the list of domain-specific DNS rules
func (h *DnsmasqHandler) GetDomainRules(w http.ResponseWriter, r *http.Request) {
	config, err := h.dnsmasqService.GetConfig()
	if err != nil {
		h.renderAlert(w, "error", "Failed to get domain rules: "+err.Error())
		return
	}

	data := map[string]interface{}{
		"Rules": config.DNS.DomainRules,
	}

	if err := h.templates.ExecuteTemplate(w, "dns_domain_rules.html", data); err != nil {
		log.Printf("Template error: %v", err)
	}
}

// AddDomainRule adds a domain-specific DNS rule
func (h *DnsmasqHandler) AddDomainRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	input := models.DomainRuleInput{
		Domain:    strings.TrimSpace(r.FormValue("domain")),
		CustomDNS: strings.TrimSpace(r.FormValue("custom_dns")),
		IPSetName: strings.TrimSpace(r.FormValue("ipset_name")),
	}

	// Validate IPSet if provided
	if input.IPSetName != "" {
		ipsets, err := h.ipsetService.ListSets()
		if err != nil {
			h.renderAlert(w, "error", "Failed to validate IPSet: "+err.Error())
			return
		}

		found := false
		for _, set := range ipsets {
			if set.Name == input.IPSetName {
				found = true
				break
			}
		}

		if !found {
			h.renderAlert(w, "error", "IPSet '"+input.IPSetName+"' does not exist")
			return
		}
	}

	if err := h.dnsmasqService.AddDomainRule(input); err != nil {
		h.renderAlert(w, "error", "Failed to add domain rule: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "dns_add_domain_rule",
		"Domain: "+input.Domain, getClientIP(r))

	h.renderAlert(w, "success", "Domain rule added successfully")
}

// RemoveDomainRule removes a domain-specific DNS rule
func (h *DnsmasqHandler) RemoveDomainRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	domain := chi.URLParam(r, "domain")

	if err := h.dnsmasqService.RemoveDomainRule(domain); err != nil {
		h.renderAlert(w, "error", "Failed to remove domain rule: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "dns_remove_domain_rule",
		"Domain: "+domain, getClientIP(r))

	h.renderAlert(w, "success", "Domain rule removed successfully")
}

// GetCustomHosts returns the list of custom DNS hosts
func (h *DnsmasqHandler) GetCustomHosts(w http.ResponseWriter, r *http.Request) {
	config, err := h.dnsmasqService.GetConfig()
	if err != nil {
		h.renderAlert(w, "error", "Failed to get custom hosts: "+err.Error())
		return
	}

	data := map[string]interface{}{
		"Hosts": config.DNS.CustomHosts,
	}

	if err := h.templates.ExecuteTemplate(w, "dns_custom_hosts.html", data); err != nil {
		log.Printf("Template error: %v", err)
	}
}

// AddCustomHost adds a custom DNS host entry
func (h *DnsmasqHandler) AddCustomHost(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	input := models.DNSHostInput{
		Hostname: strings.TrimSpace(r.FormValue("hostname")),
		IP:       strings.TrimSpace(r.FormValue("ip")),
	}

	if err := h.dnsmasqService.AddDNSHost(input); err != nil {
		h.renderAlert(w, "error", "Failed to add custom host: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "dns_add_custom_host",
		"Hostname: "+input.Hostname+", IP: "+input.IP, getClientIP(r))

	h.renderAlert(w, "success", "Custom host added successfully")
}

// RemoveCustomHost removes a custom DNS host entry
func (h *DnsmasqHandler) RemoveCustomHost(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	hostname := chi.URLParam(r, "hostname")

	if err := h.dnsmasqService.RemoveDNSHost(hostname); err != nil {
		h.renderAlert(w, "error", "Failed to remove custom host: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "dns_remove_custom_host",
		"Hostname: "+hostname, getClientIP(r))

	h.renderAlert(w, "success", "Custom host removed successfully")
}

// Service Control

// RestartService restarts the Dnsmasq service
func (h *DnsmasqHandler) RestartService(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := h.dnsmasqService.Restart(); err != nil {
		h.renderAlert(w, "error", "Failed to restart Dnsmasq: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "dnsmasq_restart",
		"", getClientIP(r))

	h.renderAlertWithRefresh(w, "success", "Dnsmasq service restarted successfully")
}

// StartService starts the Dnsmasq service
func (h *DnsmasqHandler) StartService(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := h.dnsmasqService.Start(); err != nil {
		h.renderAlert(w, "error", "Failed to start Dnsmasq: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "dnsmasq_start",
		"", getClientIP(r))

	h.renderAlertWithRefresh(w, "success", "Dnsmasq service started successfully")
}

// StopService stops the Dnsmasq service
func (h *DnsmasqHandler) StopService(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := h.dnsmasqService.Stop(); err != nil {
		h.renderAlert(w, "error", "Failed to stop Dnsmasq: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "dnsmasq_stop",
		"", getClientIP(r))

	h.renderAlertWithRefresh(w, "success", "Dnsmasq service stopped successfully")
}

// GetServiceStatus returns the current service status
func (h *DnsmasqHandler) GetServiceStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.dnsmasqService.GetStatus()
	if err != nil {
		h.renderAlert(w, "error", "Failed to get service status: "+err.Error())
		return
	}

	data := map[string]interface{}{
		"Status": status,
	}

	if err := h.templates.ExecuteTemplate(w, "dnsmasq_status.html", data); err != nil {
		log.Printf("Template error: %v", err)
	}
}

// GetDHCPStatus returns the DHCP status partial
func (h *DnsmasqHandler) GetDHCPStatus(w http.ResponseWriter, r *http.Request) {
	config, err := h.dnsmasqService.GetConfig()
	if err != nil {
		log.Printf("Failed to get Dnsmasq config: %v", err)
		config = &models.DnsmasqConfig{}
	}

	status, err := h.dnsmasqService.GetStatus()
	if err != nil {
		log.Printf("Failed to get Dnsmasq status: %v", err)
		status = "unknown"
	}

	data := map[string]interface{}{
		"Config": config.DHCP,
		"Status": status,
	}

	if err := h.templates.ExecuteTemplate(w, "dhcp_status.html", data); err != nil {
		log.Printf("Template error: %v", err)
	}
}

// GetDNSStatus returns the DNS status partial
func (h *DnsmasqHandler) GetDNSStatus(w http.ResponseWriter, r *http.Request) {
	config, err := h.dnsmasqService.GetConfig()
	if err != nil {
		log.Printf("Failed to get Dnsmasq config: %v", err)
		config = &models.DnsmasqConfig{}
	}

	status, err := h.dnsmasqService.GetStatus()
	if err != nil {
		log.Printf("Failed to get Dnsmasq status: %v", err)
		status = "unknown"
	}

	data := map[string]interface{}{
		"Config": config.DNS,
		"Status": status,
	}

	if err := h.templates.ExecuteTemplate(w, "dns_status.html", data); err != nil {
		log.Printf("Template error: %v", err)
	}
}

// GetDNSControls returns the DNS control buttons partial
func (h *DnsmasqHandler) GetDNSControls(w http.ResponseWriter, r *http.Request) {
	status, err := h.dnsmasqService.GetStatus()
	if err != nil {
		log.Printf("Failed to get Dnsmasq status: %v", err)
		status = "unknown"
	}

	data := map[string]interface{}{
		"Status": status,
	}

	if err := h.templates.ExecuteTemplate(w, "dns_controls.html", data); err != nil {
		log.Printf("Template error: %v", err)
	}
}

// GetDHCPControls returns the DHCP control buttons partial
func (h *DnsmasqHandler) GetDHCPControls(w http.ResponseWriter, r *http.Request) {
	status, err := h.dnsmasqService.GetStatus()
	if err != nil {
		log.Printf("Failed to get Dnsmasq status: %v", err)
		status = "unknown"
	}

	data := map[string]interface{}{
		"Status": status,
	}

	if err := h.templates.ExecuteTemplate(w, "dhcp_controls.html", data); err != nil {
		log.Printf("Template error: %v", err)
	}
}

// Monitoring and Logs

// GetDNSQueryLogs returns recent DNS query logs
func (h *DnsmasqHandler) GetDNSQueryLogs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := parseInt(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	logs, err := h.dnsmasqService.GetDNSQueryLogs(limit)
	if err != nil {
		h.renderAlert(w, "error", "Failed to get DNS query logs: "+err.Error())
		return
	}

	data := map[string]interface{}{
		"Logs": logs,
	}

	if err := h.templates.ExecuteTemplate(w, "dns_query_logs.html", data); err != nil {
		log.Printf("Template error: %v", err)
	}
}

// GetDNSStatistics returns DNS query statistics
func (h *DnsmasqHandler) GetDNSStatistics(w http.ResponseWriter, r *http.Request) {
	stats, err := h.dnsmasqService.GetDNSStatistics()
	if err != nil {
		h.renderAlert(w, "error", "Failed to get DNS statistics: "+err.Error())
		return
	}

	data := map[string]interface{}{
		"Stats": stats,
	}

	if err := h.templates.ExecuteTemplate(w, "dns_statistics.html", data); err != nil {
		log.Printf("Template error: %v", err)
	}
}

// Helper methods

func (h *DnsmasqHandler) renderAlert(w http.ResponseWriter, alertType, message string) {
	if alertType == "success" {
		w.Header().Set("HX-Trigger", "refresh")
	}
	data := map[string]interface{}{
		"Type":    alertType,
		"Message": message,
	}
	if err := h.templates.ExecuteTemplate(w, "alert.html", data); err != nil {
		log.Printf("Alert template error: %v", err)
	}
}

func (h *DnsmasqHandler) renderAlertWithRefresh(w http.ResponseWriter, alertType, message string) {
	// Trigger both refresh and refreshStatus events with delays (longer delay for service to start/stop)
	w.Header().Set("HX-Trigger", `{"refresh": {"delay": 3000}, "refreshStatus": {"delay": 3000}}`)
	data := map[string]interface{}{
		"Type":    alertType,
		"Message": message,
	}
	if err := h.templates.ExecuteTemplate(w, "alert.html", data); err != nil {
		log.Printf("Alert template error: %v", err)
	}
}

func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
