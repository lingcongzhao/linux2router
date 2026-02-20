# Service Status Auto-Refresh - Verification Guide

## Issue Resolution

**Problem**: After clicking Start/Stop/Restart service buttons, the status badge did not update automatically.

**Root Cause**: The `renderAlert()` function was setting `HX-Trigger: refresh` which overwrote our `refreshStatus` trigger.

**Solution**: Created `renderAlertWithRefresh()` function that triggers BOTH events using comma-separated values: `HX-Trigger: refresh, refreshStatus`

---

## Changes Made

### 1. Added New Helper Function

**File**: `internal/handlers/dnsmasq.go`

```go
func (h *DnsmasqHandler) renderAlertWithRefresh(w http.ResponseWriter, alertType, message string) {
    // Trigger both refresh and refreshStatus events
    w.Header().Set("HX-Trigger", "refresh, refreshStatus")
    data := map[string]interface{}{
        "Type":    alertType,
        "Message": message,
    }
    if err := h.templates.ExecuteTemplate(w, "alert.html", data); err != nil {
        log.Printf("Alert template error: %v", err)
    }
}
```

### 2. Updated Service Control Handlers

Changed from `renderAlert()` to `renderAlertWithRefresh()`:

- `StartService()` - Now triggers both events
- `StopService()` - Now triggers both events  
- `RestartService()` - Now triggers both events

### 3. Added Delay to Status Refresh

**Files**: `web/templates/pages/dns.html`, `web/templates/pages/dhcp.html`

```html
<!-- Added delay:500ms to allow service to start/stop -->
<div id="dns-status-container"
     hx-get="/dns/status"
     hx-trigger="refreshStatus from:body delay:500ms"
     hx-swap="innerHTML">
```

The 500ms delay ensures the systemd service has time to fully start or stop before we query its status.

---

## How to Verify the Fix

### Test Procedure

#### Step 1: Navigate to DNS Page
```
URL: http://your-router/dns
```

#### Step 2: Check Current Status
- Look at the "Service Status" card
- Note whether it shows "Running" (green) or "Stopped" (red)

#### Step 3: Stop the Service (if running)
1. Click **"Stop Service"** button
2. **Wait** - Do NOT refresh the page
3. **Observe the status badge** in the "Service Status" card

**Expected Result**:
- ✅ Success message appears in alert
- ✅ After ~500ms, status badge changes from green "Running" to red "Stopped"
- ✅ NO manual page refresh required

#### Step 4: Start the Service
1. Click **"Start Service"** button
2. **Wait** - Do NOT refresh the page
3. **Observe the status badge** in the "Service Status" card

**Expected Result**:
- ✅ Success message appears in alert
- ✅ After ~500ms, status badge changes from red "Stopped" to green "Running"
- ✅ NO manual page refresh required

#### Step 5: Restart the Service
1. Click **"Restart Service"** button
2. **Wait** - Do NOT refresh the page
3. **Observe the status badge** in the "Service Status" card

**Expected Result**:
- ✅ Success message appears in alert
- ✅ After ~500ms, status badge remains green "Running"
- ✅ NO manual page refresh required

#### Step 6: Repeat for DHCP Page
```
URL: http://your-router/dhcp
```

Repeat steps 2-5 for the DHCP page. All behaviors should be identical.

---

## Troubleshooting

### Status Still Doesn't Update

**Check 1: Browser Console**
```
1. Open browser DevTools (F12)
2. Go to Console tab
3. Perform Start/Stop operation
4. Look for JavaScript errors
```

**Expected**: No errors

---

**Check 2: Network Tab**
```
1. Open browser DevTools (F12)
2. Go to Network tab
3. Click "Stop Service"
4. Look for the POST request to /dns/stop
5. Check the Response Headers
```

**Expected Headers**:
```
HX-Trigger: refresh, refreshStatus
```

If you see only `HX-Trigger: refresh`, the fix wasn't applied correctly.

---

**Check 3: Status Endpoint**
```bash
# Test the status endpoint directly
curl -s http://localhost:8080/dns/status

# Should return HTML like:
# <div class="card">
#   ...
#   <span class="badge badge-green">Running</span>
#   ...
# </div>
```

---

**Check 4: Service Actually Starting/Stopping**
```bash
# Check if dnsmasq service is actually running
systemctl status dnsmasq

# Should show:
# Active: active (running)  <-- if started
# Active: inactive (dead)   <-- if stopped
```

If the service isn't actually starting/stopping, the status will appear correct (reflecting the actual state).

---

### Status Updates But Slowly

**Symptom**: Status takes 2-3 seconds to update

**Possible Causes**:
1. `systemctl is-active dnsmasq` is slow
2. Network latency
3. Server under load

**Solutions**:
```go
// In dnsmasq.go, add caching to GetStatus()
var statusCache struct {
    status    string
    timestamp time.Time
    mu        sync.Mutex
}

func (s *DnsmasqService) GetStatus() (string, error) {
    statusCache.mu.Lock()
    defer statusCache.mu.Unlock()
    
    // Return cached status if less than 1 second old
    if time.Since(statusCache.timestamp) < time.Second {
        return statusCache.status, nil
    }
    
    // ... rest of function
    statusCache.status = status
    statusCache.timestamp = time.Now()
    return status, nil
}
```

