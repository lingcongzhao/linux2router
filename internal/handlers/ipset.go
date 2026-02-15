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

type IPSetHandler struct {
	templates    TemplateExecutor
	ipsetService *services.IPSetService
	userService  *auth.UserService
}

func NewIPSetHandler(templates TemplateExecutor, ipsetService *services.IPSetService, userService *auth.UserService) *IPSetHandler {
	return &IPSetHandler{
		templates:    templates,
		ipsetService: ipsetService,
		userService:  userService,
	}
}

// List renders the main IPSets page
func (h *IPSetHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	sets, err := h.ipsetService.ListSets()
	if err != nil {
		log.Printf("Failed to list IP sets: %v", err)
		sets = []models.IPSet{}
	}

	data := map[string]interface{}{
		"Title":      "IPSets",
		"ActivePage": "ipset",
		"User":       user,
		"Sets":       sets,
		"SetTypes":   h.ipsetService.GetSetTypes(),
	}

	if err := h.templates.ExecuteTemplate(w, "ipset.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// GetSets returns the IP sets table partial for HTMX
func (h *IPSetHandler) GetSets(w http.ResponseWriter, r *http.Request) {
	sets, err := h.ipsetService.ListSets()
	if err != nil {
		log.Printf("Failed to list IP sets: %v", err)
		h.renderAlert(w, "error", "Failed to list IP sets: "+err.Error())
		return
	}

	data := map[string]interface{}{
		"Sets": sets,
	}

	if err := h.templates.ExecuteTemplate(w, "ipset_table.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ViewSet renders the detail page for a single IP set
func (h *IPSetHandler) ViewSet(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	set, err := h.ipsetService.GetSet(name)
	if err != nil {
		log.Printf("Failed to get IP set %s: %v", name, err)
		http.Error(w, "IP Set not found", http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"Title":      "IPSet: " + name,
		"ActivePage": "ipset",
		"User":       user,
		"Set":        set,
	}

	if err := h.templates.ExecuteTemplate(w, "ipset_detail.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// GetEntries returns the entries table partial for HTMX
func (h *IPSetHandler) GetEntries(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	set, err := h.ipsetService.GetSet(name)
	if err != nil {
		log.Printf("Failed to get IP set %s: %v", name, err)
		h.renderAlert(w, "error", "Failed to get IP set: "+err.Error())
		return
	}

	data := map[string]interface{}{
		"Set": set,
	}

	if err := h.templates.ExecuteTemplate(w, "ipset_entries.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// CreateSet creates a new IP set
func (h *IPSetHandler) CreateSet(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	timeout, _ := strconv.Atoi(r.FormValue("timeout"))
	comment := r.FormValue("comment") == "on"

	input := models.IPSetInput{
		Name:    strings.TrimSpace(r.FormValue("name")),
		Type:    r.FormValue("type"),
		Family:  r.FormValue("family"),
		Timeout: timeout,
		Comment: comment,
	}

	if input.Name == "" || input.Type == "" {
		h.renderAlert(w, "error", "Name and type are required")
		return
	}

	if err := h.ipsetService.CreateSet(input); err != nil {
		log.Printf("Failed to create IP set: %v", err)
		h.renderAlert(w, "error", "Failed to create IP set: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "ipset_create",
		"Name: "+input.Name+", Type: "+input.Type, getClientIP(r))
	h.renderAlert(w, "success", "IP set "+input.Name+" created successfully")
}

// DestroySet deletes an IP set
func (h *IPSetHandler) DestroySet(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := h.ipsetService.DestroySet(name); err != nil {
		log.Printf("Failed to destroy IP set %s: %v", name, err)
		h.renderAlert(w, "error", "Failed to destroy IP set: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "ipset_destroy", "Name: "+name, getClientIP(r))
	h.renderAlert(w, "success", "IP set "+name+" destroyed successfully")
}

// FlushSet removes all entries from an IP set
func (h *IPSetHandler) FlushSet(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := h.ipsetService.FlushSet(name); err != nil {
		log.Printf("Failed to flush IP set %s: %v", name, err)
		h.renderAlert(w, "error", "Failed to flush IP set: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "ipset_flush", "Name: "+name, getClientIP(r))
	h.renderAlert(w, "success", "IP set "+name+" flushed successfully")
}

// AddEntry adds an entry to an IP set
func (h *IPSetHandler) AddEntry(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	timeout, _ := strconv.Atoi(r.FormValue("timeout"))

	input := models.IPSetEntryInput{
		SetName: name,
		Entry:   strings.TrimSpace(r.FormValue("entry")),
		Timeout: timeout,
		Comment: strings.TrimSpace(r.FormValue("comment")),
	}

	if input.Entry == "" {
		h.renderAlert(w, "error", "Entry is required")
		return
	}

	if err := h.ipsetService.AddEntry(input); err != nil {
		log.Printf("Failed to add entry to IP set %s: %v", name, err)
		h.renderAlert(w, "error", "Failed to add entry: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "ipset_add_entry",
		"Set: "+name+", Entry: "+input.Entry, getClientIP(r))
	h.renderAlert(w, "success", "Entry added to "+name)
}

// DeleteEntry removes an entry from an IP set
func (h *IPSetHandler) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	name := chi.URLParam(r, "name")

	if err := r.ParseForm(); err != nil {
		h.renderAlert(w, "error", "Invalid form data")
		return
	}

	entry := r.FormValue("entry")
	if entry == "" {
		h.renderAlert(w, "error", "Entry is required")
		return
	}

	if err := h.ipsetService.DeleteEntry(name, entry); err != nil {
		log.Printf("Failed to delete entry from IP set %s: %v", name, err)
		h.renderAlert(w, "error", "Failed to delete entry: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "ipset_delete_entry",
		"Set: "+name+", Entry: "+entry, getClientIP(r))
	h.renderAlert(w, "success", "Entry deleted from "+name)
}

// SaveSets saves all IP sets to file
func (h *IPSetHandler) SaveSets(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	if err := h.ipsetService.SaveSets(); err != nil {
		log.Printf("Failed to save IP sets: %v", err)
		h.renderAlert(w, "error", "Failed to save IP sets: "+err.Error())
		return
	}

	h.userService.LogAction(&user.ID, "ipset_save", "", getClientIP(r))
	h.renderAlert(w, "success", "IP sets saved successfully")
}

func (h *IPSetHandler) renderAlert(w http.ResponseWriter, alertType, message string) {
	if alertType == "success" {
		w.Header().Set("HX-Trigger", "refresh")
	}
	data := map[string]interface{}{
		"Type":    alertType,
		"Message": message,
	}
	h.templates.ExecuteTemplate(w, "alert.html", data)
}
