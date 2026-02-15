package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"linuxtorouter/internal/auth"
	"linuxtorouter/internal/config"
	"linuxtorouter/internal/database"
	"linuxtorouter/internal/handlers"
	"linuxtorouter/internal/middleware"
	"linuxtorouter/internal/services"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// TemplateRegistry holds separate template instances for each page
type TemplateRegistry struct {
	templates map[string]*template.Template
	funcMap   template.FuncMap
}

func NewTemplateRegistry(funcMap template.FuncMap) *TemplateRegistry {
	return &TemplateRegistry{
		templates: make(map[string]*template.Template),
		funcMap:   funcMap,
	}
}

func (tr *TemplateRegistry) Add(name string, tmpl *template.Template) {
	tr.templates[name] = tmpl
}

func (tr *TemplateRegistry) ExecuteTemplate(w io.Writer, name string, data interface{}) error {
	// First try direct lookup in registry
	tmpl, ok := tr.templates[name]
	if ok {
		// For partial templates, the file might define a template without .html extension
		// Check if there's a defined template matching the name without .html
		if strings.HasSuffix(name, ".html") {
			baseName := strings.TrimSuffix(name, ".html")
			if lookup := tmpl.Lookup(baseName); lookup != nil {
				return lookup.Execute(w, data)
			}
		}
		// For page templates, execute the template named in the file
		return tmpl.ExecuteTemplate(w, name, data)
	}

	// For partial templates, the registry key might be different from the template name
	// Try to find a template that contains the requested define
	for _, t := range tr.templates {
		if lookup := t.Lookup(name); lookup != nil {
			return lookup.Execute(w, data)
		}
	}

	return fmt.Errorf("template %s not found", name)
}

