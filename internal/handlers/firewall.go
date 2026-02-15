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

type FirewallHandler struct {
	templates       TemplateExecutor
	iptablesService *services.IPTablesService
	ipsetService    *services.IPSetService
	userService     *auth.UserService
}

func NewFirewallHandler(templates TemplateExecutor, iptablesService *services.IPTablesService, ipsetService *services.IPSetService, userService *auth.UserService) *FirewallHandler {
	return &FirewallHandler{
		templates:       templates,
		iptablesService: iptablesService,
		ipsetService:    ipsetService,
		userService:     userService,
	}
}

func (h *FirewallHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	table := r.URL.Query().Get("table")
	selectedChainName := r.URL.Query().Get("chain")
	if table == "" {
		table = "filter"
	}

	chains, err := h.iptablesService.ListChains(table)
	if err != nil {
		log.Printf("Failed to list chains: %v", err)
		chains = []models.ChainInfo{}
	}

	// Separate system chains from custom chains based on table
	systemChainNames := getSystemChains(table)
	var systemChains []models.ChainInfo
	var selectedChain *models.ChainInfo

	for i := range chains {
		if isSystemChain(chains[i].Name, systemChainNames) {
			systemChains = append(systemChains, chains[i])
		}
		// Check if this is the selected chain (any chain, not just custom)
		if chains[i].Name == selectedChainName {
			selectedChain = &chains[i]
		}
	}

	// Get IPSets for the dropdown
	ipsets, err := h.ipsetService.ListSets()
	if err != nil {
		log.Printf("Failed to list IP sets: %v", err)
		ipsets = []models.IPSet{}
	}

	data := map[string]interface{}{
		"Title":             "Firewall",
		"ActivePage":        "firewall",
		"User":              user,
		"Chains":            chains,
		"SystemChains":      systemChains,
		"SelectedChain":     selectedChain,
		"SelectedChainName": selectedChainName,
		"CurrentTable":      table,
		"Tables":            []string{"filter", "nat", "mangle", "raw"},
		"IPSets":            ipsets,
	}

	if err := h.templates.ExecuteTemplate(w, "firewall.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// getSystemChains returns the built-in chain names for each iptables table
func getSystemChains(table string) []string {
	switch table {
	case "filter":
		return []string{"INPUT", "FORWARD", "OUTPUT"}
	case "nat":
		return []string{"PREROUTING", "INPUT", "OUTPUT", "POSTROUTING"}
	case "mangle":
		return []string{"PREROUTING", "INPUT", "FORWARD", "OUTPUT", "POSTROUTING"}
	case "raw":
		return []string{"PREROUTING", "OUTPUT"}
	default:
		return []string{}
	}
}

// isSystemChain checks if a chain name is a system chain
func isSystemChain(name string, systemChains []string) bool {
	for _, sc := range systemChains {
		if name == sc {
			return true
		}
	}
	return false
}

func (h *FirewallHandler) GetRules(w http.ResponseWriter, r *http.Request) {
	table := r.URL.Query().Get("table")
	selectedChainName := r.URL.Query().Get("chain")

	if table == "" {
		table = "filter"
	}

	chains, err := h.iptablesService.ListChains(table)
	if err != nil {
		log.Printf("Failed to list chains: %v", err)
		h.renderAlert(w, "error", "Failed to get rules: "+err.Error())
		return
	}

	// Separate system chains from custom chains
	systemChainNames := getSystemChains(table)
	var systemChains []models.ChainInfo
	var selectedChain *models.ChainInfo

	for i := range chains {
		if isSystemChain(chains[i].Name, systemChainNames) {
			systemChains = append(systemChains, chains[i])
		}
		if chains[i].Name == selectedChainName {
			selectedChain = &chains[i]
		}
	}

	data := map[string]interface{}{
		"Chains":            chains,
		"SystemChains":      systemChains,
		"SelectedChain":     selectedChain,
		"SelectedChainName": selectedChainName,
		"CurrentTable":      table,
	}

	if err := h.templates.ExecuteTemplate(w, "firewall_table.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *FirewallHandler) AddRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	position, _ := strconv.Atoi(r.FormValue("position"))

	input := models.FirewallRuleInput{
		Table:         r.FormValue("table"),
		Chain:         r.FormValue("chain"),
		Position:      position,
		Protocol:      r.FormValue("protocol"),
		Source:        strings.TrimSpace(r.FormValue("source")),
		Destination:   strings.TrimSpace(r.FormValue("destination")),
		InInterface:   strings.TrimSpace(r.FormValue("in_interface")),
		OutInterface:  strings.TrimSpace(r.FormValue("out_interface")),
		DPort:         strings.TrimSpace(r.FormValue("dport")),
		SPort:         strings.TrimSpace(r.FormValue("sport")),
		Target:        r.FormValue("target"),
		ToDestination: strings.TrimSpace(r.FormValue("to_destination")),
		ToSource:      strings.TrimSpace(r.FormValue("to_source")),
		State:         r.FormValue("state"),
		Comment:       strings.TrimSpace(r.FormValue("comment")),
		IPSet:         strings.TrimSpace(r.FormValue("ipset")),
		IPSetFlags:    r.FormValue("ipset_flags"),
		Mark:          strings.TrimSpace(r.FormValue("mark")),
	}

	if input.Table == "" {
		input.Table = "filter"
	}
	if input.Chain == "" || input.Target == "" {
		h.renderAlert(w, "error", "Chain and target are required")
		return
	}

	if err := h.iptablesService.AddRule(input); err != nil {
		log.Printf("Failed to add rule: %v", err)
		h.renderAlert(w, "error", "Failed to add rule: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "firewall_add_rule",
		"Table: "+input.Table+", Chain: "+input.Chain+", Target: "+input.Target, getClientIP(r))
	h.renderAlert(w, "success", "Rule added successfully")
}

func (h *FirewallHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	ruleNumStr := chi.URLParam(r, "num")
	ruleNum, err := strconv.Atoi(ruleNumStr)
	if err != nil {
		h.renderAlert(w, "error", "Invalid rule number")
		return
	}

	table := r.URL.Query().Get("table")
	chain := r.URL.Query().Get("chain")

	if table == "" {
		table = "filter"
	}
	if chain == "" {
		h.renderAlert(w, "error", "Chain is required")
		return
	}

	if err := h.iptablesService.DeleteRule(table, chain, ruleNum); err != nil {
		log.Printf("Failed to delete rule: %v", err)
		h.renderAlert(w, "error", "Failed to delete rule: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "firewall_delete_rule",
		"Table: "+table+", Chain: "+chain+", Rule: "+ruleNumStr, getClientIP(r))
	h.renderAlert(w, "success", "Rule deleted successfully")
}

func (h *FirewallHandler) MoveRule(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	ruleNumStr := chi.URLParam(r, "num")
	ruleNum, err := strconv.Atoi(ruleNumStr)
	if err != nil {
		h.renderAlert(w, "error", "Invalid rule number")
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	table := r.FormValue("table")
	chain := r.FormValue("chain")
	direction := r.FormValue("direction")

	if table == "" {
		table = "filter"
	}

	var newPos int
	if direction == "up" {
		newPos = ruleNum - 1
	} else {
		newPos = ruleNum + 1
	}

	if newPos < 1 {
		h.renderAlert(w, "error", "Cannot move rule further up")
		return
	}

	if err := h.iptablesService.MoveRule(table, chain, ruleNum, newPos); err != nil {
		log.Printf("Failed to move rule: %v", err)
		h.renderAlert(w, "error", "Failed to move rule: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "firewall_move_rule",
		"Table: "+table+", Chain: "+chain+", From: "+ruleNumStr+", Direction: "+direction, getClientIP(r))
	h.renderAlert(w, "success", "Rule moved successfully")
}

func (h *FirewallHandler) CreateChain(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	table := r.FormValue("table")
	chain := strings.TrimSpace(r.FormValue("chain"))

	if table == "" {
		table = "filter"
	}
	if chain == "" {
		h.renderAlert(w, "error", "Chain name is required")
		return
	}

	if err := h.iptablesService.CreateChain(table, chain); err != nil {
		log.Printf("Failed to create chain: %v", err)
		h.renderAlert(w, "error", "Failed to create chain: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "firewall_create_chain",
		"Table: "+table+", Chain: "+chain, getClientIP(r))
	h.renderAlert(w, "success", "Chain "+chain+" created successfully")
}

func (h *FirewallHandler) DeleteChain(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	chain := chi.URLParam(r, "name")
	table := r.URL.Query().Get("table")

	if table == "" {
		table = "filter"
	}

	if err := h.iptablesService.DeleteChain(table, chain); err != nil {
		log.Printf("Failed to delete chain: %v", err)
		h.renderAlert(w, "error", "Failed to delete chain: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "firewall_delete_chain",
		"Table: "+table+", Chain: "+chain, getClientIP(r))
	h.renderAlert(w, "success", "Chain "+chain+" deleted successfully")
}

func (h *FirewallHandler) SetPolicy(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	chain := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	table := r.FormValue("table")
	policy := r.FormValue("policy")

	if table == "" {
		table = "filter"
	}

	if err := h.iptablesService.SetPolicy(table, chain, policy); err != nil {
		log.Printf("Failed to set policy: %v", err)
		h.renderAlert(w, "error", "Failed to set policy: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "firewall_set_policy",
		"Table: "+table+", Chain: "+chain+", Policy: "+policy, getClientIP(r))
	h.renderAlert(w, "success", "Policy set to "+policy+" for chain "+chain)
}

func (h *FirewallHandler) SaveRules(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := h.iptablesService.SaveRules(); err != nil {
		log.Printf("Failed to save rules: %v", err)
		h.renderAlert(w, "error", "Failed to save rules: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "firewall_save", "", getClientIP(r))
	h.renderAlert(w, "success", "Firewall rules saved successfully")
}

func (h *FirewallHandler) FlushChain(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	table := r.FormValue("table")
	chain := r.FormValue("chain")

	if table == "" {
		table = "filter"
	}

	if err := h.iptablesService.FlushChain(table, chain); err != nil {
		log.Printf("Failed to flush chain: %v", err)
		h.renderAlert(w, "error", "Failed to flush chain: "+err.Error())
		return
	}

	target := "all chains"
	if chain != "" {
		target = "chain " + chain
	}
	h.userService.LogAction(&user.ID, "firewall_flush",
		"Table: "+table+", Chain: "+chain, getClientIP(r))
	h.renderAlert(w, "success", "Flushed "+target+" in "+table+" table")
}

func (h *FirewallHandler) renderAlert(w http.ResponseWriter, alertType, message string) {
	if alertType == "success" {
		w.Header().Set("HX-Trigger", "refresh")
	}
	data := map[string]interface{}{
		"Type":    alertType,
		"Message": message,
	}
	h.templates.ExecuteTemplate(w, "alert.html", data)
}
