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
- [x] **Live Query Tail**: Real-time visualization of incoming DNS requests. Multi-client support added to WebSockets and basic tail implemented in HTMX dashboard.
- [ ] **Advanced Data Visualization**: More detailed charts for top permitted/blocked domains and client activity over time.
- [x] **Network Topology Graph**: Interactive 2D visualization of connected clients and their DNS traffic patterns (Implemented via DNSServerVisualizer).
- [x] **Mobile-First Responsive Audit**: Core dashboard routes and critical UX flows hardened for mobile/touch usage, including responsive layouts and touch-friendly controls.
- [x] **Go-Native Frontend Migration**: HTMX Alpha dashboard implemented as a Proof of Concept with stats, logs, and resolution management. Zero NodeJS dependency achieved for this mode.

---

## 🟡 Medium Complexity (Core Enhancements)

### Advanced DNS Management
- [x] **DNS Caching Layer**: Intelligent in-memory DNS caching system implemented with TTL respect and UI toggle (On/Off).
- [x] **Local DNS & CNAME Records**: Support for A/AAAA/CNAME records with a dedicated management UI and database persistence.
- [x] **Allowlist / Whitelist Lists**: Full support for allowlists over riding blacklists.
- [x] **Wildcard Matching**: Introduced wildcard domain matching (e.g., `*.evil.com`) for both blocklists and allowlists using suffix matching.
- [x] **Regex Blocking**: Implemented Regular Expressions (Regex) support for advanced domain blacklisting and whitelisting.
- [x] **Conditional Forwarding**: Domain-specific upstream routing via `ConditionalForwarders` config + REST API (`GET/POST/DELETE /api/dns/forwarders`).

### System Architecture
- [ ] **Schema Migrations**: Introduce a migration runner (e.g., `golang-migrate`) for managing backend database schema updates across versions explicitly.
- [x] **Data Backup & Restore (Teleporter)**: ZIP-based export (`GET /api/teleporter/export`) and import (`POST /api/teleporter/import`) for settings and database.
- [x] **Remote Backups**: Implemented remote backup sync to AWS S3, mounted remote directories (NFS/SMB), and WebDAV with manual trigger + scheduled execution.
- [x] **Metrics & Observability**: Prometheus metrics exposed at `/metrics` for DNS latency (histogram), queries, blocks, cache hits, and forwarded queries. Compatible with Grafana.

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
- [x] **Group Management (Per-Client Blocking)**: Multi-group backend implemented (default + custom groups) with per-client IP/MAC assignments, group-scoped block/allow domains, and DNS policy enforcement integrated into runtime resolution.

### Platform & Scaling
- [ ] **High Availability / Synchronization**: Enable running multiple instances of `goaway` on the same network that can sync blocklists, DHCP leases, allowlists, and local DNS automatically for redundancy (Primary/Secondary setup).
- [ ] **Full Windows / macOS Support**: Move macOS and Windows support from "Beta" to "Full". This involves validating low-level networking behaviors, DHCP broadcasts, and path resolutions specific to these OS environments.
- [ ] **E2E & Integration Tests**: Set up Docker-based End-to-End integration tests simulating actual client queries, HTTP requests, and verifying database states dynamically.
