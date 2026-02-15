package handlers

import (
	"log"
	"net/http"

	"linuxtorouter/internal/auth"
)

type AuthHandler struct {
	templates   TemplateExecutor
	sessions    *auth.SessionManager
	userService *auth.UserService
}

func NewAuthHandler(templates TemplateExecutor, sessions *auth.SessionManager, userService *auth.UserService) *AuthHandler {
	return &AuthHandler{
		templates:   templates,
		sessions:    sessions,
		userService: userService,
	}
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect to dashboard
	if _, ok := h.sessions.GetUserID(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := map[string]interface{}{
		"Title": "Login",
	}

	if err := h.templates.ExecuteTemplate(w, "login.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderLoginError(w, r, "Invalid form data")
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	remember := r.FormValue("remember") == "on"

	if username == "" || password == "" {
		h.renderLoginError(w, r, "Username and password are required")
		return
	}

	user, err := h.userService.Authenticate(username, password)
	if err != nil {
		h.userService.LogAction(nil, "login_failed", "Username: "+username, getClientIP(r))
		h.renderLoginError(w, r, "Invalid username or password")
		return
	}

	if err := h.sessions.SetUser(w, r, user.ID, user.IsAdmin, remember); err != nil {
		log.Printf("Session error: %v", err)
		h.renderLoginError(w, r, "Failed to create session")
		return
	}

	h.userService.LogAction(&user.ID, "login_success", "", getClientIP(r))

	// Check if this is an HTMX request
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.sessions.GetUserID(r)
	if userID > 0 {
		h.userService.LogAction(&userID, "logout", "", getClientIP(r))
	}

	h.sessions.Clear(w, r)

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *AuthHandler) renderLoginError(w http.ResponseWriter, r *http.Request, message string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		h.templates.ExecuteTemplate(w, "alert.html", map[string]interface{}{
			"Type":    "error",
			"Message": message,
		})
		return
	}

	data := map[string]interface{}{
		"Title": "Login",
		"Error": message,
	}
	h.templates.ExecuteTemplate(w, "login.html", data)
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header for proxy setups
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}
