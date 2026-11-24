package settings

import (
	"sync"
	"time"
)

type Status struct {
	PausedAt  time.Time `json:"pausedAt"`
	PauseTime time.Time `json:"pauseTime"`
	Paused    bool      `json:"paused"`
}

type TLSConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Cert    string `yaml:"cert" json:"cert"`
	Key     string `yaml:"key" json:"key"`
}

type UpstreamServer struct {
	Name     string `yaml:"name" json:"name"`
	Address  string `yaml:"address" json:"address"`
	Protocol string `yaml:"protocol" json:"protocol"` // "udp", "tcp", "dot", "doh", "doq"
	Enabled  bool   `yaml:"enabled" json:"enabled"`
}

type UpstreamConfig struct {
	Strategy string           `yaml:"strategy,omitempty" json:"strategy,omitempty"` // "strict" (default), "parallel_best", "random"
	Servers  []UpstreamServer `yaml:"servers" json:"servers"`

	// Legacy fields for migration
	Preferred string   `yaml:"preferred,omitempty" json:"preferred,omitempty"`
	Fallback  []string `yaml:"fallback,omitempty" json:"fallback,omitempty"`
}

type ConditionalForwarder struct {
	Domain   string `yaml:"domain" json:"domain"`
	Upstream string `yaml:"upstream" json:"upstream"`
}

type PortsConfig struct {
	TCPUDP int `yaml:"udptcp" json:"udptcp"`
	DoT    int `yaml:"dot" json:"dot"`
	DoH    int `yaml:"doh" json:"doh"`
	DoQ    int `yaml:"doq,omitempty" json:"doq,omitempty"`
}

type RedisCacheConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Address  string `yaml:"address" json:"address"`
	Password string `yaml:"password,omitempty" json:"-"`
	DB       int    `yaml:"db,omitempty" json:"db,omitempty"`
	Prefix   string `yaml:"prefix,omitempty" json:"prefix,omitempty"`
}

type DNSConfig struct {
	Status                Status                 `yaml:"-" json:"status"`
	Address               string                 `yaml:"address" json:"address"`
	Gateway               string                 `yaml:"gateway" json:"gateway"`
	CacheEnabled          bool                   `yaml:"cacheEnabled" json:"cacheEnabled"`
	CacheTTL              int                    `yaml:"cacheTTL" json:"cacheTTL"`
	RedisCache            RedisCacheConfig       `yaml:"redisCache,omitempty" json:"redisCache,omitempty"`
	EDEEnabled            bool                   `yaml:"edeEnabled" json:"edeEnabled"`
	DNSSEC                DNSSECConfig           `yaml:"dnssec" json:"dnssec"`
	RateLimit             DNSRateLimitConfig     `yaml:"rateLimit" json:"rateLimit"`
	UDPSize               int                    `yaml:"udpSize" json:"udpSize"`
	TLS                   TLSConfig              `yaml:"tls" json:"tls"`
	Upstream              UpstreamConfig         `yaml:"upstream" json:"upstream"`
	Ports                 PortsConfig            `yaml:"ports" json:"ports"`
	ConditionalForwarders []ConditionalForwarder `yaml:"conditionalForwarders" json:"conditionalForwarders"`
	BlockedQueryTypes     []string               `yaml:"blockedQueryTypes,omitempty" json:"blockedQueryTypes,omitempty"`
	Resolutions           map[string]string      `yaml:"resolution,omitempty" json:"resolution,omitempty"`
}

type DNSRateLimitConfig struct {
	Enabled              bool `yaml:"enabled" json:"enabled"`
	MaxQueries           int  `yaml:"maxQueries" json:"maxQueries"`
	WindowSeconds        int  `yaml:"windowSeconds" json:"windowSeconds"`
	BlockDurationSeconds int  `yaml:"blockDurationSeconds" json:"blockDurationSeconds"`
}

type DNSSECConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Mode    string `yaml:"mode" json:"mode"` // "off", "permissive", "strict"
}

type RateLimitConfig struct {
	Enabled  bool `yaml:"enabled" json:"enabled"`
	MaxTries int  `yaml:"maxTries" json:"maxTries"`
	Window   int  `yaml:"window" json:"window"`
}

