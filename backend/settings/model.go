package settings

import "time"

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
	Servers []UpstreamServer `yaml:"servers" json:"servers"`

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
}

type DNSConfig struct {
	Status                Status                 `yaml:"-" json:"status"`
	Address               string                 `yaml:"address" json:"address"`
	Gateway               string                 `yaml:"gateway" json:"gateway"`
	CacheEnabled          bool                   `yaml:"cacheEnabled" json:"cacheEnabled"`
	CacheTTL              int                    `yaml:"cacheTTL" json:"cacheTTL"`
	DNSSEC                DNSSECConfig           `yaml:"dnssec" json:"dnssec"`
	RateLimit             DNSRateLimitConfig     `yaml:"rateLimit" json:"rateLimit"`
	UDPSize               int                    `yaml:"udpSize" json:"udpSize"`
	TLS                   TLSConfig              `yaml:"tls" json:"tls"`
	Upstream              UpstreamConfig         `yaml:"upstream" json:"upstream"`
	Ports                 PortsConfig            `yaml:"ports" json:"ports"`
	ConditionalForwarders []ConditionalForwarder `yaml:"conditionalForwarders" json:"conditionalForwarders"`
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

type APIConfig struct {
	Port           int             `yaml:"port" json:"port"`
	Authentication bool            `yaml:"authentication" json:"authentication"`
	JWTSecret      string          `yaml:"jwtSecret" json:"-"`
	RateLimit      RateLimitConfig `yaml:"rateLimit" json:"rateLimit"`
}

type LoggingConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	Level   int  `yaml:"level" json:"level"`
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

type HighAvailabilityConfig struct {
	Enabled                bool      `yaml:"enabled" json:"enabled"`
	Mode                   string    `yaml:"mode" json:"mode"`                                   // "primary" or "replica"
	ReplicaSyncInterval    string    `yaml:"replicaSyncInterval" json:"replicaSyncInterval"`     // duration: "5m", "15m", "1h"
	PrimaryBackupProvider  string    `yaml:"primaryBackupProvider" json:"primaryBackupProvider"` // provider type: "s3", "webdav", "local"
	PrimaryBackupEndpoint  string    `yaml:"primaryBackupEndpoint" json:"primaryBackupEndpoint"` // endpoint/URL for Primary's remote backup
	PrimaryBackupBucket    string    `yaml:"primaryBackupBucket" json:"primaryBackupBucket"`     // S3 bucket or path
	PrimaryBackupRegion    string    `yaml:"primaryBackupRegion" json:"primaryBackupRegion"`     // S3 region (optional)
	PrimaryBackupAccessKey string    `yaml:"primaryBackupAccessKey" json:"-"`                    // credentials
	PrimaryBackupSecretKey string    `yaml:"primaryBackupSecretKey" json:"-"`
	PrimaryBackupUsername  string    `yaml:"primaryBackupUsername" json:"primaryBackupUsername"` // WebDAV/SMB credentials
	PrimaryBackupPassword  string    `yaml:"primaryBackupPassword" json:"-"`
	LastSyncTime           time.Time `yaml:"-" json:"lastSyncTime"` // last successful sync timestamp
}

type Config struct {
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
