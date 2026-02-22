package handlers

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"linuxtorouter/internal/auth"
	"linuxtorouter/internal/middleware"
	"linuxtorouter/internal/services"

	"github.com/go-chi/chi/v5"
)

type SettingsHandler struct {
	templates       TemplateExecutor
	userService     *auth.UserService
	persistService  *services.PersistService
	iptablesService *services.IPTablesService
	routeService    *services.IPRouteService
	ruleService     *services.IPRuleService
	ipsetService    *services.IPSetService
	tunnelService   *services.TunnelService
	netnsService    *services.NetnsService
	dnsmasqService  *services.DnsmasqService
}

func NewSettingsHandler(
	templates TemplateExecutor,
	userService *auth.UserService,
	persistService *services.PersistService,
	iptablesService *services.IPTablesService,
	routeService *services.IPRouteService,
	ruleService *services.IPRuleService,
	ipsetService *services.IPSetService,
	tunnelService *services.TunnelService,
	netnsService *services.NetnsService,
	dnsmasqService *services.DnsmasqService,
) *SettingsHandler {
	return &SettingsHandler{
		templates:       templates,
		userService:     userService,
		persistService:  persistService,
		iptablesService: iptablesService,
		routeService:    routeService,
		ruleService:     ruleService,
		ipsetService:    ipsetService,
		tunnelService:   tunnelService,
		netnsService:    netnsService,
		dnsmasqService:  dnsmasqService,
	}
}

func (h *SettingsHandler) Settings(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	users, err := h.userService.List()
	if err != nil {
		log.Printf("Failed to list users: %v", err)
	}

	auditLogs, err := h.userService.GetAuditLogs(50)
	if err != nil {
		log.Printf("Failed to get audit logs: %v", err)
	}

	data := map[string]interface{}{
		"Title":      "Settings",
		"ActivePage": "settings",
		"User":       user,
		"Users":      users,
		"AuditLogs":  auditLogs,
	}

	if err := h.templates.ExecuteTemplate(w, "settings.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *SettingsHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	currentUser := middleware.GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	isAdmin := r.FormValue("is_admin") == "on"

	if username == "" || password == "" {
		h.renderAlert(w, "error", "Username and password are required")
		return
	}

	if len(password) < 6 {
		h.renderAlert(w, "error", "Password must be at least 6 characters")
		return
	}

	_, err := h.userService.Create(username, password, isAdmin)
	if err != nil {
		if err == auth.ErrUserExists {
			h.renderAlert(w, "error", "Username already exists")
			return
		}
		log.Printf("Failed to create user: %v", err)
		h.renderAlert(w, "error", "Failed to create user")
		return
	}

	h.userService.LogAction(&currentUser.ID, "user_create", "Username: "+username, getClientIP(r))
	h.renderAlert(w, "success", "User "+username+" created successfully")
}

func (h *SettingsHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	currentUser := middleware.GetUser(r)
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.renderAlert(w, "error", "Invalid user ID")
		return
	}

	// Prevent self-deletion
	if id == currentUser.ID {
		h.renderAlert(w, "error", "You cannot delete your own account")
		return
	}

	targetUser, err := h.userService.GetByID(id)
	if err != nil {
		h.renderAlert(w, "error", "User not found")
		return
	}

	if err := h.userService.Delete(id); err != nil {
		log.Printf("Failed to delete user: %v", err)
		h.renderAlert(w, "error", "Failed to delete user")
		return
	}

	h.userService.LogAction(&currentUser.ID, "user_delete", "Username: "+targetUser.Username, getClientIP(r))
	h.renderAlert(w, "success", "User deleted successfully")
}

func (h *SettingsHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	currentUser := middleware.GetUser(r)
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.renderAlert(w, "error", "Invalid user ID")
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	password := r.FormValue("password")
	isAdminStr := r.FormValue("is_admin")

	var passwordPtr *string
	var isAdminPtr *bool

	if password != "" {
		if len(password) < 6 {
			h.renderAlert(w, "error", "Password must be at least 6 characters")
			return
		}
		passwordPtr = &password
	}

	if isAdminStr != "" {
		isAdmin := isAdminStr == "on" || isAdminStr == "true"
		isAdminPtr = &isAdmin
	}

	if err := h.userService.Update(id, passwordPtr, isAdminPtr); err != nil {
		log.Printf("Failed to update user: %v", err)
		h.renderAlert(w, "error", "Failed to update user")
		return
	}

	h.userService.LogAction(&currentUser.ID, "user_update", "User ID: "+idStr, getClientIP(r))
	h.renderAlert(w, "success", "User updated successfully")
}