type OIDCConfig struct {
	Enabled      bool     `yaml:"enabled" json:"enabled"`
	Issuer       string   `yaml:"issuer,omitempty" json:"issuer,omitempty"`
	ClientID     string   `yaml:"clientId,omitempty" json:"clientId,omitempty"`
	ClientSecret string   `yaml:"clientSecret,omitempty" json:"-"`
	RedirectURL  string   `yaml:"redirectUrl,omitempty" json:"redirectUrl,omitempty"`
	Scopes       []string `yaml:"scopes,omitempty" json:"scopes,omitempty"`
	RoleClaim    string   `yaml:"roleClaim,omitempty" json:"roleClaim,omitempty"`
	AdminGroups  []string `yaml:"adminGroups,omitempty" json:"adminGroups,omitempty"`
}

type APIConfig struct {
	Port           int             `yaml:"port" json:"port"`
	Authentication bool            `yaml:"authentication" json:"authentication"`
	ReadOnly       bool            `yaml:"readOnly,omitempty" json:"readOnly,omitempty"`
	JWTSecret      string          `yaml:"jwtSecret" json:"-"`
	RateLimit      RateLimitConfig `yaml:"rateLimit" json:"rateLimit"`
	OIDC           OIDCConfig      `yaml:"oidc,omitempty" json:"oidc,omitempty"`
}

type QueryLogExportConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Format  string `yaml:"format" json:"format"` // "json" (NDJSON) or "csv"
	Target  string `yaml:"target" json:"target"` // file path or "stdout"
}

type LoggingConfig struct {
	Enabled        bool                 `yaml:"enabled" json:"enabled"`
	Level          int                  `yaml:"level" json:"level"`
	QueryLogExport QueryLogExportConfig `yaml:"queryLogExport,omitempty" json:"queryLogExport,omitempty"`
}

type MiscConfig struct {
	InAppUpdate               bool `yaml:"inAppUpdate" json:"inAppUpdate"`
	StatisticsRetention       int  `yaml:"statisticsRetention" json:"statisticsRetention"`
	AnonymizeIP               bool `yaml:"anonymizeIP" json:"anonymizeIP"`
	Dashboard                 bool `yaml:"dashboard" json:"dashboard"`
	ScheduledBlacklistUpdates bool `yaml:"scheduledBlacklistUpdates" json:"scheduledBlacklistUpdates"`
}

type DHCPPortsConfig struct {
	IPv4 int `yaml:"ipv4" json:"ipv4"`
	IPv6 int `yaml:"ipv6" json:"ipv6"`
}

type DHCPConfig struct {
	Enabled       bool            `yaml:"enabled" json:"enabled"`
	Address       string          `yaml:"address" json:"address"`
	Interface     string          `yaml:"interface" json:"interface"`
	IPv4Enabled   bool            `yaml:"ipv4Enabled" json:"ipv4Enabled"`
	IPv6Enabled   bool            `yaml:"ipv6Enabled" json:"ipv6Enabled"`
	RangeStart    string          `yaml:"rangeStart" json:"rangeStart"`
	RangeEnd      string          `yaml:"rangeEnd" json:"rangeEnd"`
	RangeStartV6  string          `yaml:"rangeStartV6" json:"rangeStartV6"`
	RangeEndV6    string          `yaml:"rangeEndV6" json:"rangeEndV6"`
	LeaseDuration int             `yaml:"leaseDuration" json:"leaseDuration"`
	Router        string          `yaml:"router" json:"router"`
	DNSServers    []string        `yaml:"dnsServers" json:"dnsServers"`
	DNSServersV6  []string        `yaml:"dnsServersV6" json:"dnsServersV6"`
	DomainSearch  string          `yaml:"domainSearch" json:"domainSearch"`
	Ports         DHCPPortsConfig `yaml:"ports" json:"ports"`
}

type RemoteBackupConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	Provider  string `yaml:"provider" json:"provider"` // "s3", "webdav", "local"
	Endpoint  string `yaml:"endpoint" json:"endpoint"` // S3 endpoint or WebDAV URL or local path
	Bucket    string `yaml:"bucket" json:"bucket"`     // S3 bucket name
	Region    string `yaml:"region" json:"region"`     // S3 region
	AccessKey string `yaml:"accessKey" json:"-"`
	SecretKey string `yaml:"secretKey" json:"-"`
	Username  string `yaml:"username" json:"username"` // WebDAV / SMB username
	Password  string `yaml:"password" json:"-"`
	Schedule  string `yaml:"schedule" json:"schedule"` // "daily", "weekly", "manual"
}

type HaProxyConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	Port    int  `yaml:"port" json:"port"`
}

type VipConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	Interface string `yaml:"interface" json:"interface"`
	Address   string `yaml:"address" json:"address"`
}

type ClusterTLSConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	CACert     string `yaml:"caCert,omitempty" json:"caCert,omitempty"`
	ClientCert string `yaml:"clientCert,omitempty" json:"clientCert,omitempty"`
	ClientKey  string `yaml:"clientKey,omitempty" json:"clientKey,omitempty"`
}

