// Minimal API responses so pages can render; shapes follow the backend handlers.
export const settings = {
  dns: {
    status: { pausedAt: "", pauseTime: "", paused: false },
    address: "0.0.0.0",
    gateway: "192.168.0.1:53",
    cacheTTL: 3600,
    rateLimit: { enabled: false, maxQueries: 120, windowSeconds: 60, blockDurationSeconds: 300 },
    udpSize: 512,
    tls: { enabled: false, cert: "", key: "" },
    upstream: { preferred: "8.8.8.8:53", fallback: ["1.1.1.1:53"] },
    ports: { udptcp: 53, dot: 853, doh: 443 }
  },
  dhcp: {
    enabled: false,
    address: "0.0.0.0",
    interface: "",
    ipv4Enabled: true,
    ipv6Enabled: false,
    rangeStart: "192.168.0.100",
    rangeEnd: "192.168.0.200",
    leaseDuration: 86400,
    router: "192.168.0.1",
    dnsServers: [],
    domainSearch: "",
    ports: { ipv4: 67, ipv6: 547 }
  },
  api: { port: 8080, authentication: true, rateLimit: { enabled: true, maxTries: 5, window: 5 } },
  logging: { enabled: true, level: 1 },
  misc: { inAppUpdate: false, statisticsRetention: 7, dashboard: true, scheduledBlacklistUpdates: true }
};

const now = Date.parse("2026-11-25T08:00:00Z");
const minutes = (n: number) => Array.from({ length: n }, (_, i) => now - (n - i) * 60_000);

export const apiDefaults: Record<string, unknown> = {
  settings,
  lists: {},
  server: {
    cpuTemp: 45,
    cpuUsage: 12.5,
    dbSize: 3.2,
    portDNS: 53,
    portWebsite: 8080,
    totalMem: 8,
    usedMem: 2,
    usedMemPercentage: 25,
    version: "1.0.0",
    commit: "abc1234",
    date: "2026-11-25T00:00:00Z"
  },
  dnsMetrics: { allowed: 90, blocked: 10, cached: 30, clients: 2, domainBlockLen: 1500, percentageBlocked: 10, total: 100 },
  clients: [
    { ip: "192.0.2.10", lastSeen: "2026-11-25T07:59:00Z", name: "laptop", mac: "aa:bb:cc:dd:ee:ff", vendor: "Acme" },
    { ip: "192.0.2.11", lastSeen: "2026-11-25T07:00:00Z", name: "phone", mac: "", vendor: "" }
  ],
  audit: [],
  notifications: { notifications: [{ id: 1, severity: "info", category: "system", text: "Lists updated", read: false, createdAt: "2026-11-25T07:00:00Z" }], total: 1, page: 1, limit: 50, totalPages: 1 },
  queryTimestamps: minutes(10).map((start, i) => ({ start, blocked: i % 3 === 0, cached: i % 2 === 0, allowed: true })),
  responseSizeTimestamps: minutes(10).map((start) => ({
    start,
    total_size_bytes: 4000,
    avg_response_size_bytes: 80,
    min_response_size_bytes: 40,
    max_response_size_bytes: 200
  })),
  topBlockedDomains: [{ name: "ads.example", hits: 12, frequency: 60 }],
  topPermittedDomains: [{ name: "ok.example", hits: 40, frequency: 80 }],
  topClients: [{ client: "192.0.2.10", clientName: "laptop", requestCount: 50, frequency: 50 }],
  topDestinations: { topDestinations: [{ name: "example.com", hits: 30 }] },
  queryTypes: [{ queryType: "A", count: 70 }, { queryType: "AAAA", count: 30 }],
  upstreams: {
    upstreams: [{ upstreamName: "Google", dnsPing: "5ms", icmpPing: "4ms", name: "dns.google", preferred: true, upstream: "8.8.8.8:53" }]
  },
  alert: { discord: { enabled: false, name: "", webhook: "" } },
  "dhcp/status": { enabled: true, running: true, ipv4Enabled: true, ipv6Enabled: false, leaseCount: 1 },
  "dhcp/leases": [{ id: 1, mac: "aa:bb:cc:dd:ee:01", ip: "192.168.0.50", hostname: "printer", enabled: true }],
  "dhcp/activeLeases": [{ id: 2, mac: "aa:bb:cc:dd:ee:02", ip: "192.168.0.120", hostname: "tablet", expiresAt: "2026-11-26T00:00:00Z" }],
  users: [{ username: "admin", role: "admin" }, { username: "bob", role: "viewer" }],
  whitelist: [],
  resolutions: [{ domain: "nas.lan", value: "192.168.0.5", type: "A" }],
  prefetch: [{ domain: "example.com", refresh: 300, queryType: 1 }],
  "ha/cluster": {
    selfRole: "primary",
    activeNodes: 1,
    clusterId: "c1",
    nodes: [{ id: "n1", address: "10.0.0.1", role: "primary", status: "healthy", priority: 100, lastSeen: "2026-11-25T07:59:00Z", latencyMs: 1, unreachable: false, pendingEvents: 0, needsResync: false }]
  },
  apiKey: []
};
