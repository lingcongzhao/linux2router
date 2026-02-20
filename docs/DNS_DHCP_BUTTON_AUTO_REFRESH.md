# DNS/DHCP Service Button Auto-Refresh Fix

## Problem

After clicking "Start Service" or "Stop Service" buttons on the `/dns` or `/dhcp` pages, the button did not automatically change to reflect the new service state. Users had to manually refresh the page to see the updated button state.

## Root Cause

The DNS and DHCP services share the same underlying dnsmasq service. When a user clicks Start/Stop:
1. The service action is executed successfully
2. The server returns an alert message
3. But the button UI was not being refreshed to reflect the new service state

Multiple approaches were tried but failed due to:
- HTMX event timing issues
- Incorrect element ID targeting in JavaScript
- Race conditions between button click and status check

## Solution

The final working solution uses **JavaScript polling** to periodically fetch the actual service state from the server and update the UI.

### Implementation

**File: `web/templates/layouts/base.html`**

Added JavaScript polling at the end of the body:

```html
<script>
function updateServiceControls(service) {
    Promise.all([
        fetch('/' + service + '/controls').then(r => r.text()),
        fetch('/' + service + '/status').then(r => r.text())
    ]).then(([controlsHtml, statusHtml]) => {
        const controlsContainer = document.getElementById(service + '-controls-container');
        if (controlsContainer) {
            controlsContainer.innerHTML = controlsHtml;
            htmx.process(controlsContainer);
        }
        
        const statusContainer = document.getElementById(service + '-status-container');
        if (statusContainer) {
            statusContainer.innerHTML = statusHtml;
            htmx.process(statusContainer);
        }
    }).catch(err => console.error('Error updating ' + service + ' controls:', err));
}

// Start polling after a short delay to ensure page is loaded
setTimeout(function() {
    updateServiceControls('dns');
    updateServiceControls('dhcp');
    
    setInterval(function() { updateServiceControls('dns'); }, 3000);
    setInterval(function() { updateServiceControls('dhcp'); }, 3000);
}, 500);
</script>
```

**Key points:**
- Fetches from `/dns/controls` and `/dhcp/controls` endpoints
- Updates both the button controls and status display
- Polls every 3 seconds
- Initial update after 500ms delay to ensure page is loaded

### Template Changes

**Files: `web/templates/pages/dns.html` and `dhcp.html`**

Removed HTMX triggers and made containers static:

```html
<div id="dns-controls-container">
    {{template "dns_controls" .}}
</div>

<div id="dns-status-container">
    {{template "dns_status" .}}
</div>
```

### Server Endpoints

The endpoints `/dns/controls` and `/dhcp/controls` return the button HTML based on the current service status:

```go
func (h *DnsmasqHandler) GetDNSControls(w http.ResponseWriter, r *http.Request) {
    status, err := h.dnsmasqService.GetStatus()
    // ...
    data := map[string]interface{}{
        "Status": status,
    }
    h.templates.ExecuteTemplate(w, "dns_controls.html", data)
}
```

## Why This Works

1. **Separation of concerns**: The button state is determined by the server, not client-side JavaScript logic
2. **Reliability**: Polling is simple and doesn't depend on complex event chains
3. **Consistency**: Both the button and status display stay in sync with actual service state
4. **No timing issues**: The 3-second interval gives the service enough time to start/stop

## Alternative Approaches That Failed

1. **HTMX triggers with HX-Trigger header**: The server sent refresh events but they didn't reliably trigger UI updates
2. **Client-side onclick handlers**: After HTML replacement, event handlers were lost
3. **Immediate button toggle**: Race conditions caused the button to show wrong state after multiple clicks

## Files Modified

- `web/templates/layouts/base.html` - Added JavaScript polling
- `web/templates/pages/dns.html` - Simplified containers
- `web/templates/pages/dhcp.html` - Simplified containers
- `web/templates/partials/dns_controls.html` - Simplified (no special attributes)
- `web/templates/partials/dhcp_controls.html` - Simplified (no special attributes)

## Testing

1. Navigate to `/dns` or `/dhcp` page
2. Click "Start Service" or "Stop Service"
3. Wait up to 3 seconds
4. The button should automatically update to show the correct state
5. The service status display should also update
