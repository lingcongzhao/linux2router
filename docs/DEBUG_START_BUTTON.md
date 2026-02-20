## Debugging "Start Service" Button Not Changing

### Changes Made

1. **Increased Delay**: Changed from 500ms to 1000ms (1 second)
   - File: `web/templates/pages/dns.html`
   - File: `web/templates/pages/dhcp.html`
   - Both status and controls now wait 1 second before refreshing

2. **Added Debug Logging**: Server logs will show status value
   - Check server logs for: `DEBUG: GetDNSControls - Status: '...'`
   - Check server logs for: `DEBUG: GetDHCPControls - Status: '...'`

3. **Added Tooltip Debugging**: Hover over button to see status
   - Hover mouse over Start/Stop button
   - Tooltip will show: "Status: active" or "Status: inactive"

### Testing Steps

#### Step 1: Check Initial State

1. Go to `/dns` page
2. Hover over the Start/Stop button
3. Note the tooltip (should show current status)
4. Note the button text

#### Step 2: Test Starting Service

1. Ensure service is stopped first:
   ```bash
   sudo systemctl stop dnsmasq
   ```

2. Refresh the `/dns` page
3. Button should show "Start Service" (green)
4. Hover over button - tooltip should show "Status: inactive"

5. Click "Start Service"
6. Wait 1-2 seconds
7. **Check server logs** for status updates
8. Hover over button again
9. Button should now show "Stop Service" (red)
10. Tooltip should show "Status: active"

#### Step 3: Test Stopping Service

1. Button should currently show "Stop Service" (red)
2. Click "Stop Service"
3. Wait 1-2 seconds
4. Hover over button
5. Button should show "Start Service" (green)
6. Tooltip should show "Status: inactive"

### Debugging

#### Check 1: Server Logs

Watch the server logs while testing:

```bash
# If running as systemd service
sudo journalctl -u your-router-service -f

# Or if running directly
./server 2>&1 | grep -E "DEBUG|Status|Start|Stop"
```

Look for:
```
DEBUG: GetDNSControls - Status: 'active'
DEBUG: GetDNSControls - Status: 'inactive'
```

#### Check 2: Network Tab

1. Open DevTools (F12)
2. Go to Network tab
3. Click "Start Service"
4. Look for these requests:

   **Immediate**:
   - POST `/dns/start` → Response should have `HX-Trigger: refresh, refreshStatus`

   **After 1 second**:
   - GET `/dns/controls` → Should return HTML with button
   - GET `/dns/status` → Should return HTML with status badge

5. Click on `/dns/controls` request
6. Check "Preview" tab
7. Should see button HTML with status

#### Check 3: Status Endpoint Directly

Test the endpoint manually:

```bash
# Start the service
sudo systemctl start dnsmasq

# Check what the endpoint returns
curl -s http://localhost:8080/dns/controls | grep -o 'title="Status: [^"]*"'

# Should output: title="Status: active"

# Stop the service  
sudo systemctl stop dnsmasq

# Check again
curl -s http://localhost:8080/dns/controls | grep -o 'title="Status: [^"]*"'

# Should output: title="Status: inactive"
```

#### Check 4: Button HTML

After clicking Start, inspect the button element:

```html
<!-- Should look like this after starting: -->
<button class="btn btn-danger"
        hx-post="/dns/stop"
        title="Status: active">
    Stop Service
</button>

<!-- Should look like this after stopping: -->
<button class="btn btn-success"
        hx-post="/dns/start"
        title="Status: inactive">
    Start Service
</button>
```

### Common Issues

#### Issue 1: Button doesn't change after Start

**Symptom**: Button stays "Start Service" after clicking

**Possible Causes**:

1. **Service failed to start**
   ```bash
   sudo systemctl status dnsmasq
   # Check if actually running
   ```

2. **Status check happening too fast**
   - Current delay: 1 second
   - Try increasing to 2 seconds in templates:
     ```html
     hx-trigger="refreshStatus from:body delay:2s"
     ```

3. **Controls not refreshing**
   - Check browser console for errors
   - Check if `/dns/controls` request is made
   - Check if HX-Trigger header is sent

4. **Wrong status value**
   - Check server logs for status value
   - Hover button to see tooltip
   - Status might not be "active" (could be "running" or something else)

#### Issue 2: Button changes for Stop but not Start

**Symptom**: Stop → Start works, but Start → Stop doesn't

**Possible Cause**: Service starts slower than it stops

**Solution**: Increase delay for start operations only, or increase global delay