func (h *SettingsHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	if currentPassword == "" || newPassword == "" {
		h.renderAlert(w, "error", "All fields are required")
		return
	}

	if newPassword != confirmPassword {
		h.renderAlert(w, "error", "New passwords do not match")
		return
	}

	if len(newPassword) < 6 {
		h.renderAlert(w, "error", "Password must be at least 6 characters")
		return
	}

	// Verify current password
	_, err := h.userService.Authenticate(user.Username, currentPassword)
	if err != nil {
		h.renderAlert(w, "error", "Current password is incorrect")
		return
	}

	if err := h.userService.Update(user.ID, &newPassword, nil); err != nil {
		log.Printf("Failed to change password: %v", err)
		h.renderAlert(w, "error", "Failed to change password")
		return
	}

	h.userService.LogAction(&user.ID, "password_change", "", getClientIP(r))
	h.renderAlert(w, "success", "Password changed successfully")
}

func (h *SettingsHandler) ExportConfig(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	archive, err := h.persistService.ExportConfig()
	if err != nil {
		log.Printf("Failed to export config: %v", err)
		http.Error(w, "Failed to export configuration", http.StatusInternalServerError)
		return
	}

	h.userService.LogAction(&user.ID, "config_export", "", getClientIP(r))

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", "attachment; filename=router-config.tar.gz")
	w.Write(archive)
}

func (h *SettingsHandler) ImportConfig(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	file, _, err := r.FormFile("config")
	if err != nil {
		h.renderAlert(w, "error", "Failed to read uploaded file")
		return
	}
	defer file.Close()

	if err := h.persistService.ImportConfig(file); err != nil {
		log.Printf("Failed to import config: %v", err)
		h.renderAlert(w, "error", "Failed to import configuration: "+err.Error())
		return
	}

	// Apply imported configurations immediately
	var restoreErrors []string

	if err := h.netnsService.RestoreNamespaces(); err != nil {
		restoreErrors = append(restoreErrors, "netns: "+err.Error())
	}

	if err := h.ipsetService.RestoreSets(); err != nil {
		restoreErrors = append(restoreErrors, "ipset: "+err.Error())
	}

	if err := h.iptablesService.RestoreRules(); err != nil {
		restoreErrors = append(restoreErrors, "iptables: "+err.Error())
	}

	if err := h.routeService.RestoreRoutes(); err != nil {
		restoreErrors = append(restoreErrors, "routes: "+err.Error())
	}

	if err := h.routeService.RestoreIPForwarding(); err != nil {
		restoreErrors = append(restoreErrors, "ip_forward: "+err.Error())
	}

	if err := h.ruleService.RestoreRules(); err != nil {
		restoreErrors = append(restoreErrors, "rules: "+err.Error())
	}

	if err := h.tunnelService.RestoreTunnels(); err != nil {
		restoreErrors = append(restoreErrors, "tunnels: "+err.Error())
	}

	if err := h.dnsmasqService.RestoreConfig(); err != nil {
		restoreErrors = append(restoreErrors, "dnsmasq: "+err.Error())
	}

	h.userService.LogAction(&user.ID, "config_import", "", getClientIP(r))

	if len(restoreErrors) > 0 {
		h.renderAlert(w, "warning", "Configuration imported but some failed to apply: "+strings.Join(restoreErrors, "; "))
		return
	}

	h.renderAlert(w, "success", "Configuration imported and applied successfully")
}

func (h *SettingsHandler) SaveAll(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	var errors []string

	if err := h.ipsetService.SaveSets(); err != nil {
		errors = append(errors, "ipset: "+err.Error())
	}

	if err := h.iptablesService.SaveRules(); err != nil {
		errors = append(errors, "iptables: "+err.Error())
	}

	if err := h.routeService.SaveRoutes(); err != nil {
		errors = append(errors, "routes: "+err.Error())
	}

	if err := h.routeService.SaveIPForwarding(); err != nil {
		errors = append(errors, "ip_forward: "+err.Error())
	}

	if err := h.ruleService.SaveRules(); err != nil {
		errors = append(errors, "rules: "+err.Error())
	}

	if err := h.tunnelService.SaveTunnels(); err != nil {
		errors = append(errors, "tunnels: "+err.Error())
	}

	if err := h.netnsService.SaveNamespaces(); err != nil {
		errors = append(errors, "netns: "+err.Error())
	}

	if err := h.dnsmasqService.SaveConfig(); err != nil {
		errors = append(errors, "dnsmasq: "+err.Error())
	}

	if len(errors) > 0 {
		h.renderAlert(w, "error", "Some configurations failed to save: "+strings.Join(errors, "; "))
		return
	}

	h.userService.LogAction(&user.ID, "config_save_all", "", getClientIP(r))
	h.renderAlert(w, "success", "All configurations saved successfully")
}

func (h *SettingsHandler) renderAlert(w http.ResponseWriter, alertType, message string) {
	data := map[string]interface{}{
		"Type":    alertType,
		"Message": message,
	}
	h.templates.ExecuteTemplate(w, "alert.html", data)
}