type HighAvailabilityConfig struct {
	Enabled      bool             `yaml:"enabled" json:"enabled"`
	Mode         string           `yaml:"mode" json:"mode"`           // "primary" or "replica"
	ClusterID    string           `yaml:"clusterId" json:"clusterId"` // Unique ID for the cluster
	Priority     int              `yaml:"priority" json:"priority"`   // Higher priority = better candidate for Primary
	Peers        []string         `yaml:"peers" json:"peers"`         // List of peer node API endpoints (e.g. https://192.168.1.10:8080)
	HeartbeatURL string           `yaml:"heartbeatUrl" json:"heartbeatUrl"`
	Secret       string           `yaml:"secret" json:"-"` // Shared HMAC key; every heartbeat/replication call must be signed with it
	TLS          ClusterTLSConfig `yaml:"tls,omitempty" json:"tls,omitempty"`
	Proxy        HaProxyConfig    `yaml:"proxy" json:"proxy"`
	VIP          VipConfig        `yaml:"vip" json:"vip"`

	// Passive Pull-Sync (Phase 1 legacy/fallback)
	ReplicaSyncInterval    string `yaml:"replicaSyncInterval" json:"replicaSyncInterval"`
	PrimaryBackupProvider  string `yaml:"primaryBackupProvider" json:"primaryBackupProvider"`
	PrimaryBackupEndpoint  string `yaml:"primaryBackupEndpoint" json:"primaryBackupEndpoint"`
	PrimaryBackupBucket    string `yaml:"primaryBackupBucket" json:"primaryBackupBucket"`
	PrimaryBackupRegion    string `yaml:"primaryBackupRegion" json:"primaryBackupRegion"`
	PrimaryBackupAccessKey string `yaml:"primaryBackupAccessKey" json:"-"`
	PrimaryBackupSecretKey string `yaml:"primaryBackupSecretKey" json:"-"`
	PrimaryBackupUsername  string `yaml:"primaryBackupUsername" json:"primaryBackupUsername"`
	PrimaryBackupPassword  string `yaml:"primaryBackupPassword" json:"-"`

	LastSyncTime time.Time `yaml:"-" json:"lastSyncTime"`
}

type Config struct {
	mu               *sync.RWMutex          `yaml:"-" json:"-"`
	SchemaVersion    int                    `yaml:"schemaVersion" json:"schemaVersion"`
	BinaryPath       string                 `yaml:"-" json:"-"`
	DNS              DNSConfig              `yaml:"dns" json:"dns"`
	DHCP             DHCPConfig             `yaml:"dhcp" json:"dhcp"`
	API              APIConfig              `yaml:"api" json:"api"`
	Logging          LoggingConfig          `yaml:"logging" json:"logging"`
	Misc             MiscConfig             `yaml:"misc" json:"misc"`
	RemoteBackup     RemoteBackupConfig     `yaml:"remoteBackup" json:"remoteBackup"`
	HighAvailability HighAvailabilityConfig `yaml:"highAvailability" json:"highAvailability"`
}

var fallbackConfigMu sync.RWMutex

func (c *Config) rwMu() *sync.RWMutex {
	if c != nil && c.mu != nil {
		return c.mu
	}
	return &fallbackConfigMu
}

func (c *Config) RLockConfig()   { c.rwMu().RLock() }
func (c *Config) RUnlockConfig() { c.rwMu().RUnlock() }
func (c *Config) LockConfig()    { c.rwMu().Lock() }
func (c *Config) UnlockConfig()  { c.rwMu().Unlock() }

// DNSSnapshot returns a concurrency-safe copy of DNS settings with cloned slices
// for hot-path readers in the DNS resolver.
func (c *Config) DNSSnapshot() DNSConfig {
	if c == nil {
		return DNSConfig{}
	}
	c.RLockConfig()
	defer c.RUnlockConfig()

	snap := c.DNS
	if len(c.DNS.Upstream.Servers) > 0 {
		snap.Upstream.Servers = append([]UpstreamServer(nil), c.DNS.Upstream.Servers...)
	}
	if len(c.DNS.ConditionalForwarders) > 0 {
		snap.ConditionalForwarders = append([]ConditionalForwarder(nil), c.DNS.ConditionalForwarders...)
	}
	if len(c.DNS.BlockedQueryTypes) > 0 {
		snap.BlockedQueryTypes = append([]string(nil), c.DNS.BlockedQueryTypes...)
	}
	return snap
}