func main() {
	// Load configuration
	cfg := config.Load()

	// Determine web directory
	webDir := getWebDir()
	log.Printf("Using web directory: %s", webDir)

	// Initialize database
	db, err := database.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize services
	userService := auth.NewUserService(db)
	sessionManager := auth.NewSessionManager(cfg.SessionSecret, cfg.SessionMaxAge)
	netlinkService := services.NewNetlinkService()
	iptablesService := services.NewIPTablesService(cfg.ConfigDir)
	routeService := services.NewIPRouteService(cfg.ConfigDir)
	ruleService := services.NewIPRuleService(cfg.ConfigDir)
	ipsetService := services.NewIPSetService(cfg.ConfigDir)
	tunnelService := services.NewTunnelService(cfg.ConfigDir)
	netnsService := services.NewNetnsService(cfg.ConfigDir)
	persistService := services.NewPersistService(cfg.ConfigDir)

	// Ensure default admin user exists
	if err := userService.EnsureDefaultAdmin(cfg.DefaultAdmin, cfg.DefaultPassword); err != nil {
		log.Printf("Warning: Failed to create default admin: %v", err)
	}

	// Restore saved configurations
	if err := persistService.RestoreAll(iptablesService, routeService, ruleService, ipsetService, netnsService); err != nil {
		log.Printf("Warning: Failed to restore some configurations: %v", err)
	}

	// Load templates
	templates, err := loadTemplates(filepath.Join(webDir, "templates"))
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(templates, sessionManager, userService)
	dashboardHandler := handlers.NewDashboardHandler(templates, netlinkService)
	interfacesHandler := handlers.NewInterfacesHandler(templates, netlinkService, userService)
	firewallHandler := handlers.NewFirewallHandler(templates, iptablesService, ipsetService, userService)
	routesHandler := handlers.NewRoutesHandler(templates, routeService, netlinkService, userService)
	rulesHandler := handlers.NewRulesHandler(templates, ruleService, routeService, netlinkService, userService)
	ipsetHandler := handlers.NewIPSetHandler(templates, ipsetService, userService)
	tunnelHandler := handlers.NewTunnelHandler(templates, tunnelService, userService)
	netnsHandler := handlers.NewNetnsHandler(templates, netnsService, iptablesService, routeService, ruleService, tunnelService, userService)
	settingsHandler := handlers.NewSettingsHandler(templates, userService, persistService, iptablesService, routeService, ruleService, ipsetService, tunnelService)

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(sessionManager, userService)

	// Setup router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)

	// Static files
	staticDir := filepath.Join(webDir, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	// Public routes
	r.Get("/login", authHandler.LoginPage)
	r.Post("/login", authHandler.Login)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.RequireAuth)

		// Logout
		r.Post("/logout", authHandler.Logout)

		// Dashboard
		r.Get("/", dashboardHandler.Dashboard)
		r.Get("/api/stats", dashboardHandler.Stats)

		// Interfaces
		r.Get("/interfaces", interfacesHandler.List)
		r.Get("/interfaces/table", interfacesHandler.GetTable)
		r.Get("/interfaces/{name}", interfacesHandler.Detail)
		r.Post("/interfaces/{name}/up", interfacesHandler.SetUp)
		r.Post("/interfaces/{name}/down", interfacesHandler.SetDown)
		r.Post("/interfaces/{name}/addr", interfacesHandler.AddAddress)
		r.Delete("/interfaces/{name}/addr", interfacesHandler.RemoveAddress)
		r.Put("/interfaces/{name}/mtu", interfacesHandler.SetMTU)

		// Firewall
		r.Get("/firewall", firewallHandler.List)
		r.Get("/firewall/rules", firewallHandler.GetRules)
		r.Post("/firewall/rules", firewallHandler.AddRule)
		r.Delete("/firewall/rules/{num}", firewallHandler.DeleteRule)
		r.Post("/firewall/rules/{num}/move", firewallHandler.MoveRule)
		r.Post("/firewall/chains", firewallHandler.CreateChain)
		r.Delete("/firewall/chains/{name}", firewallHandler.DeleteChain)
		r.Put("/firewall/chains/{name}/policy", firewallHandler.SetPolicy)
		r.Post("/firewall/save", firewallHandler.SaveRules)
		r.Post("/firewall/flush", firewallHandler.FlushChain)

		// Routes
		r.Get("/routes", routesHandler.List)
		r.Get("/routes/list", routesHandler.GetRoutes)
		r.Post("/routes", routesHandler.AddRoute)
		r.Delete("/routes", routesHandler.DeleteRoute)
		r.Post("/routes/save", routesHandler.SaveRoutes)
		r.Get("/routes/forwarding", routesHandler.GetIPForwarding)
		r.Post("/routes/forwarding/toggle", routesHandler.ToggleIPForwarding)

		// IP Rules
		r.Get("/rules", rulesHandler.List)
		r.Get("/rules/list", rulesHandler.GetRules)
		r.Post("/rules", rulesHandler.AddRule)
		r.Delete("/rules/{priority}", rulesHandler.DeleteRule)
		r.Post("/rules/save", rulesHandler.SaveRules)

		// Routing Tables
		r.Get("/rules/tables", rulesHandler.GetTables)
		r.Get("/rules/tables/options", rulesHandler.GetTableOptions)
		r.Post("/rules/tables", rulesHandler.CreateTable)
		r.Delete("/rules/tables/{name}", rulesHandler.DeleteTable)

		// IPSets
		r.Get("/ipset", ipsetHandler.List)
		r.Get("/ipset/list", ipsetHandler.GetSets)
		r.Post("/ipset", ipsetHandler.CreateSet)
		r.Get("/ipset/{name}", ipsetHandler.ViewSet)
		r.Delete("/ipset/{name}", ipsetHandler.DestroySet)
		r.Post("/ipset/{name}/flush", ipsetHandler.FlushSet)
		r.Get("/ipset/{name}/entries", ipsetHandler.GetEntries)
		r.Post("/ipset/{name}/entries", ipsetHandler.AddEntry)
		r.Delete("/ipset/{name}/entries", ipsetHandler.DeleteEntry)
		r.Post("/ipset/save", ipsetHandler.SaveSets)

		// Tunnels
		r.Post("/tunnels/save", tunnelHandler.SaveTunnels)

		// GRE Tunnels
		r.Get("/tunnels/gre", tunnelHandler.ListGRE)
		r.Get("/tunnels/gre/list", tunnelHandler.GetGRETunnels)
		r.Post("/tunnels/gre", tunnelHandler.CreateGRE)
		r.Delete("/tunnels/gre/{name}", tunnelHandler.DeleteGRE)
		r.Post("/tunnels/gre/{name}/up", tunnelHandler.SetGREUp)
		r.Post("/tunnels/gre/{name}/down", tunnelHandler.SetGREDown)

		// VXLAN Tunnels
		r.Get("/tunnels/vxlan", tunnelHandler.ListVXLAN)
		r.Get("/tunnels/vxlan/list", tunnelHandler.GetVXLANTunnels)
		r.Post("/tunnels/vxlan", tunnelHandler.CreateVXLAN)
		r.Delete("/tunnels/vxlan/{name}", tunnelHandler.DeleteVXLAN)
		r.Post("/tunnels/vxlan/{name}/up", tunnelHandler.SetVXLANUp)
		r.Post("/tunnels/vxlan/{name}/down", tunnelHandler.SetVXLANDown)

		// WireGuard Tunnels
		r.Get("/tunnels/wireguard", tunnelHandler.ListWireGuard)
		r.Get("/tunnels/wireguard/list", tunnelHandler.GetWireGuardTunnels)
		r.Post("/tunnels/wireguard", tunnelHandler.CreateWireGuard)
		r.Get("/tunnels/wireguard/{name}", tunnelHandler.ViewWireGuard)
		r.Delete("/tunnels/wireguard/{name}", tunnelHandler.DeleteWireGuard)
		r.Put("/tunnels/wireguard/{name}", tunnelHandler.UpdateWireGuardInterface)
		r.Post("/tunnels/wireguard/{name}/up", tunnelHandler.SetWireGuardUp)
		r.Post("/tunnels/wireguard/{name}/down", tunnelHandler.SetWireGuardDown)
		r.Get("/tunnels/wireguard/{name}/peers", tunnelHandler.GetWireGuardPeers)
		r.Post("/tunnels/wireguard/{name}/peers", tunnelHandler.AddWireGuardPeer)
		r.Put("/tunnels/wireguard/{name}/peers", tunnelHandler.UpdateWireGuardPeer)
		r.Delete("/tunnels/wireguard/{name}/peers", tunnelHandler.RemoveWireGuardPeer)
		r.Post("/tunnels/wireguard/{name}/addresses", tunnelHandler.AddWireGuardAddress)
		r.Delete("/tunnels/wireguard/{name}/addresses", tunnelHandler.RemoveWireGuardAddress)
		r.Get("/tunnels/wireguard/generate-keypair", tunnelHandler.GenerateKeyPair)

		// Network Namespaces
		r.Get("/netns", netnsHandler.List)
		r.Get("/netns/list", netnsHandler.GetNamespaces)
		r.Post("/netns", netnsHandler.CreateNamespace)
		r.Post("/netns/veth", netnsHandler.CreateVethPair)
		r.Post("/netns/save", netnsHandler.SaveNamespaces)
		r.Get("/netns/{name}", netnsHandler.ViewNamespace)
		r.Delete("/netns/{name}", netnsHandler.DeleteNamespace)
		r.Get("/netns/{name}/interfaces", netnsHandler.GetInterfaces)
		r.Post("/netns/{name}/interfaces", netnsHandler.MoveInterface)
		r.Post("/netns/{name}/interfaces/{iface}/up", netnsHandler.SetInterfaceUp)
		r.Post("/netns/{name}/interfaces/{iface}/down", netnsHandler.SetInterfaceDown)
		r.Delete("/netns/{name}/interfaces/{iface}", netnsHandler.RemoveInterface)

		// Namespace Firewall
		r.Get("/netns/{name}/firewall", netnsHandler.ListFirewall)
		r.Get("/netns/{name}/firewall/rules", netnsHandler.GetFirewallRules)
		r.Post("/netns/{name}/firewall/rules", netnsHandler.AddFirewallRule)
		r.Delete("/netns/{name}/firewall/rules/{num}", netnsHandler.DeleteFirewallRule)
		r.Post("/netns/{name}/firewall/chains", netnsHandler.CreateChain)
		r.Delete("/netns/{name}/firewall/chains/{chain}", netnsHandler.DeleteChain)
		r.Put("/netns/{name}/firewall/chains/{chain}/policy", netnsHandler.SetChainPolicy)

		// Namespace Routes
		r.Get("/netns/{name}/routes", netnsHandler.ListRoutes)
		r.Get("/netns/{name}/routes/list", netnsHandler.GetRoutes)
		r.Post("/netns/{name}/routes", netnsHandler.AddRoute)
		r.Delete("/netns/{name}/routes", netnsHandler.DeleteRoute)

		// Namespace IP Rules
		r.Get("/netns/{name}/rules", netnsHandler.ListRules)
		r.Get("/netns/{name}/rules/list", netnsHandler.GetRules)
		r.Post("/netns/{name}/rules", netnsHandler.AddRule)
		r.Delete("/netns/{name}/rules/{priority}", netnsHandler.DeleteRule)

		// Namespace Routing Tables
		r.Get("/netns/{name}/rules/tables", netnsHandler.GetTables)
		r.Get("/netns/{name}/rules/tables/options", netnsHandler.GetTableOptions)
		r.Post("/netns/{name}/rules/tables", netnsHandler.CreateTable)
		r.Delete("/netns/{name}/rules/tables/{table}", netnsHandler.DeleteTable)

		// Namespace Tunnels
		r.Get("/netns/{name}/tunnels", netnsHandler.ListTunnels)
		r.Get("/netns/{name}/tunnels/list", netnsHandler.GetTunnels)
		r.Post("/netns/{name}/tunnels/gre", netnsHandler.CreateGRETunnel)
		r.Delete("/netns/{name}/tunnels/gre/{tunnel}", netnsHandler.DeleteGRETunnel)
		r.Post("/netns/{name}/tunnels/vxlan", netnsHandler.CreateVXLANTunnel)
		r.Delete("/netns/{name}/tunnels/vxlan/{tunnel}", netnsHandler.DeleteVXLANTunnel)
		r.Post("/netns/{name}/tunnels/wireguard", netnsHandler.CreateWireGuardTunnel)
		r.Delete("/netns/{name}/tunnels/wireguard/{tunnel}", netnsHandler.DeleteWireGuardTunnel)
		r.Post("/netns/{name}/tunnels/{tunnel}/up", netnsHandler.SetTunnelUp)
		r.Post("/netns/{name}/tunnels/{tunnel}/down", netnsHandler.SetTunnelDown)

		// Settings
		r.Get("/settings", settingsHandler.Settings)
		r.Post("/settings/password", settingsHandler.ChangePassword)
		r.Post("/settings/save-all", settingsHandler.SaveAll)
		r.Get("/settings/export", settingsHandler.ExportConfig)
		r.Post("/settings/import", settingsHandler.ImportConfig)

		// Admin-only routes
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.RequireAdmin)
			r.Post("/settings/users", settingsHandler.CreateUser)
			r.Put("/settings/users/{id}", settingsHandler.UpdateUser)
			r.Delete("/settings/users/{id}", settingsHandler.DeleteUser)
		})
	})

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Starting Linux Router GUI on %s", addr)
	log.Printf("Default credentials: %s / %s", cfg.DefaultAdmin, cfg.DefaultPassword)

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
		os.Exit(0)
	}()

	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getWebDir() string {
	// Check for environment variable
	if dir := os.Getenv("ROUTER_WEB_DIR"); dir != "" {
		return dir
	}

	// Try relative paths from executable
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)

		// Check ../web (for build directory structure)
		candidate := filepath.Join(exeDir, "..", "web")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}

		// Check ../../web (for cmd/server structure)
		candidate = filepath.Join(exeDir, "..", "..", "web")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Try current working directory
	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, "web")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Default fallback
	return "./web"
}

