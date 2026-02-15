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

type TunnelHandler struct {
	templates     TemplateExecutor
	tunnelService *services.TunnelService
	userService   *auth.UserService
}

func NewTunnelHandler(templates TemplateExecutor, tunnelService *services.TunnelService, userService *auth.UserService) *TunnelHandler {
	return &TunnelHandler{
		templates:     templates,
		tunnelService: tunnelService,
		userService:   userService,
	}
}

// ==================== GRE Tunnels ====================

func (h *TunnelHandler) ListGRE(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	tunnels, err := h.tunnelService.ListGRETunnels()
	if err != nil {
		log.Printf("Failed to list GRE tunnels: %v", err)
		tunnels = []models.GRETunnel{}
	}

	data := map[string]interface{}{
		"Title":      "GRE Tunnels",
		"ActivePage": "tunnels-gre",
		"User":       user,
		"Tunnels":    tunnels,
	}

	if err := h.templates.ExecuteTemplate(w, "tunnels_gre.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *TunnelHandler) GetGRETunnels(w http.ResponseWriter, r *http.Request) {
	tunnels, err := h.tunnelService.ListGRETunnels()
	if err != nil {
		log.Printf("Failed to list GRE tunnels: %v", err)
		tunnels = []models.GRETunnel{}
	}

	data := map[string]interface{}{
		"Tunnels": tunnels,
	}

	if err := h.templates.ExecuteTemplate(w, "gre_table.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *TunnelHandler) CreateGRE(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	ttl, _ := strconv.Atoi(r.FormValue("ttl"))

	input := models.GRETunnelInput{
		Name:    strings.TrimSpace(r.FormValue("name")),
		Local:   strings.TrimSpace(r.FormValue("local")),
		Remote:  strings.TrimSpace(r.FormValue("remote")),
		Key:     strings.TrimSpace(r.FormValue("key")),
		TTL:     ttl,
		Address: strings.TrimSpace(r.FormValue("address")),
	}

	if input.Name == "" {
		h.renderAlert(w, "error", "Tunnel name is required")
		return
	}

	if err := h.tunnelService.CreateGRETunnel(input); err != nil {
		log.Printf("Failed to create GRE tunnel: %v", err)
		h.renderAlert(w, "error", "Failed to create tunnel: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "gre_tunnel_create", "Name: "+input.Name, getClientIP(r))
	h.renderAlert(w, "success", "GRE tunnel "+input.Name+" created successfully")
}

func (h *TunnelHandler) DeleteGRE(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := h.tunnelService.DeleteGRETunnel(name); err != nil {
		log.Printf("Failed to delete GRE tunnel: %v", err)
		h.renderAlert(w, "error", "Failed to delete tunnel: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "gre_tunnel_delete", "Name: "+name, getClientIP(r))
	h.renderAlert(w, "success", "GRE tunnel "+name+" deleted successfully")
}

func (h *TunnelHandler) SetGREUp(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := h.tunnelService.SetInterfaceUp(name); err != nil {
		h.renderAlert(w, "error", "Failed to bring tunnel up: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "gre_tunnel_up", "Name: "+name, getClientIP(r))
	h.renderAlert(w, "success", "Tunnel "+name+" is now UP")
}

func (h *TunnelHandler) SetGREDown(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := h.tunnelService.SetInterfaceDown(name); err != nil {
		h.renderAlert(w, "error", "Failed to bring tunnel down: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "gre_tunnel_down", "Name: "+name, getClientIP(r))
	h.renderAlert(w, "success", "Tunnel "+name+" is now DOWN")
}

// ==================== VXLAN Tunnels ====================

func (h *TunnelHandler) ListVXLAN(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	tunnels, err := h.tunnelService.ListVXLANTunnels()
	if err != nil {
		log.Printf("Failed to list VXLAN tunnels: %v", err)
		tunnels = []models.VXLANTunnel{}
	}

	data := map[string]interface{}{
		"Title":      "VXLAN Tunnels",
		"ActivePage": "tunnels-vxlan",
		"User":       user,
		"Tunnels":    tunnels,
	}

	if err := h.templates.ExecuteTemplate(w, "tunnels_vxlan.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *TunnelHandler) GetVXLANTunnels(w http.ResponseWriter, r *http.Request) {
	tunnels, err := h.tunnelService.ListVXLANTunnels()
	if err != nil {
		log.Printf("Failed to list VXLAN tunnels: %v", err)
		tunnels = []models.VXLANTunnel{}
	}

	data := map[string]interface{}{
		"Tunnels": tunnels,
	}

	if err := h.templates.ExecuteTemplate(w, "vxlan_table.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *TunnelHandler) CreateVXLAN(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	vni, _ := strconv.Atoi(r.FormValue("vni"))
	dstport, _ := strconv.Atoi(r.FormValue("dstport"))

	input := models.VXLANTunnelInput{
		Name:    strings.TrimSpace(r.FormValue("name")),
		VNI:     vni,
		Local:   strings.TrimSpace(r.FormValue("local")),
		Remote:  strings.TrimSpace(r.FormValue("remote")),
		Group:   strings.TrimSpace(r.FormValue("group")),
		DstPort: dstport,
		Dev:     strings.TrimSpace(r.FormValue("dev")),
		MAC:     strings.TrimSpace(r.FormValue("mac")),
		Address: strings.TrimSpace(r.FormValue("address")),
	}

	if input.Name == "" {
		h.renderAlert(w, "error", "Tunnel name is required")
		return
	}

	if err := h.tunnelService.CreateVXLANTunnel(input); err != nil {
		log.Printf("Failed to create VXLAN tunnel: %v", err)
		h.renderAlert(w, "error", "Failed to create tunnel: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "vxlan_tunnel_create", "Name: "+input.Name, getClientIP(r))
	h.renderAlert(w, "success", "VXLAN tunnel "+input.Name+" created successfully")
}

func (h *TunnelHandler) DeleteVXLAN(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := h.tunnelService.DeleteVXLANTunnel(name); err != nil {
		log.Printf("Failed to delete VXLAN tunnel: %v", err)
		h.renderAlert(w, "error", "Failed to delete tunnel: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "vxlan_tunnel_delete", "Name: "+name, getClientIP(r))
	h.renderAlert(w, "success", "VXLAN tunnel "+name+" deleted successfully")
}

func (h *TunnelHandler) SetVXLANUp(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := h.tunnelService.SetInterfaceUp(name); err != nil {
		h.renderAlert(w, "error", "Failed to bring tunnel up: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "vxlan_tunnel_up", "Name: "+name, getClientIP(r))
	h.renderAlert(w, "success", "Tunnel "+name+" is now UP")
}

func (h *TunnelHandler) SetVXLANDown(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := h.tunnelService.SetInterfaceDown(name); err != nil {
		h.renderAlert(w, "error", "Failed to bring tunnel down: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "vxlan_tunnel_down", "Name: "+name, getClientIP(r))
	h.renderAlert(w, "success", "Tunnel "+name+" is now DOWN")
}

// ==================== WireGuard Tunnels ====================

func (h *TunnelHandler) ListWireGuard(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	tunnels, err := h.tunnelService.ListWireGuardTunnels()
	if err != nil {
		log.Printf("Failed to list WireGuard tunnels: %v", err)
		tunnels = []models.WireGuardTunnel{}
	}

	data := map[string]interface{}{
		"Title":      "WireGuard Tunnels",
		"ActivePage": "tunnels-wireguard",
		"User":       user,
		"Tunnels":    tunnels,
	}

	if err := h.templates.ExecuteTemplate(w, "tunnels_wireguard.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *TunnelHandler) GetWireGuardTunnels(w http.ResponseWriter, r *http.Request) {
	tunnels, err := h.tunnelService.ListWireGuardTunnels()
	if err != nil {
		log.Printf("Failed to list WireGuard tunnels: %v", err)
		tunnels = []models.WireGuardTunnel{}
	}

	data := map[string]interface{}{
		"Tunnels": tunnels,
	}

	if err := h.templates.ExecuteTemplate(w, "wireguard_table.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *TunnelHandler) ViewWireGuard(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	tunnel, err := h.tunnelService.GetWireGuardTunnel(name)
	if err != nil {
		http.Error(w, "Tunnel not found", http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"Title":      "WireGuard: " + name,
		"ActivePage": "tunnels-wireguard",
		"User":       user,
		"Tunnel":     tunnel,
	}

	if err := h.templates.ExecuteTemplate(w, "tunnels_wireguard_detail.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *TunnelHandler) CreateWireGuard(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	listenPort, _ := strconv.Atoi(r.FormValue("listen_port"))

	input := models.WireGuardTunnelInput{
		Name:       strings.TrimSpace(r.FormValue("name")),
		PrivateKey: strings.TrimSpace(r.FormValue("private_key")),
		ListenPort: listenPort,
		Address:    strings.TrimSpace(r.FormValue("address")),
	}

	if input.Name == "" {
		h.renderAlert(w, "error", "Tunnel name is required")
		return
	}

	if err := h.tunnelService.CreateWireGuardTunnel(input); err != nil {
		log.Printf("Failed to create WireGuard tunnel: %v", err)
		h.renderAlert(w, "error", "Failed to create tunnel: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "wireguard_tunnel_create", "Name: "+input.Name, getClientIP(r))
	h.renderAlert(w, "success", "WireGuard tunnel "+input.Name+" created successfully")
}

func (h *TunnelHandler) DeleteWireGuard(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := h.tunnelService.DeleteWireGuardTunnel(name); err != nil {
		log.Printf("Failed to delete WireGuard tunnel: %v", err)
		h.renderAlert(w, "error", "Failed to delete tunnel: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "wireguard_tunnel_delete", "Name: "+name, getClientIP(r))
	h.renderAlert(w, "success", "WireGuard tunnel "+name+" deleted successfully")
}

func (h *TunnelHandler) SetWireGuardUp(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := h.tunnelService.SetInterfaceUp(name); err != nil {
		h.renderAlert(w, "error", "Failed to bring tunnel up: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "wireguard_tunnel_up", "Name: "+name, getClientIP(r))
	h.renderAlert(w, "success", "Tunnel "+name+" is now UP")
}

func (h *TunnelHandler) SetWireGuardDown(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := h.tunnelService.SetInterfaceDown(name); err != nil {
		h.renderAlert(w, "error", "Failed to bring tunnel down: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "wireguard_tunnel_down", "Name: "+name, getClientIP(r))
	h.renderAlert(w, "success", "Tunnel "+name+" is now DOWN")
}

func (h *TunnelHandler) AddWireGuardPeer(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	keepalive, _ := strconv.Atoi(r.FormValue("persistent_keepalive"))

	input := models.WireGuardPeerInput{
		Interface:           name,
		PublicKey:           strings.TrimSpace(r.FormValue("public_key")),
		PresharedKey:        strings.TrimSpace(r.FormValue("preshared_key")),
		Endpoint:            strings.TrimSpace(r.FormValue("endpoint")),
		AllowedIPs:          strings.TrimSpace(r.FormValue("allowed_ips")),
		PersistentKeepalive: keepalive,
	}

	if input.PublicKey == "" {
		h.renderAlert(w, "error", "Peer public key is required")
		return
	}

	if err := h.tunnelService.AddWireGuardPeer(input); err != nil {
		log.Printf("Failed to add WireGuard peer: %v", err)
		h.renderAlert(w, "error", "Failed to add peer: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "wireguard_peer_add", "Interface: "+name, getClientIP(r))
	h.renderAlert(w, "success", "Peer added successfully")
}

func (h *TunnelHandler) RemoveWireGuardPeer(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")
	publicKey := r.URL.Query().Get("public_key")

	if publicKey == "" {
		h.renderAlert(w, "error", "Peer public key is required")
		return
	}

	if err := h.tunnelService.RemoveWireGuardPeer(name, publicKey); err != nil {
		log.Printf("Failed to remove WireGuard peer: %v", err)
		h.renderAlert(w, "error", "Failed to remove peer: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "wireguard_peer_remove", "Interface: "+name, getClientIP(r))
	h.renderAlert(w, "success", "Peer removed successfully")
}

func (h *TunnelHandler) UpdateWireGuardInterface(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	listenPort, _ := strconv.Atoi(r.FormValue("listen_port"))
	address := strings.TrimSpace(r.FormValue("address"))

	if err := h.tunnelService.UpdateWireGuardInterface(name, listenPort, address); err != nil {
		log.Printf("Failed to update WireGuard interface: %v", err)
		h.renderAlert(w, "error", "Failed to update interface: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "wireguard_interface_update", "Interface: "+name, getClientIP(r))
	h.renderAlert(w, "success", "Interface configuration updated")
}

func (h *TunnelHandler) UpdateWireGuardPeer(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	keepalive, _ := strconv.Atoi(r.FormValue("persistent_keepalive"))

	input := models.WireGuardPeerInput{
		Interface:           name,
		PublicKey:           strings.TrimSpace(r.FormValue("public_key")),
		Endpoint:            strings.TrimSpace(r.FormValue("endpoint")),
		AllowedIPs:          strings.TrimSpace(r.FormValue("allowed_ips")),
		PersistentKeepalive: keepalive,
	}

	if input.PublicKey == "" {
		h.renderAlert(w, "error", "Peer public key is required")
		return
	}

	if err := h.tunnelService.UpdateWireGuardPeer(input); err != nil {
		log.Printf("Failed to update WireGuard peer: %v", err)
		h.renderAlert(w, "error", "Failed to update peer: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "wireguard_peer_update", "Interface: "+name, getClientIP(r))
	h.renderAlert(w, "success", "Peer configuration updated")
}

func (h *TunnelHandler) AddWireGuardAddress(w http.ResponseWriter, r *http.Request) {
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

	if err := h.tunnelService.AddWireGuardAddress(name, address); err != nil {
		log.Printf("Failed to add WireGuard address: %v", err)
		h.renderAlert(w, "error", "Failed to add address: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "wireguard_address_add", "Interface: "+name+" Address: "+address, getClientIP(r))
	h.renderAlert(w, "success", "Address added successfully")
}

func (h *TunnelHandler) RemoveWireGuardAddress(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")
	address := r.URL.Query().Get("address")

	if address == "" {
		h.renderAlert(w, "error", "Address is required")
		return
	}

	if err := h.tunnelService.RemoveWireGuardAddress(name, address); err != nil {
		log.Printf("Failed to remove WireGuard address: %v", err)
		h.renderAlert(w, "error", "Failed to remove address: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "wireguard_address_remove", "Interface: "+name+" Address: "+address, getClientIP(r))
	h.renderAlert(w, "success", "Address removed successfully")
}

func (h *TunnelHandler) GetWireGuardPeers(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	tunnel, err := h.tunnelService.GetWireGuardTunnel(name)
	if err != nil {
		http.Error(w, "Tunnel not found", http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"Tunnel": tunnel,
	}

	if err := h.templates.ExecuteTemplate(w, "wireguard_peers.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *TunnelHandler) GenerateKeyPair(w http.ResponseWriter, r *http.Request) {
	privateKey, publicKey, err := h.tunnelService.GenerateWireGuardKeyPair()
	if err != nil {
		http.Error(w, "Failed to generate key pair: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"PrivateKey": privateKey,
		"PublicKey":  publicKey,
	}

	if err := h.templates.ExecuteTemplate(w, "wireguard_keypair.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ==================== Save Tunnels ====================

func (h *TunnelHandler) SaveTunnels(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := h.tunnelService.SaveTunnels(); err != nil {
		log.Printf("Failed to save tunnels: %v", err)
		h.renderAlert(w, "error", "Failed to save tunnels: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "tunnels_save", "", getClientIP(r))
	h.renderAlert(w, "success", "Tunnel configurations saved successfully")
}

func (h *TunnelHandler) renderAlert(w http.ResponseWriter, alertType, message string) {
	if alertType == "success" {
		w.Header().Set("HX-Trigger", "refresh")
	}
	data := map[string]interface{}{
		"Type":    alertType,
		"Message": message,
	}
	h.templates.ExecuteTemplate(w, "alert.html", data)
}
