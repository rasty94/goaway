# GoAway - Project Improvements & TODOs

After reviewing the `goaway` project architecture, documentation, and drawing inspiration from Pi-hole, here are the proposed areas for enhancements and new features, organized by estimated implementation complexity.

## 🟢 Low Complexity (Quick Wins)

### Developer Experience & CI/CD
- [x] **Code Coverage & Linters**: Add `golangci-lint` to CI workflows and publish test coverage reports. Add Git `pre-commit` hooks.
- [x] **Binary Signing**: Add detached signatures or checksums (`sha256sum`) generation to the release CI/CD for better security and verification of releases.
- [x] **OpenAPI/Swagger Specification**: Auto-generate API documentation for the Go backend endpoints using tools like `swaggo/swag`. This aids custom integrations and frontend generation.

### Security & System
- [x] **Drop Root Privileges**: Document and provide tooling (`setcap cap_net_bind_service=+ep`) so the DNS server does not need to run as full root to bind on port 53.
- [x] **Graceful Shutdown**: Implement comprehensive graceful shutdown logic in `backend/lifecycle/manager.go` handling the `SIGINT`/`SIGTERM` signals correctly to avoid dropping in-flight DNS queries.

### Dashboard / Frontend
- [x] **Localization (i18n)**: Fully implemented frontend support for multiple languages (English/Spanish). Integrated across all main pages and components.
- [ ] **Live Query Tail**: Real-time visualization of incoming DNS requests with interactive filtering and highlighting.
- [ ] **Advanced Data Visualization**: More detailed charts for top permitted/blocked domains and client activity over time.
- [x] **Network Topology Graph**: Interactive 2D visualization of connected clients and their DNS traffic patterns (Implemented via DNSServerVisualizer).
- [ ] **Mobile-First Responsive Audit**: Ensure the entire dashboard is 100% usable on mobile devices with touch-friendly targets.
- [ ] **Go-Native Frontend Migration**: Evaluate and potentially refactor the current React frontend into Go Templates + HTMX to achieve a "Zero-NodeJS" dependency goal and a single-binary deployment.

---

## 🟡 Medium Complexity (Core Enhancements)

### Advanced DNS Management
- [ ] **DNS Caching Layer**: Implement an intelligent in-memory DNS caching system (respecting record TTLs) or integrate a cache database to drastically reduce upstream latency and improve resolution performance for repeated queries.
- [ ] **Local DNS & CNAME Records**: Allow the creation of custom Local DNS records (A/AAAA) and CNAME mapping directly from the UI, overriding upstream resolution for internal networks.
- [ ] **Allowlist / Whitelist Lists**: Add support for explicit allowlists to override blocklists for specific domains (or broadly).
- [ ] **Wildcard Matching**: Introduce wildcard domain matching (e.g., `*.evil.com`) for both blocklists and allowlists to improve coverage easily.
- [ ] **Regex Blocking**: Implement Regular Expressions (Regex) support for advanced domain blacklisting and whitelisting.
- [ ] **Conditional Forwarding**: Add support to forward queries for local domains (e.g., `*.lan`) and reverse lookups directly to a localized router/gateway.

### System Architecture
- [ ] **Schema Migrations**: Introduce a migration runner (e.g., `golang-migrate`) for managing backend database schema updates across versions explicitly.
- [ ] **Data Backup & Restore (Teleporter)**: Implement an export/import feature allowing users to backup all settings, blocklists, local DNS, and DHCP configurations.
- [ ] **Remote Backups**: Extend backup functionality to automatically sync backups to remote storages like an AWS S3 bucket, a remote directory (NFS/SMB), or WebDAV.
- [ ] **Metrics & Observability**: Expose detailed Prometheus metrics for DNS latency, cache hit/miss rates, and blocked domains to allow integration with Grafana.

### Authentication & Users
- [ ] **Multi-User Administration**: Refactor the auth system to support custom Admin usernames (not just a single password) and allow multiple administrative or view-only users to access the dashboard.

---

## 🔴 High Complexity (Major Undertakings)

### Core Network Services
- [ ] **Built-in DHCP Server**: Implement a lightweight native DHCP server in Go (IPv4/IPv6 support) to allow GoAway to natively manage LAN IP assignments and hand out its own IP for DNS automatically.
- [ ] **Static DHCP Leases**: Allow admins to bind specific IP addresses to MAC addresses persistently via the dashboard (requires the DHCP server module).

### Advanced Security
- [ ] **DNSSEC Validation**: Add support for rigorous DNSSEC validation for outgoing upstream queries to ensure cryptographically secure resolutions.
- [ ] **Rate Limiting & Throttling**: Add advanced rate-limiting logic per client IP to mitigate DNS amplification or DoS attacks.
- [ ] **Group Management (Per-Client Blocking)**: Introduce a multi-group system (default vs. custom) where blocklists, allowlists, and specific domains can be selectively applied to different clients/IPs/MACs across the network (similar to Pi-hole V5+).

### Platform & Scaling
- [ ] **High Availability / Synchronization**: Enable running multiple instances of `goaway` on the same network that can sync blocklists, DHCP leases, allowlists, and local DNS automatically for redundancy (Primary/Secondary setup).
- [ ] **Full Windows / macOS Support**: Move macOS and Windows support from "Beta" to "Full". This involves validating low-level networking behaviors, DHCP broadcasts, and path resolutions specific to these OS environments.
- [ ] **E2E & Integration Tests**: Set up Docker-based End-to-End integration tests simulating actual client queries, HTTP requests, and verifying database states dynamically.