---

### Multiple Status Updates

**Symptom**: Status flickers or updates multiple times

**Cause**: Multiple HTMX triggers firing

**Debug**:
```html
<!-- Add to status container for debugging -->
<div id="dns-status-container"
     hx-get="/dns/status"
     hx-trigger="refreshStatus from:body delay:500ms"
     hx-swap="innerHTML"
     hx-on::before-request="console.log('Refreshing status...')"
     hx-on::after-request="console.log('Status refreshed')">
```

Check browser console - should only see one "Refreshing status..." per button click.

---

## Technical Details

### HTMX Multi-Event Triggering

HTMX supports multiple events in a single header:

```go
// Single event
w.Header().Set("HX-Trigger", "refresh")

// Multiple events (comma-separated)
w.Header().Set("HX-Trigger", "refresh, refreshStatus")

// Multiple events with data (JSON)
w.Header().Set("HX-Trigger", `{"refresh": null, "refreshStatus": {"delay": 500}}`)
```

### Why the Delay?

The 500ms delay accounts for:
1. Systemd processing the start/stop command
2. Service actually starting/stopping
3. Status check returning accurate result

Without the delay, we might query the status before systemd has updated it, showing stale information.

### Event Flow

```
User clicks "Start Service"
    ↓
POST /dns/start
    ↓
handler: dnsmasqService.Start()
    ↓
handler: Set HX-Trigger: refresh, refreshStatus
    ↓
handler: Render success alert
    ↓
Response sent to browser
    ↓
HTMX processes response headers
    ↓
Triggers "refresh" event → Other components reload
    ↓
Triggers "refreshStatus" event → Status container detects it
    ↓
Wait 500ms (delay:500ms)
    ↓
GET /dns/status
    ↓
handler: Get current service status
    ↓
handler: Render dns_status.html partial
    ↓
Response sent to browser
    ↓
HTMX swaps innerHTML of status container
    ↓
Badge updates: "Stopped" (red) → "Running" (green)
```

---

## Success Criteria

### ✅ All Tests Pass

- [ ] DNS Start updates status automatically
- [ ] DNS Stop updates status automatically
- [ ] DNS Restart updates status automatically
- [ ] DHCP Start updates status automatically
- [ ] DHCP Stop updates status automatically
- [ ] DHCP Restart updates status automatically
- [ ] No page refresh required
- [ ] No JavaScript errors in console
- [ ] Status updates within 1 second
- [ ] Badge color changes correctly (red ↔ green)

### ✅ User Experience

- [ ] Clicking button shows immediate success message
- [ ] Status badge updates automatically after ~500ms
- [ ] Page doesn't reload
- [ ] Scroll position maintained
- [ ] Other page content unaffected

---

## Comparison: Before vs After

| Aspect | Before | After |
|--------|--------|-------|
| Status Update | Manual refresh required ❌ | Automatic ✅ |
| Wait Time | User must refresh | ~500ms delay ✅ |
| UX | Confusing, status seems stuck ❌ | Smooth, instant feedback ✅ |
| Network Requests | Full page reload | Single partial update ✅ |
| Data Transfer | ~50KB (full page) | ~500B (status only) ✅ |
| Browser Work | Full re-render | Minimal DOM update ✅ |

---

## Additional Testing

### Test Edge Cases

1. **Rapid Clicking**
   - Click Start → Stop → Start quickly
   - Status should reflect final state

2. **Service Already Running**
   - Click "Start Service" when already running
   - Should show error or success
   - Status should remain "Running"

3. **Service Already Stopped**
   - Click "Stop Service" when already stopped
   - Should show error or success
   - Status should remain "Stopped"

4. **Network Disconnection**
   - Disconnect network
   - Click Start/Stop
   - Should show appropriate error
   - Status should not update incorrectly

5. **Multiple Browser Tabs**
   - Open /dns in two tabs
   - Click Start in tab 1
   - Manually refresh tab 2
   - Both should show consistent status

---

## Rollback Procedure

If the fix causes issues, rollback:

```bash
cd /home/ubuntu/opencode/linux2router

# Revert handler changes
git diff internal/handlers/dnsmasq.go
git checkout internal/handlers/dnsmasq.go

# Revert template changes
git checkout web/templates/pages/dns.html
git checkout web/templates/pages/dhcp.html

# Rebuild
go build ./cmd/server

# Restart service
sudo systemctl restart router-gui
```

---

## Success! 🎉

If all tests pass, the status auto-refresh is working correctly!

**Summary of Fix**:
- Created `renderAlertWithRefresh()` to trigger multiple HTMX events
- Added 500ms delay to allow service state to stabilize
- Status now updates automatically without manual refresh

**Files Modified**:
- `internal/handlers/dnsmasq.go` - Added renderAlertWithRefresh() method
- `web/templates/pages/dns.html` - Added delay to refresh trigger
- `web/templates/pages/dhcp.html` - Added delay to refresh trigger
