package handlers

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"linuxtorouter/internal/auth"
	"linuxtorouter/internal/middleware"
	"linuxtorouter/internal/models"
	"linuxtorouter/internal/services"

	"github.com/go-chi/chi/v5"
)

type InterfacesHandler struct {
	templates      TemplateExecutor
	netlinkService *services.NetlinkService
	userService    *auth.UserService
}

func NewInterfacesHandler(templates TemplateExecutor, netlinkService *services.NetlinkService, userService *auth.UserService) *InterfacesHandler {
	return &InterfacesHandler{
		templates:      templates,
		netlinkService: netlinkService,
		userService:    userService,
	}
}

func (h *InterfacesHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	interfaces, err := h.netlinkService.ListInterfaces()
	if err != nil {
		log.Printf("Failed to list interfaces: %v", err)
		interfaces = []models.NetworkInterface{}
	}

	// Add stats to each interface
	type InterfaceWithStats struct {
		models.NetworkInterface
		Stats *models.InterfaceStats
	}

	var interfacesWithStats []InterfaceWithStats
	for _, iface := range interfaces {
		stats, _ := h.netlinkService.GetStats(iface.Name)
		interfacesWithStats = append(interfacesWithStats, InterfaceWithStats{
			NetworkInterface: iface,
			Stats:            stats,
		})
	}

	data := map[string]interface{}{
		"Title":      "Network Interfaces",
		"ActivePage": "interfaces",
		"User":       user,
		"Interfaces": interfacesWithStats,
	}

	if err := h.templates.ExecuteTemplate(w, "interfaces.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *InterfacesHandler) Detail(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	iface, err := h.netlinkService.GetInterface(name)
	if err != nil {
		http.Error(w, "Interface not found", http.StatusNotFound)
		return
	}

	stats, _ := h.netlinkService.GetStats(name)

	data := map[string]interface{}{
		"Title":      "Interface: " + name,
		"ActivePage": "interfaces",
		"User":       user,
		"Interface":  iface,
		"Stats":      stats,
	}

	if err := h.templates.ExecuteTemplate(w, "interface_detail.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *InterfacesHandler) SetUp(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := h.netlinkService.SetInterfaceUp(name); err != nil {
		log.Printf("Failed to bring interface up: %v", err)
		h.renderAlert(w, "error", "Failed to bring interface up: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "interface_up", "Interface: "+name, getClientIP(r))
	h.renderAlert(w, "success", "Interface "+name+" is now UP")
}

func (h *InterfacesHandler) SetDown(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := h.netlinkService.SetInterfaceDown(name); err != nil {
		log.Printf("Failed to bring interface down: %v", err)
		h.renderAlert(w, "error", "Failed to bring interface down: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "interface_down", "Interface: "+name, getClientIP(r))
	h.renderAlert(w, "success", "Interface "+name+" is now DOWN")
}

func (h *InterfacesHandler) AddAddress(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	address := strings.TrimSpace(r.FormValue("address"))
	if address == "" {
		h.renderAlert(w, "error", "Address is required")
		return
	}

	// Validate CIDR format
	if !strings.Contains(address, "/") {
		h.renderAlert(w, "error", "Address must be in CIDR format (e.g., 192.168.1.1/24)")
		return
	}

	if err := h.netlinkService.AddAddress(name, address); err != nil {
		log.Printf("Failed to add address: %v", err)
		h.renderAlert(w, "error", "Failed to add address: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "address_add", "Interface: "+name+", Address: "+address, getClientIP(r))
	h.renderAlert(w, "success", "Address "+address+" added to "+name)
}

func (h *InterfacesHandler) RemoveAddress(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	// Use query parameters for DELETE requests
	address := strings.TrimSpace(r.URL.Query().Get("address"))
	if address == "" {
		h.renderAlert(w, "error", "Address is required")
		return
	}

	if err := h.netlinkService.RemoveAddress(name, address); err != nil {
		log.Printf("Failed to remove address: %v", err)
		h.renderAlert(w, "error", "Failed to remove address: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "address_remove", "Interface: "+name+", Address: "+address, getClientIP(r))
	h.renderAlert(w, "success", "Address "+address+" removed from "+name)
}

func (h *InterfacesHandler) SetMTU(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	mtuStr := strings.TrimSpace(r.FormValue("mtu"))
	mtu, err := strconv.Atoi(mtuStr)
	if err != nil || mtu < 68 || mtu > 65536 {
		h.renderAlert(w, "error", "Invalid MTU value (must be between 68 and 65536)")
		return
	}

	if err := h.netlinkService.SetMTU(name, mtu); err != nil {
		log.Printf("Failed to set MTU: %v", err)
		h.renderAlert(w, "error", "Failed to set MTU: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "mtu_change", "Interface: "+name+", MTU: "+mtuStr, getClientIP(r))
	h.renderAlert(w, "success", "MTU set to "+mtuStr+" on "+name)
}

func (h *InterfacesHandler) GetTable(w http.ResponseWriter, r *http.Request) {
	interfaces, err := h.netlinkService.ListInterfaces()
	if err != nil {
		log.Printf("Failed to list interfaces: %v", err)
		interfaces = []models.NetworkInterface{}
	}

	type InterfaceWithStats struct {
		models.NetworkInterface
		Stats *models.InterfaceStats
	}

	var interfacesWithStats []InterfaceWithStats
	for _, iface := range interfaces {
		stats, _ := h.netlinkService.GetStats(iface.Name)
		interfacesWithStats = append(interfacesWithStats, InterfaceWithStats{
			NetworkInterface: iface,
			Stats:            stats,
		})
	}

	data := map[string]interface{}{
		"Interfaces": interfacesWithStats,
	}

	if err := h.templates.ExecuteTemplate(w, "interface_table.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *InterfacesHandler) renderAlert(w http.ResponseWriter, alertType, message string) {
	if alertType == "success" {
		w.Header().Set("HX-Trigger", "refresh")
	}
	data := map[string]interface{}{
		"Type":    alertType,
		"Message": message,
	}
	h.templates.ExecuteTemplate(w, "alert.html", data)
}

func getInterfaceStatsFromSys(name string) *models.InterfaceStats {
	stats := &models.InterfaceStats{}

	readStat := func(statName string) uint64 {
		data, err := os.ReadFile("/sys/class/net/" + name + "/statistics/" + statName)
		if err != nil {
			return 0
		}
		val, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		return val
	}

	stats.RxBytes = readStat("rx_bytes")
	stats.TxBytes = readStat("tx_bytes")
	stats.RxPackets = readStat("rx_packets")
	stats.TxPackets = readStat("tx_packets")
	stats.RxErrors = readStat("rx_errors")
	stats.TxErrors = readStat("tx_errors")
	stats.RxDropped = readStat("rx_dropped")
	stats.TxDropped = readStat("tx_dropped")

	return stats
}
