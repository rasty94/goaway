export interface Root {
  dns: Dns;
  dhcp: Dhcp;
  api: Api;
  logging: Logging;
  misc: Misc;
}

export interface Dns {
  status: Status;
  address: string;
  gateway: string;
  cacheTTL: number;
  rateLimit: DnsRateLimit;
  udpSize: number;
  tls: Tls;
  upstream: Upstream;
  ports: Ports;
}

export interface DnsRateLimit {
  enabled: boolean;
  maxQueries: number;
  windowSeconds: number;
  blockDurationSeconds: number;
}

export interface Dhcp {
  enabled: boolean;
  address: string;
  interface: string;
  ipv4Enabled: boolean;
  ipv6Enabled: boolean;
  rangeStart: string;
  rangeEnd: string;
  leaseDuration: number;
  router: string;
  dnsServers: string[];
  domainSearch: string;
  ports: DhcpPorts;
}

export interface DhcpPorts {
  ipv4: number;
  ipv6: number;
}

export interface Status {
  pausedAt: string;
  pauseTime: string;
  paused: boolean;
}

export interface Tls {
  enabled: boolean;
  cert: string;
  key: string;
}

export interface Upstream {
  preferred: string;
  fallback: string[];
}

export interface Ports {
  udptcp: number;
  dot: number;
  doh: number;
}

export interface Api {
  port: number;
  authentication: boolean;
  rateLimit: RateLimit;
}

export interface RateLimit {
  enabled: boolean;
  maxTries: number;
  window: number;
}

export interface Logging {
  enabled: boolean;
  level: number;
}

export interface Misc {
  inAppUpdate: boolean;
  statisticsRetention: number;
  dashboard: boolean;
  scheduledBlacklistUpdates: boolean;
}

export interface SetModalsType {
  password: false;
  apiKey: false;
  importConfirm: false;
  notifications: false;
}
