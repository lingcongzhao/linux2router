# Dnsmasq Troubleshooting Guide

## Issue: Empty DHCP and DNS Pages

### Problem
When accessing `/dhcp` or `/dns` pages, the content appears empty or only shows the header/footer.

### Root Cause
This was caused by:
1. **Template syntax error**: The `{{template "base" .}}` call was placed before `{{end}}` instead of after
2. **Missing template context**: Partial templates weren't receiving the correct data context

### Fix Applied

#### 1. Template Order Fix
**Before (WRONG)**:
```go
</div>
{{template "base" .}}
{{end}}
```

**After (CORRECT)**:
```go
</div>
{{end}}

{{template "base" .}}
```

#### 2. Template Context Fix
**Before (WRONG)**:
```html
{{template "dhcp_static_leases" .}}
```

**After (CORRECT)**:
```html
{{template "dhcp_static_leases" (dict "Leases" .Config.StaticLeases)}}
```

### Verification Steps

1. **Check Template Syntax**:
   ```bash
   # Verify the templates end correctly
   tail -5 web/templates/pages/dhcp.html
   tail -5 web/templates/pages/dns.html
   
   # Should show:
   # </div>
   # {{end}}
   #
   # {{template "base" .}}
   ```

2. **Rebuild Application**:
   ```bash
   go build ./cmd/server
   ```

3. **Test Pages**:
   - Navigate to `/dhcp` - should show full DHCP configuration UI
   - Navigate to `/dns` - should show full DNS configuration UI
   - Both pages should display:
     - Service status
     - Configuration forms
     - Tables (static leases, custom hosts, etc.)

### Expected Page Content

#### DHCP Page (`/dhcp`)
Should display:
- ✅ Service status badge (Running/Stopped)
- ✅ Service control buttons (Start/Stop/Restart)
- ✅ DHCP configuration form (interface, IP range, etc.)
- ✅ Static leases table
- ✅ Active leases table
- ✅ "Add Reservation" button and modal

#### DNS Page (`/dns`)
Should display:
- ✅ Service status badge (Running/Stopped)
- ✅ Service control buttons (Start/Stop/Restart)
- ✅ DNS configuration form (port, cache size, upstream servers)
- ✅ Custom DNS hosts table
- ✅ Advanced domain rules table
- ✅ DNS statistics dashboard
- ✅ Recent DNS queries table
- ✅ "Add Host" and "Add Rule" buttons with modals

### Common Template Issues

#### Issue: Blank sections in the page
**Cause**: Missing data in template context
**Fix**: Ensure handler passes all required data:
```go
data := map[string]interface{}{
    "Title":      "DHCP Server",
    "ActivePage": "dhcp",
    "User":       user,
    "Config":     config.DHCP,  // Must include this
    "Interfaces": interfaces,
    "Status":     status,
}
```

#### Issue: Template not found errors
**Cause**: Partial template name mismatch
**Fix**: Verify template define name matches reference:
```html
<!-- In partial file -->
{{define "dhcp_static_leases"}}

<!-- In page file -->
{{template "dhcp_static_leases" ...}}
```

#### Issue: HTMX sections not loading
**Cause**: Missing HTMX endpoint or incorrect URL
**Fix**: Verify routes are registered:
```go
r.Get("/dhcp/static-leases", dnsmasqHandler.GetStaticLeases)
r.Get("/dns/custom-hosts", dnsmasqHandler.GetCustomHosts)
```

### Debug Mode

To see detailed template errors, check the server logs:

```bash
# Run server with logging
./server 2>&1 | grep -i "template\|error"
```

Common log messages:
- `Template error: ...` - Template rendering failed
- `Failed to get config: ...` - Service error
- `Template ... not found` - Missing template file

### Testing Checklist

After fixing template issues, verify:

- [ ] DHCP page loads completely
- [ ] DNS page loads completely
- [ ] All forms are visible
- [ ] All tables are visible
- [ ] Modals open when clicking "+ Add" buttons
- [ ] HTMX auto-refresh works (tables update every 10s)
- [ ] Service control buttons work
- [ ] No JavaScript console errors
- [ ] No template errors in server logs

### Additional Notes

1. **Template Function `dict`**: Used to create map contexts for partials
   ```html
   {{template "partial" (dict "Key" .Value)}}
   ```

2. **Auto-refresh**: HTMX triggers refresh every 10 seconds
   ```html
   hx-trigger="every 10s, refresh from:body"
   ```

3. **Manual refresh**: Triggered by success responses
   ```go
   w.Header().Set("HX-Trigger", "refresh")
   ```

### Related Files

Modified to fix the issue:
- `web/templates/pages/dhcp.html` - Fixed template order and context
- `web/templates/pages/dns.html` - Fixed template order and context

No changes needed:
- `internal/handlers/dnsmasq.go` - Handlers working correctly
- `web/templates/partials/*.html` - Partials defined correctly