func loadTemplates(templatesDir string) (*TemplateRegistry, error) {
	funcMap := template.FuncMap{
		"formatBytes": formatBytes,
		"dict":        dict,
		"uint64":      toUint64,
	}

	registry := NewTemplateRegistry(funcMap)

	layoutsDir := filepath.Join(templatesDir, "layouts")
	partialsDir := filepath.Join(templatesDir, "partials")
	pagesDir := filepath.Join(templatesDir, "pages")

	// Collect shared template files
	var sharedFiles []string

	layoutFiles, _ := filepath.Glob(filepath.Join(layoutsDir, "*.html"))
	sharedFiles = append(sharedFiles, layoutFiles...)

	partialFiles, _ := filepath.Glob(filepath.Join(partialsDir, "*.html"))
	sharedFiles = append(sharedFiles, partialFiles...)

	// Get page template files
	pageFiles, err := filepath.Glob(filepath.Join(pagesDir, "*.html"))
	if err != nil {
		return nil, err
	}

	// For each page, create a separate template that includes shared templates + that page
	for _, pageFile := range pageFiles {
		pageName := filepath.Base(pageFile)

		// Create a new template set for this page
		tmpl := template.New(pageName).Funcs(funcMap)

		// Parse shared templates
		for _, sharedFile := range sharedFiles {
			content, err := os.ReadFile(sharedFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", sharedFile, err)
			}
			_, err = tmpl.Parse(string(content))
			if err != nil {
				return nil, fmt.Errorf("failed to parse %s: %w", sharedFile, err)
			}
		}

		// Parse the page template
		pageContent, err := os.ReadFile(pageFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", pageFile, err)
		}
		_, err = tmpl.Parse(string(pageContent))
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", pageFile, err)
		}

		registry.Add(pageName, tmpl)
	}

	// Also add partial templates standalone for HTMX partial responses
	for _, partialFile := range partialFiles {
		partialName := filepath.Base(partialFile)

		tmpl := template.New(partialName).Funcs(funcMap)

		// Parse all partials (they may reference each other)
		for _, pf := range partialFiles {
			content, err := os.ReadFile(pf)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", pf, err)
			}
			_, err = tmpl.Parse(string(content))
			if err != nil {
				return nil, fmt.Errorf("failed to parse %s: %w", pf, err)
			}
		}

		registry.Add(partialName, tmpl)
	}

	return registry, nil
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), []string{"KB", "MB", "GB", "TB"}[exp])
}

func dict(values ...interface{}) map[string]interface{} {
	if len(values)%2 != 0 {
		return nil
	}
	d := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil
		}
		d[key] = values[i+1]
	}
	return d
}

func toUint64(v int) uint64 {
	if v < 0 {
		return 0
	}
	return uint64(v)
}
