The user wants me to execute the verification/test plan from the original implementation plan and fix any bugs I find. Let me start by running the application and testing each component systematically.

The test plan from the original specification was:
1. Build & Run: `go build -o router-gui ./cmd/server && sudo ./router-gui`
2. Test Authentication:
   - Access http://localhost:8090, verify redirect to login
   - Login with default admin credentials
   - Verify session persistence across page loads
3. Test Firewall:
   - View existing iptables rules
   - Add a test rule, verify it appears in `iptables -L`
   - Delete the rule, verify removal
4. Test Routes:
   - View routing table, compare with `ip route`
   - Add/delete test routes
5. Test Interfaces:
   - View interface list, compare with `ip link`
   - Test up/down operations on a test interface
6. Test Persistence:
   - Save configuration
   - Verify files created in configs/
   - Restart service, verify restoration

Let me start the application and run through these tests.
