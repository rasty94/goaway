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

type UpstreamConfig struct {
	Preferred string   `yaml:"preferred" json:"preferred"`
	Fallback  []string `yaml:"fallback" json:"fallback"`
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
	Status                 Status                 `yaml:"-" json:"status"`
	Address                string                 `yaml:"address" json:"address"`
	Gateway                string                 `yaml:"gateway" json:"gateway"`
	CacheEnabled           bool                   `yaml:"cacheEnabled" json:"cacheEnabled"`
	CacheTTL               int                    `yaml:"cacheTTL" json:"cacheTTL"`
	UDPSize                int                    `yaml:"udpSize" json:"udpSize"`
	TLS                    TLSConfig              `yaml:"tls" json:"tls"`
	Upstream               UpstreamConfig         `yaml:"upstream" json:"upstream"`
	Ports                  PortsConfig            `yaml:"ports" json:"ports"`
	ConditionalForwarders  []ConditionalForwarder `yaml:"conditionalForwarders" json:"conditionalForwarders"`
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
	Dashboard                 bool `yaml:"dashboard" json:"dashboard"`
	ScheduledBlacklistUpdates bool `yaml:"scheduledBlacklistUpdates" json:"scheduledBlacklistUpdates"`
}

type RemoteBackupConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	Provider   string `yaml:"provider" json:"provider"`   // "s3", "webdav", "local"
	Endpoint   string `yaml:"endpoint" json:"endpoint"`   // S3 endpoint or WebDAV URL or local path
	Bucket     string `yaml:"bucket" json:"bucket"`       // S3 bucket name
	Region     string `yaml:"region" json:"region"`       // S3 region
	AccessKey  string `yaml:"accessKey" json:"-"`
	SecretKey  string `yaml:"secretKey" json:"-"`
	Username   string `yaml:"username" json:"username"`   // WebDAV / SMB username
	Password   string `yaml:"password" json:"-"`
	Schedule   string `yaml:"schedule" json:"schedule"`   // "daily", "weekly", "manual"
}

type Config struct {
	BinaryPath    string              `yaml:"-" json:"-"`
	DNS           DNSConfig           `yaml:"dns" json:"dns"`
	API           APIConfig           `yaml:"api" json:"api"`
	Logging       LoggingConfig       `yaml:"logging" json:"logging"`
	Misc          MiscConfig          `yaml:"misc" json:"misc"`
	RemoteBackup  RemoteBackupConfig  `yaml:"remoteBackup" json:"remoteBackup"`
}
