package handlers

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"linuxtorouter/internal/auth"
	"linuxtorouter/internal/middleware"
	"linuxtorouter/internal/models"
	"linuxtorouter/internal/services"

	"github.com/go-chi/chi/v5"
)

type NetnsHandler struct {
	templates       TemplateExecutor
	netnsService    *services.NetnsService
	iptablesService *services.IPTablesService
	routeService    *services.IPRouteService
	ruleService     *services.IPRuleService
	tunnelService   *services.TunnelService
	userService     *auth.UserService
}

func NewNetnsHandler(templates TemplateExecutor, netnsService *services.NetnsService, iptablesService *services.IPTablesService, routeService *services.IPRouteService, ruleService *services.IPRuleService, tunnelService *services.TunnelService, userService *auth.UserService) *NetnsHandler {
	return &NetnsHandler{
		templates:       templates,
		netnsService:    netnsService,
		iptablesService: iptablesService,
		routeService:    routeService,
		ruleService:     ruleService,
		tunnelService:   tunnelService,
		userService:     userService,
	}
}

// List renders the main namespace page
func (h *NetnsHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	namespaces, err := h.netnsService.ListNamespaces()
	if err != nil {
		log.Printf("Failed to list namespaces: %v", err)
		namespaces = []models.NetworkNamespace{}
	}

	data := map[string]interface{}{
		"Title":      "Network Namespaces",
		"ActivePage": "netns",
		"User":       user,
		"Namespaces": namespaces,
	}

	if err := h.templates.ExecuteTemplate(w, "netns.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// GetNamespaces returns the namespace table partial for HTMX
func (h *NetnsHandler) GetNamespaces(w http.ResponseWriter, r *http.Request) {
	namespaces, err := h.netnsService.ListNamespaces()
	if err != nil {
		log.Printf("Failed to list namespaces: %v", err)
		namespaces = []models.NetworkNamespace{}
	}

	data := map[string]interface{}{
		"Namespaces": namespaces,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_table.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ViewNamespace renders the namespace detail page
func (h *NetnsHandler) ViewNamespace(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	ns, err := h.netnsService.GetNamespace(name)
	if err != nil {
		http.Error(w, "Namespace not found", http.StatusNotFound)
		return
	}

	interfaces, _ := h.netnsService.GetNamespaceInterfaceDetails(name)

	data := map[string]interface{}{
		"Title":      "Namespace: " + name,
		"ActivePage": "netns",
		"User":       user,
		"Namespace":  ns,
		"Interfaces": interfaces,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_detail.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// CreateNamespace creates a new namespace
func (h *NetnsHandler) CreateNamespace(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	input := models.NetnsCreateInput{
		Name: strings.TrimSpace(r.FormValue("name")),
	}

	if input.Name == "" {
		h.renderAlert(w, "error", "Namespace name is required")
		return
	}

	if err := h.netnsService.CreateNamespace(input); err != nil {
		log.Printf("Failed to create namespace: %v", err)
		h.renderAlert(w, "error", "Failed to create namespace: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_create", "Name: "+input.Name, getClientIP(r))
	h.renderAlert(w, "success", "Namespace "+input.Name+" created successfully")
}

// DeleteNamespace deletes a namespace
func (h *NetnsHandler) DeleteNamespace(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := h.netnsService.DeleteNamespace(name); err != nil {
		log.Printf("Failed to delete namespace: %v", err)
		h.renderAlert(w, "error", "Failed to delete namespace: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_delete", "Name: "+name, getClientIP(r))
	h.renderAlert(w, "success", "Namespace "+name+" deleted successfully")
}

// GetInterfaces returns interfaces in a namespace (HTMX partial)
func (h *NetnsHandler) GetInterfaces(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	interfaces, err := h.netnsService.GetNamespaceInterfaceDetails(name)
	if err != nil {
		log.Printf("Failed to get namespace interfaces: %v", err)
		interfaces = []models.NetnsInterfaceInfo{}
	}

	ns, _ := h.netnsService.GetNamespace(name)

	data := map[string]interface{}{
		"Namespace":  ns,
		"Interfaces": interfaces,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_interfaces.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// CreateVethPair creates a veth pair
func (h *NetnsHandler) CreateVethPair(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	input := models.VethPairInput{
		Name1:     strings.TrimSpace(r.FormValue("name1")),
		Name2:     strings.TrimSpace(r.FormValue("name2")),
		Namespace: strings.TrimSpace(r.FormValue("namespace")),
		Address1:  strings.TrimSpace(r.FormValue("address1")),
		Address2:  strings.TrimSpace(r.FormValue("address2")),
	}

	if input.Name1 == "" || input.Name2 == "" {
		h.renderAlert(w, "error", "Both interface names are required")
		return
	}

	if input.Namespace == "" {
		h.renderAlert(w, "error", "Target namespace is required")
		return
	}

	if err := h.netnsService.CreateVethPair(input); err != nil {
		log.Printf("Failed to create veth pair: %v", err)
		h.renderAlert(w, "error", "Failed to create veth pair: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_veth_create", "Pair: "+input.Name1+"/"+input.Name2+" Namespace: "+input.Namespace, getClientIP(r))
	h.renderAlert(w, "success", "Veth pair created successfully")
}

// MoveInterface moves an interface to a namespace
func (h *NetnsHandler) MoveInterface(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	input := models.MoveInterfaceInput{
		Interface: strings.TrimSpace(r.FormValue("interface")),
		Namespace: namespace,
	}

	if input.Interface == "" {
		h.renderAlert(w, "error", "Interface name is required")
		return
	}

	if err := h.netnsService.MoveInterface(input); err != nil {
		log.Printf("Failed to move interface: %v", err)
		h.renderAlert(w, "error", "Failed to move interface: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_move_interface", "Interface: "+input.Interface+" Namespace: "+namespace, getClientIP(r))
	h.renderAlert(w, "success", "Interface moved to namespace successfully")
}

// SaveNamespaces saves namespace configurations
func (h *NetnsHandler) SaveNamespaces(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := h.netnsService.SaveNamespaces(); err != nil {
		log.Printf("Failed to save namespaces: %v", err)
		h.renderAlert(w, "error", "Failed to save namespaces: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_save", "", getClientIP(r))
	h.renderAlert(w, "success", "Namespace configurations saved successfully")
}

// SetInterfaceUp brings up an interface in a namespace
func (h *NetnsHandler) SetInterfaceUp(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	ifName := chi.URLParam(r, "iface")

	if err := h.netnsService.SetInterfaceState(namespace, ifName, true); err != nil {
		log.Printf("Failed to bring up interface: %v", err)
		h.renderAlert(w, "error", "Failed to bring up interface: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_iface_up", "Namespace: "+namespace+" Interface: "+ifName, getClientIP(r))
	h.renderAlert(w, "success", "Interface "+ifName+" is now up")
}

// SetInterfaceDown brings down an interface in a namespace
func (h *NetnsHandler) SetInterfaceDown(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	ifName := chi.URLParam(r, "iface")

	if err := h.netnsService.SetInterfaceState(namespace, ifName, false); err != nil {
		log.Printf("Failed to bring down interface: %v", err)
		h.renderAlert(w, "error", "Failed to bring down interface: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_iface_down", "Namespace: "+namespace+" Interface: "+ifName, getClientIP(r))
	h.renderAlert(w, "success", "Interface "+ifName+" is now down")
}

// RemoveInterface removes an interface from a namespace (moves it back or deletes veth)
func (h *NetnsHandler) RemoveInterface(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	ifName := chi.URLParam(r, "iface")

	if err := h.netnsService.RemoveInterface(namespace, ifName); err != nil {
		log.Printf("Failed to remove interface: %v", err)
		h.renderAlert(w, "error", "Failed to remove interface: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_iface_remove", "Namespace: "+namespace+" Interface: "+ifName, getClientIP(r))
	h.renderAlert(w, "success", "Interface "+ifName+" removed from namespace")
}

// ListFirewall renders the firewall page for a namespace
func (h *NetnsHandler) ListFirewall(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")

	ns, err := h.netnsService.GetNamespace(namespace)
	if err != nil {
		http.Error(w, "Namespace not found", http.StatusNotFound)
		return
	}

	table := r.URL.Query().Get("table")
	if table == "" {
		table = "filter"
	}

	chainName := r.URL.Query().Get("chain")

	chains, err := h.iptablesService.ListChainsInNamespace(table, namespace)
	if err != nil {
		log.Printf("Failed to list chains in namespace: %v", err)
		chains = []models.ChainInfo{}
	}

	// Find selected chain if specified
	var selectedChain *models.ChainInfo
	if chainName != "" {
		for i := range chains {
			if chains[i].Name == chainName {
				selectedChain = &chains[i]
				break
			}
		}
	}

	data := map[string]interface{}{
		"Title":             "Firewall - " + namespace,
		"ActivePage":        "netns",
		"User":              user,
		"Namespace":         ns,
		"NamespaceName":     namespace,
		"Table":             table,
		"Chains":            chains,
		"Tables":            []string{"filter", "nat", "mangle", "raw"},
		"SelectedChain":     selectedChain,
		"SelectedChainName": chainName,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_firewall.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// GetFirewallRules returns firewall rules for HTMX refresh
func (h *NetnsHandler) GetFirewallRules(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "name")
	table := r.URL.Query().Get("table")
	chainName := r.URL.Query().Get("chain")

	if table == "" {
		table = "filter"
	}

	var chains []models.ChainInfo
	var selectedChain *models.ChainInfo
	var err error

	if chainName != "" {
		chainInfo, e := h.iptablesService.GetChainInNamespace(table, chainName, namespace)
		if e != nil {
			log.Printf("Failed to get chain: %v", e)
		} else {
			selectedChain = chainInfo
		}
	} else {
		chains, err = h.iptablesService.ListChainsInNamespace(table, namespace)
		if err != nil {
			log.Printf("Failed to list chains: %v", err)
		}
	}

	data := map[string]interface{}{
		"NamespaceName":     namespace,
		"Table":             table,
		"Chains":            chains,
		"SelectedChain":     selectedChain,
		"SelectedChainName": chainName,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_firewall_table.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AddFirewallRule adds a firewall rule in a namespace
func (h *NetnsHandler) AddFirewallRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	position, _ := strconv.Atoi(r.FormValue("position"))

	input := models.FirewallRuleInput{
		Table:         r.FormValue("table"),
		Chain:         r.FormValue("chain"),
		Position:      position,
		Target:        r.FormValue("target"),
		Protocol:      r.FormValue("protocol"),
		Source:        r.FormValue("source"),
		Destination:   r.FormValue("destination"),
		InInterface:   r.FormValue("in_interface"),
		OutInterface:  r.FormValue("out_interface"),
		DPort:         r.FormValue("dport"),
		SPort:         r.FormValue("sport"),
		State:         r.FormValue("state"),
		Comment:       r.FormValue("comment"),
		ToDestination: r.FormValue("to_destination"),
		ToSource:      r.FormValue("to_source"),
	}

	if input.Table == "" {
		input.Table = "filter"
	}

	if input.Chain == "" || input.Target == "" {
		h.renderAlert(w, "error", "Chain and target are required")
		return
	}

	if err := h.iptablesService.AddRuleInNamespace(input, namespace); err != nil {
		log.Printf("Failed to add rule: %v", err)
		h.renderAlert(w, "error", "Failed to add rule: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_firewall_add", "Namespace: "+namespace+" Chain: "+input.Chain, getClientIP(r))
	h.renderAlert(w, "success", "Rule added successfully")
}

// DeleteFirewallRule deletes a firewall rule in a namespace
func (h *NetnsHandler) DeleteFirewallRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	ruleNum, _ := strconv.Atoi(chi.URLParam(r, "num"))
	table := r.URL.Query().Get("table")
	chain := r.URL.Query().Get("chain")

	if table == "" {
		table = "filter"
	}

	if err := h.iptablesService.DeleteRuleInNamespace(table, chain, ruleNum, namespace); err != nil {
		log.Printf("Failed to delete rule: %v", err)
		h.renderAlert(w, "error", "Failed to delete rule: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_firewall_delete", "Namespace: "+namespace+" Chain: "+chain+" Rule: "+strconv.Itoa(ruleNum), getClientIP(r))
	h.renderAlert(w, "success", "Rule deleted successfully")
}

// CreateChain creates a new chain in a namespace
func (h *NetnsHandler) CreateChain(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	table := r.FormValue("table")
	chain := r.FormValue("chain")

	if table == "" {
		table = "filter"
	}

	if chain == "" {
		h.renderAlert(w, "error", "Chain name is required")
		return
	}

	if err := h.iptablesService.CreateChainInNamespace(table, chain, namespace); err != nil {
		log.Printf("Failed to create chain: %v", err)
		h.renderAlert(w, "error", "Failed to create chain: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_chain_create", "Namespace: "+namespace+" Chain: "+chain, getClientIP(r))
	h.renderAlert(w, "success", "Chain created successfully")
}

// DeleteChain deletes a chain in a namespace
func (h *NetnsHandler) DeleteChain(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	chain := chi.URLParam(r, "chain")
	table := r.URL.Query().Get("table")

	if table == "" {
		table = "filter"
	}

	if err := h.iptablesService.DeleteChainInNamespace(table, chain, namespace); err != nil {
		log.Printf("Failed to delete chain: %v", err)
		h.renderAlert(w, "error", "Failed to delete chain: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_chain_delete", "Namespace: "+namespace+" Chain: "+chain, getClientIP(r))
	h.renderAlert(w, "success", "Chain deleted successfully")
}

// SetChainPolicy sets the policy for a chain in a namespace
func (h *NetnsHandler) SetChainPolicy(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	chain := chi.URLParam(r, "chain")
	table := r.FormValue("table")
	policy := r.FormValue("policy")

	if table == "" {
		table = "filter"
	}

	if policy != "ACCEPT" && policy != "DROP" {
		h.renderAlert(w, "error", "Invalid policy: must be ACCEPT or DROP")
		return
	}

	if err := h.iptablesService.SetPolicyInNamespace(table, chain, policy, namespace); err != nil {
		log.Printf("Failed to set policy: %v", err)
		h.renderAlert(w, "error", "Failed to set policy: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_chain_policy", "Namespace: "+namespace+" Chain: "+chain+" Policy: "+policy, getClientIP(r))
	h.renderAlert(w, "success", "Policy set successfully")
}

// ListRoutes renders the routes page for a namespace
func (h *NetnsHandler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")

	ns, err := h.netnsService.GetNamespace(namespace)
	if err != nil {
		http.Error(w, "Namespace not found", http.StatusNotFound)
		return
	}

	table := r.URL.Query().Get("table")
	if table == "" {
		table = "main"
	}

	routes, err := h.routeService.ListRoutesInNamespace(table, namespace)
	if err != nil {
		log.Printf("Failed to list routes in namespace: %v", err)
		routes = []models.Route{}
	}

	tables, _ := h.routeService.GetRoutingTablesForNamespace(namespace)

	interfaces, _ := h.netnsService.GetNamespaceInterfaceDetails(namespace)
	var ifNames []string
	for _, iface := range interfaces {
		ifNames = append(ifNames, iface.Name)
	}

	data := map[string]interface{}{
		"Title":         "Routes - " + namespace,
		"ActivePage":    "netns",
		"User":          user,
		"Namespace":     ns,
		"NamespaceName": namespace,
		"CurrentTable":  table,
		"Tables":        tables,
		"Routes":        routes,
		"Interfaces":    ifNames,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_routes.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// GetRoutes returns routes for HTMX refresh
func (h *NetnsHandler) GetRoutes(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "name")
	table := r.URL.Query().Get("table")

	if table == "" {
		table = "main"
	}

	routes, err := h.routeService.ListRoutesInNamespace(table, namespace)
	if err != nil {
		log.Printf("Failed to list routes: %v", err)
		routes = []models.Route{}
	}

	data := map[string]interface{}{
		"NamespaceName": namespace,
		"CurrentTable":  table,
		"Routes":        routes,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_routes_table.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AddRoute adds a route in a namespace
func (h *NetnsHandler) AddRoute(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	metric, _ := strconv.Atoi(r.FormValue("metric"))

	input := models.RouteInput{
		Destination: r.FormValue("destination"),
		Gateway:     r.FormValue("gateway"),
		Interface:   r.FormValue("interface"),
		Table:       r.FormValue("table"),
		Metric:      metric,
	}

	if input.Destination == "" {
		h.renderAlert(w, "error", "Destination is required")
		return
	}

	if err := h.routeService.AddRouteInNamespace(input, namespace); err != nil {
		log.Printf("Failed to add route: %v", err)
		h.renderAlert(w, "error", "Failed to add route: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_route_add", "Namespace: "+namespace+" Dest: "+input.Destination, getClientIP(r))
	h.renderAlert(w, "success", "Route added successfully")
}

// DeleteRoute deletes a route in a namespace
func (h *NetnsHandler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	input := models.RouteInput{
		Destination: r.FormValue("destination"),
		Gateway:     r.FormValue("gateway"),
		Interface:   r.FormValue("interface"),
		Table:       r.FormValue("table"),
	}

	if err := h.routeService.DeleteRouteInNamespace(input, namespace); err != nil {
		log.Printf("Failed to delete route: %v", err)
		h.renderAlert(w, "error", "Failed to delete route: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_route_delete", "Namespace: "+namespace+" Dest: "+input.Destination, getClientIP(r))
	h.renderAlert(w, "success", "Route deleted successfully")
}

// ListRules renders the IP rules page for a namespace
func (h *NetnsHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")

	ns, err := h.netnsService.GetNamespace(namespace)
	if err != nil {
		http.Error(w, "Namespace not found", http.StatusNotFound)
		return
	}

	rules, err := h.ruleService.ListRulesInNamespace(namespace)
	if err != nil {
		log.Printf("Failed to list rules in namespace: %v", err)
		rules = []models.IPRule{}
	}

	// Use namespace-specific routing tables
	tables, _ := h.routeService.GetRoutingTablesForNamespace(namespace)
	nextID := h.routeService.GetNextAvailableTableIDForNamespace(namespace)

	interfaces, _ := h.netnsService.GetNamespaceInterfaceDetails(namespace)
	var ifNames []string
	for _, iface := range interfaces {
		ifNames = append(ifNames, iface.Name)
	}

	data := map[string]interface{}{
		"Title":         "Policies - " + namespace,
		"ActivePage":    "netns",
		"User":          user,
		"Namespace":     ns,
		"NamespaceName": namespace,
		"Rules":         rules,
		"Tables":        tables,
		"NextID":        nextID,
		"Interfaces":    ifNames,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_rules.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// GetRules returns IP rules for HTMX refresh
func (h *NetnsHandler) GetRules(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "name")

	rules, err := h.ruleService.ListRulesInNamespace(namespace)
	if err != nil {
		log.Printf("Failed to list rules: %v", err)
		rules = []models.IPRule{}
	}

	data := map[string]interface{}{
		"NamespaceName": namespace,
		"Rules":         rules,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_rules_table.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AddRule adds an IP rule in a namespace
func (h *NetnsHandler) AddRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	priority, _ := strconv.Atoi(r.FormValue("priority"))

	input := models.IPRuleInput{
		Priority: priority,
		From:     r.FormValue("from"),
		To:       r.FormValue("to"),
		FWMark:   r.FormValue("fwmark"),
		Table:    r.FormValue("table"),
	}

	if input.Table == "" {
		h.renderAlert(w, "error", "Table is required")
		return
	}

	if err := h.ruleService.AddRuleInNamespace(input, namespace); err != nil {
		log.Printf("Failed to add rule: %v", err)
		h.renderAlert(w, "error", "Failed to add rule: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_rule_add", "Namespace: "+namespace+" Table: "+input.Table, getClientIP(r))
	h.renderAlert(w, "success", "IP rule added successfully")
}

// DeleteRule deletes an IP rule in a namespace
func (h *NetnsHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	priority, _ := strconv.Atoi(chi.URLParam(r, "priority"))

	if err := h.ruleService.DeleteRuleInNamespace(priority, namespace); err != nil {
		log.Printf("Failed to delete rule: %v", err)
		h.renderAlert(w, "error", "Failed to delete rule: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_rule_delete", "Namespace: "+namespace+" Priority: "+strconv.Itoa(priority), getClientIP(r))
	h.renderAlert(w, "success", "IP rule deleted successfully")
}

// GetTables returns routing tables for a namespace (HTMX partial)
func (h *NetnsHandler) GetTables(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "name")

	tables, err := h.routeService.GetRoutingTablesForNamespace(namespace)
	if err != nil {
		tables = []models.RoutingTable{}
	}

	nextID := h.routeService.GetNextAvailableTableIDForNamespace(namespace)

	data := map[string]interface{}{
		"Tables":        tables,
		"NextID":        nextID,
		"NamespaceName": namespace,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_table_list", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// CreateTable creates a routing table for a namespace
func (h *NetnsHandler) CreateTable(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	idStr := r.FormValue("id")
	name := r.FormValue("name")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.renderAlert(w, "error", "Invalid table ID")
		return
	}

	if name == "" {
		h.renderAlert(w, "error", "Table name is required")
		return
	}

	if err := h.routeService.CreateRoutingTableForNamespace(namespace, id, name); err != nil {
		h.renderAlert(w, "error", err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_table_create", "Namespace: "+namespace+" Table: "+name, getClientIP(r))
	h.renderAlertWithTrigger(w, "success", "Routing table '"+name+"' created successfully", "refresh, refreshTables")
}

// DeleteTable deletes a routing table from a namespace
func (h *NetnsHandler) DeleteTable(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	tableName := chi.URLParam(r, "table")

	if err := h.routeService.DeleteRoutingTableForNamespace(namespace, tableName); err != nil {
		h.renderAlert(w, "error", err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_table_delete", "Namespace: "+namespace+" Table: "+tableName, getClientIP(r))
	h.renderAlertWithTrigger(w, "success", "Routing table '"+tableName+"' deleted successfully", "refresh, refreshTables")
}

// GetTableOptions returns table options for a namespace (for dropdowns)
func (h *NetnsHandler) GetTableOptions(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "name")

	tables, err := h.routeService.GetRoutingTablesForNamespace(namespace)
	if err != nil {
		tables = []models.RoutingTable{}
	}

	data := map[string]interface{}{
		"Tables": tables,
	}

	if err := h.templates.ExecuteTemplate(w, "table_options", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// CreateGRETunnel creates a GRE tunnel in a namespace
func (h *NetnsHandler) CreateGRETunnel(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	ttl, _ := strconv.Atoi(r.FormValue("ttl"))

	input := models.GRETunnelInput{
		Name:    r.FormValue("name"),
		Local:   r.FormValue("local"),
		Remote:  r.FormValue("remote"),
		Key:     r.FormValue("key"),
		TTL:     ttl,
		Address: r.FormValue("address"),
	}

	if input.Name == "" || input.Local == "" || input.Remote == "" {
		h.renderAlert(w, "error", "Name, local, and remote addresses are required")
		return
	}

	if err := h.tunnelService.CreateGRETunnelInNamespace(input, namespace); err != nil {
		log.Printf("Failed to create GRE tunnel: %v", err)
		h.renderAlert(w, "error", "Failed to create tunnel: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_gre_create", "Namespace: "+namespace+" Tunnel: "+input.Name, getClientIP(r))
	h.renderAlert(w, "success", "GRE tunnel created successfully")
}

// DeleteGRETunnel deletes a GRE tunnel in a namespace
func (h *NetnsHandler) DeleteGRETunnel(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	tunnelName := chi.URLParam(r, "tunnel")

	if err := h.tunnelService.DeleteGRETunnelInNamespace(tunnelName, namespace); err != nil {
		log.Printf("Failed to delete GRE tunnel: %v", err)
		h.renderAlert(w, "error", "Failed to delete tunnel: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_gre_delete", "Namespace: "+namespace+" Tunnel: "+tunnelName, getClientIP(r))
	h.renderAlert(w, "success", "GRE tunnel deleted successfully")
}

// CreateVXLANTunnel creates a VXLAN tunnel in a namespace
func (h *NetnsHandler) CreateVXLANTunnel(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	vni, _ := strconv.Atoi(r.FormValue("vni"))
	dstPort, _ := strconv.Atoi(r.FormValue("dstport"))

	input := models.VXLANTunnelInput{
		Name:    r.FormValue("name"),
		VNI:     vni,
		Local:   r.FormValue("local"),
		Remote:  r.FormValue("remote"),
		Group:   r.FormValue("group"),
		Dev:     r.FormValue("dev"),
		DstPort: dstPort,
		Address: r.FormValue("address"),
	}

	if input.Name == "" || input.VNI <= 0 {
		h.renderAlert(w, "error", "Name and VNI are required")
		return
	}

	if err := h.tunnelService.CreateVXLANTunnelInNamespace(input, namespace); err != nil {
		log.Printf("Failed to create VXLAN tunnel: %v", err)
		h.renderAlert(w, "error", "Failed to create tunnel: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_vxlan_create", "Namespace: "+namespace+" Tunnel: "+input.Name, getClientIP(r))
	h.renderAlert(w, "success", "VXLAN tunnel created successfully")
}

// DeleteVXLANTunnel deletes a VXLAN tunnel in a namespace
func (h *NetnsHandler) DeleteVXLANTunnel(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	tunnelName := chi.URLParam(r, "tunnel")

	if err := h.tunnelService.DeleteVXLANTunnelInNamespace(tunnelName, namespace); err != nil {
		log.Printf("Failed to delete VXLAN tunnel: %v", err)
		h.renderAlert(w, "error", "Failed to delete tunnel: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_vxlan_delete", "Namespace: "+namespace+" Tunnel: "+tunnelName, getClientIP(r))
	h.renderAlert(w, "success", "VXLAN tunnel deleted successfully")
}

// CreateWireGuardTunnel creates a WireGuard tunnel in a namespace
func (h *NetnsHandler) CreateWireGuardTunnel(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	listenPort, _ := strconv.Atoi(r.FormValue("listen_port"))

	input := models.WireGuardTunnelInput{
		Name:       r.FormValue("name"),
		PrivateKey: r.FormValue("private_key"),
		ListenPort: listenPort,
		Address:    r.FormValue("address"),
	}

	if input.Name == "" {
		h.renderAlert(w, "error", "Tunnel name is required")
		return
	}

	if err := h.tunnelService.CreateWireGuardTunnelInNamespace(input, namespace); err != nil {
		log.Printf("Failed to create WireGuard tunnel: %v", err)
		h.renderAlert(w, "error", "Failed to create tunnel: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_wg_create", "Namespace: "+namespace+" Tunnel: "+input.Name, getClientIP(r))
	h.renderAlert(w, "success", "WireGuard tunnel created successfully")
}

// DeleteWireGuardTunnel deletes a WireGuard tunnel in a namespace
func (h *NetnsHandler) DeleteWireGuardTunnel(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	tunnelName := chi.URLParam(r, "tunnel")

	if err := h.tunnelService.DeleteWireGuardTunnelInNamespace(tunnelName, namespace); err != nil {
		log.Printf("Failed to delete WireGuard tunnel: %v", err)
		h.renderAlert(w, "error", "Failed to delete tunnel: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_wg_delete", "Namespace: "+namespace+" Tunnel: "+tunnelName, getClientIP(r))
	h.renderAlert(w, "success", "WireGuard tunnel deleted successfully")
}

// SetTunnelUp brings up a tunnel in a namespace
func (h *NetnsHandler) SetTunnelUp(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	tunnelName := chi.URLParam(r, "tunnel")

	if err := h.tunnelService.SetInterfaceUpInNamespace(tunnelName, namespace); err != nil {
		log.Printf("Failed to bring up tunnel: %v", err)
		h.renderAlert(w, "error", "Failed to bring up tunnel: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_tunnel_up", "Namespace: "+namespace+" Tunnel: "+tunnelName, getClientIP(r))
	h.renderAlert(w, "success", "Tunnel is now up")
}

// SetTunnelDown brings down a tunnel in a namespace
func (h *NetnsHandler) SetTunnelDown(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	tunnelName := chi.URLParam(r, "tunnel")

	if err := h.tunnelService.SetInterfaceDownInNamespace(tunnelName, namespace); err != nil {
		log.Printf("Failed to bring down tunnel: %v", err)
		h.renderAlert(w, "error", "Failed to bring down tunnel: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_tunnel_down", "Namespace: "+namespace+" Tunnel: "+tunnelName, getClientIP(r))
	h.renderAlert(w, "success", "Tunnel is now down")
}

// ==================== GRE Tunnel Pages ====================

func (h *NetnsHandler) ListGREInNamespace(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")

	ns, err := h.netnsService.GetNamespace(namespace)
	if err != nil {
		http.Error(w, "Namespace not found", http.StatusNotFound)
		return
	}

	tunnels, err := h.tunnelService.ListGRETunnelsInNamespace(namespace)
	if err != nil {
		log.Printf("Failed to list GRE tunnels: %v", err)
		tunnels = []models.GRETunnel{}
	}

	data := map[string]interface{}{
		"Title":         "GRE Tunnels - " + namespace,
		"ActivePage":    "netns",
		"User":          user,
		"Namespace":     ns,
		"NamespaceName": namespace,
		"Tunnels":       tunnels,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_gre.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *NetnsHandler) GetGRETunnelsInNamespace(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "name")

	tunnels, err := h.tunnelService.ListGRETunnelsInNamespace(namespace)
	if err != nil {
		log.Printf("Failed to list GRE tunnels: %v", err)
		tunnels = []models.GRETunnel{}
	}

	data := map[string]interface{}{
		"NamespaceName": namespace,
		"Tunnels":       tunnels,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_gre_table.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ==================== VXLAN Tunnel Pages ====================

func (h *NetnsHandler) ListVXLANInNamespace(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")

	ns, err := h.netnsService.GetNamespace(namespace)
	if err != nil {
		http.Error(w, "Namespace not found", http.StatusNotFound)
		return
	}

	tunnels, err := h.tunnelService.ListVXLANTunnelsInNamespace(namespace)
	if err != nil {
		log.Printf("Failed to list VXLAN tunnels: %v", err)
		tunnels = []models.VXLANTunnel{}
	}

	data := map[string]interface{}{
		"Title":         "VXLAN Tunnels - " + namespace,
		"ActivePage":    "netns",
		"User":          user,
		"Namespace":     ns,
		"NamespaceName": namespace,
		"Tunnels":       tunnels,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_vxlan.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *NetnsHandler) GetVXLANTunnelsInNamespace(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "name")

	tunnels, err := h.tunnelService.ListVXLANTunnelsInNamespace(namespace)
	if err != nil {
		log.Printf("Failed to list VXLAN tunnels: %v", err)
		tunnels = []models.VXLANTunnel{}
	}

	data := map[string]interface{}{
		"NamespaceName": namespace,
		"Tunnels":       tunnels,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_vxlan_table.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ==================== WireGuard Tunnel Pages ====================

func (h *NetnsHandler) ListWireGuardInNamespace(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")

	ns, err := h.netnsService.GetNamespace(namespace)
	if err != nil {
		http.Error(w, "Namespace not found", http.StatusNotFound)
		return
	}

	tunnels, err := h.tunnelService.ListWireGuardTunnelsInNamespace(namespace)
	if err != nil {
		log.Printf("Failed to list WireGuard tunnels: %v", err)
		tunnels = []models.WireGuardTunnel{}
	}

	data := map[string]interface{}{
		"Title":         "WireGuard Tunnels - " + namespace,
		"ActivePage":    "netns",
		"User":          user,
		"Namespace":     ns,
		"NamespaceName": namespace,
		"Tunnels":       tunnels,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_wireguard.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *NetnsHandler) GetWireGuardTunnelsInNamespace(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "name")

	tunnels, err := h.tunnelService.ListWireGuardTunnelsInNamespace(namespace)
	if err != nil {
		log.Printf("Failed to list WireGuard tunnels: %v", err)
		tunnels = []models.WireGuardTunnel{}
	}

	data := map[string]interface{}{
		"NamespaceName": namespace,
		"Tunnels":       tunnels,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_wireguard_table.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *NetnsHandler) ViewWireGuardInNamespace(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	tunnelName := chi.URLParam(r, "tunnel")

	tunnel, err := h.tunnelService.GetWireGuardTunnelInNamespace(tunnelName, namespace)
	if err != nil {
		http.Error(w, "Tunnel not found", http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"Title":         "WireGuard: " + tunnelName,
		"ActivePage":    "netns",
		"User":          user,
		"NamespaceName": namespace,
		"Tunnel":        tunnel,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_wireguard_detail.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *NetnsHandler) UpdateWireGuardInterfaceInNamespace(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	tunnelName := chi.URLParam(r, "tunnel")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	listenPort, _ := strconv.Atoi(r.FormValue("listen_port"))

	input := models.WireGuardInterfaceInput{
		ListenPort: listenPort,
	}

	if err := h.tunnelService.UpdateWireGuardInterfaceInNamespace(tunnelName, namespace, input); err != nil {
		log.Printf("Failed to update WireGuard interface: %v", err)
		h.renderAlert(w, "error", "Failed to update interface: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_wg_interface_update", "Namespace: "+namespace+" Tunnel: "+tunnelName, getClientIP(r))
	h.renderAlert(w, "success", "Interface updated successfully")
}

func (h *NetnsHandler) AddWireGuardPeerInNamespace(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	tunnelName := chi.URLParam(r, "tunnel")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	keepalive, _ := strconv.Atoi(r.FormValue("persistent_keepalive"))

	input := models.WireGuardPeerInput{
		PublicKey:           r.FormValue("public_key"),
		Endpoint:            r.FormValue("endpoint"),
		AllowedIPs:          r.FormValue("allowed_ips"),
		PresharedKey:        r.FormValue("preshared_key"),
		PersistentKeepalive: keepalive,
	}

	if input.PublicKey == "" || input.AllowedIPs == "" {
		h.renderAlert(w, "error", "Public key and allowed IPs are required")
		return
	}

	if err := h.tunnelService.AddWireGuardPeerInNamespace(tunnelName, namespace, input); err != nil {
		log.Printf("Failed to add WireGuard peer: %v", err)
		h.renderAlert(w, "error", "Failed to add peer: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_wg_peer_add", "Namespace: "+namespace+" Tunnel: "+tunnelName, getClientIP(r))
	h.renderAlert(w, "success", "Peer added successfully")
}

func (h *NetnsHandler) RemoveWireGuardPeerInNamespace(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	tunnelName := chi.URLParam(r, "tunnel")

	publicKey := r.URL.Query().Get("public_key")
	if publicKey == "" {
		h.renderAlert(w, "error", "Public key is required")
		return
	}

	if err := h.tunnelService.RemoveWireGuardPeerInNamespace(tunnelName, namespace, publicKey); err != nil {
		log.Printf("Failed to remove WireGuard peer: %v", err)
		h.renderAlert(w, "error", "Failed to remove peer: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_wg_peer_remove", "Namespace: "+namespace+" Tunnel: "+tunnelName, getClientIP(r))
	h.renderAlert(w, "success", "Peer removed successfully")
}

func (h *NetnsHandler) UpdateWireGuardPeerInNamespace(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	tunnelName := chi.URLParam(r, "tunnel")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	keepalive, _ := strconv.Atoi(r.FormValue("persistent_keepalive"))

	input := models.WireGuardPeerInput{
		PublicKey:           r.FormValue("public_key"),
		Endpoint:            r.FormValue("endpoint"),
		AllowedIPs:          r.FormValue("allowed_ips"),
		PersistentKeepalive: keepalive,
	}

	if input.PublicKey == "" || input.AllowedIPs == "" {
		h.renderAlert(w, "error", "Public key and allowed IPs are required")
		return
	}

	if err := h.tunnelService.UpdateWireGuardPeerInNamespace(tunnelName, namespace, input); err != nil {
		log.Printf("Failed to update WireGuard peer: %v", err)
		h.renderAlert(w, "error", "Failed to update peer: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_wg_peer_update", "Namespace: "+namespace+" Tunnel: "+tunnelName, getClientIP(r))
	h.renderAlert(w, "success", "Peer updated successfully")
}

func (h *NetnsHandler) GetWireGuardPeersInNamespace(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "name")
	tunnelName := chi.URLParam(r, "tunnel")

	tunnel, err := h.tunnelService.GetWireGuardTunnelInNamespace(tunnelName, namespace)
	if err != nil {
		log.Printf("Failed to get WireGuard tunnel: %v", err)
	}

	data := map[string]interface{}{
		"NamespaceName": namespace,
		"Tunnel":        tunnel,
	}

	if err := h.templates.ExecuteTemplate(w, "netns_wireguard_peers.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *NetnsHandler) AddWireGuardAddressInNamespace(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	tunnelName := chi.URLParam(r, "tunnel")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	address := strings.TrimSpace(r.FormValue("address"))
	if address == "" {
		h.renderAlert(w, "error", "Address is required")
		return
	}

	if err := h.tunnelService.AddWireGuardAddressInNamespace(tunnelName, namespace, address); err != nil {
		log.Printf("Failed to add WireGuard address: %v", err)
		h.renderAlert(w, "error", "Failed to add address: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_wg_address_add", "Namespace: "+namespace+" Tunnel: "+tunnelName+" Address: "+address, getClientIP(r))
	h.renderAlert(w, "success", "Address added successfully")
}

func (h *NetnsHandler) RemoveWireGuardAddressInNamespace(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")
	tunnelName := chi.URLParam(r, "tunnel")

	address := r.URL.Query().Get("address")
	if address == "" {
		h.renderAlert(w, "error", "Address is required")
		return
	}

	if err := h.tunnelService.RemoveWireGuardAddressInNamespace(tunnelName, namespace, address); err != nil {
		log.Printf("Failed to remove WireGuard address: %v", err)
		h.renderAlert(w, "error", "Failed to remove address: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_wg_address_remove", "Namespace: "+namespace+" Tunnel: "+tunnelName+" Address: "+address, getClientIP(r))
	h.renderAlert(w, "success", "Address removed successfully")
}

// SaveTunnels saves tunnel configurations for a namespace
func (h *NetnsHandler) SaveTunnels(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	namespace := chi.URLParam(r, "name")

	if err := h.tunnelService.SaveTunnelsForNamespace(namespace); err != nil {
		log.Printf("Failed to save tunnels: %v", err)
		h.renderAlert(w, "error", "Failed to save tunnels: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "netns_tunnels_save", "Namespace: "+namespace, getClientIP(r))
	h.renderAlert(w, "success", "Tunnels saved successfully")
}

func (h *NetnsHandler) renderAlert(w http.ResponseWriter, alertType, message string) {
	if alertType == "success" {
		w.Header().Set("HX-Trigger", "refresh")
	}
	data := map[string]interface{}{
		"Type":    alertType,
		"Message": message,
	}
	h.templates.ExecuteTemplate(w, "alert.html", data)
}

func (h *NetnsHandler) renderAlertWithTrigger(w http.ResponseWriter, alertType, message, trigger string) {
	w.Header().Set("HX-Trigger", trigger)
	data := map[string]interface{}{
		"Type":    alertType,
		"Message": message,
	}
	h.templates.ExecuteTemplate(w, "alert.html", data)
}