```html
<!-- Current -->
hx-trigger="refreshStatus from:body delay:1s"

<!-- Try -->
hx-trigger="refreshStatus from:body delay:2s"
```

#### Issue 3: Tooltip shows wrong status

**Symptom**: Tooltip doesn't match button text

**Cause**: Template logic error or status not "active"/"inactive"

**Check**:
```bash
systemctl is-active dnsmasq
# Note exact output - might not be "active"/"inactive"
```

If output is different (e.g., "running", "stopped"), update templates:

```go
// In template, change from:
{{if eq .Status "active"}}

// To:
{{if or (eq .Status "active") (eq .Status "running")}}
```

### Expected Timeline

```
0ms: User clicks "Start Service"
    ↓
10ms: POST /dns/start sent
    ↓
50ms: Service starts (systemctl start dnsmasq)
    ↓
100ms: Handler responds with HX-Trigger: refresh, refreshStatus
    ↓
150ms: Success alert appears
    ↓
1000ms: HTMX delay expires
    ↓
1050ms: GET /dns/controls sent
        GET /dns/status sent
    ↓
1100ms: Responses received
    ↓
1110ms: Button updates to "Stop Service" (red)
        Status badge updates to "Running" (green)
```

### Manual Verification

Run these commands in sequence:

```bash
# 1. Stop service
sudo systemctl stop dnsmasq

# 2. Check status endpoint shows inactive
curl -s http://localhost:8080/dns/controls | grep -E "Start Service|Stop Service"
# Should output: Start Service

# 3. Start service
sudo systemctl start dnsmasq

# 4. Wait 1 second
sleep 1

# 5. Check status endpoint shows active
curl -s http://localhost:8080/dns/controls | grep -E "Start Service|Stop Service"
# Should output: Stop Service
```

If step 5 shows "Start Service", the issue is in the GetStatus() function or template logic.

### Quick Fixes

#### Fix 1: Increase Delay

If service takes longer to start:

```html
<!-- In dns.html and dhcp.html -->
<!-- Change from delay:1s to delay:2s -->
hx-trigger="refreshStatus from:body delay:2s"
```

#### Fix 2: Force Refresh

Add a second refresh trigger:

```html
hx-trigger="refreshStatus from:body delay:1s, refreshStatus from:body delay:3s"
```

This refreshes twice - once at 1s, once at 3s.

#### Fix 3: Polling Fallback

Add periodic polling:

```html
hx-trigger="refreshStatus from:body delay:1s, every 5s"
```

This refreshes on event + every 5 seconds as backup.

### Success Criteria

✅ **Working Correctly**:
- [ ] Hover over button shows correct status in tooltip
- [ ] Server logs show correct status
- [ ] Click "Start Service" → wait 1-2s → button changes to "Stop Service"
- [ ] Click "Stop Service" → wait 1-2s → button changes to "Start Service"  
- [ ] Status badge updates in sync with button
- [ ] No JavaScript errors in console
- [ ] Network tab shows control refresh request

### Final Test Script

```bash
#!/bin/bash

echo "=== Automated Button Update Test ==="

# Function to check button text via API
check_button() {
    BUTTON_TEXT=$(curl -s http://localhost:8080/dns/controls | grep -oP '(?<=">)[^<]+(?=</button>)' | head -1)
    echo "$BUTTON_TEXT"
}

echo "1. Stopping service..."
sudo systemctl stop dnsmasq
sleep 1

echo "2. Checking button text (should be 'Start Service')..."
BUTTON=$(check_button)
echo "   Button: $BUTTON"
[[ "$BUTTON" == *"Start"* ]] && echo "   ✅ PASS" || echo "   ❌ FAIL"

echo
echo "3. Starting service..."
sudo systemctl start dnsmasq
sleep 2  # Wait for status to update

echo "4. Checking button text (should be 'Stop Service')..."
BUTTON=$(check_button)
echo "   Button: $BUTTON"
[[ "$BUTTON" == *"Stop"* ]] && echo "   ✅ PASS" || echo "   ❌ FAIL"

echo
echo "=== Test Complete ==="
```

Save as `test_button_update.sh`, make executable, and run:

```bash
chmod +x test_button_update.sh
./test_button_update.sh
```

If both tests PASS, the API is working correctly. If they FAIL, the issue is in the GetStatus() or template logic.

### Next Steps

After testing:

1. **If button updates correctly**: Remove debug logging and tooltips
2. **If button doesn't update**: Check server logs and share the status value
3. **If status is not "active"/"inactive"**: Update template conditions

The debug tooltip will help identify exactly what status value is being received!
