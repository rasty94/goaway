package sync

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	synchronization "sync"
	"time"

	"goaway/backend/logging"
	"goaway/backend/settings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var log = logging.GetLogger()

// Importer defines the interface for importing teleporter backups
type Importer interface {
	ImportTeleporterData(reader io.Reader, size int64) error
}

// ReplicaSyncManager handles backup synchronization from Primary to Replica
type ReplicaSyncManager struct {
	config   *settings.Config
	importer Importer
	stopCh   chan struct{}
	stopOnce synchronization.Once
	interval time.Duration
}

// NewReplicaSyncManager creates a new replica sync manager for pulling backups
// from the Primary instance's remote storage.
func NewReplicaSyncManager(config *settings.Config, importer Importer) *ReplicaSyncManager {
	interval, _ := time.ParseDuration(config.HighAvailability.ReplicaSyncInterval)
	if interval == 0 {
		interval = 15 * time.Minute // default
	}

	return &ReplicaSyncManager{
		config:   config,
		importer: importer,
		stopCh:   make(chan struct{}),
		interval: interval,
	}
}

// Start begins the replica sync scheduler.
func (rsm *ReplicaSyncManager) Start() {
	if !rsm.config.HighAvailability.Enabled ||
		rsm.config.HighAvailability.Mode != "replica" {
		log.Info("[HA/Replica] Sync disabled or not in replica mode")
		return
	}

	log.Info("[HA/Replica] Starting sync scheduler (interval: %v)", rsm.interval)

	go func() {
		ticker := time.NewTicker(rsm.interval)
		defer ticker.Stop()

		// Perform initial sync on startup
		if err := rsm.syncNow(); err != nil {
			log.Error("[HA/Replica] Initial sync failed: %v", err)
		}

		for {
			select {
			case <-rsm.stopCh:
				log.Info("[HA/Replica] Sync scheduler stopped")
				return
			case <-ticker.C:
				if err := rsm.syncNow(); err != nil {
					log.Error("[HA/Replica] Sync cycle failed: %v", err)
				}
			}
		}
	}()
}

// Stop halts the replica sync scheduler.
func (rsm *ReplicaSyncManager) Stop() {
	rsm.stopOnce.Do(func() {
		close(rsm.stopCh)
	})
}

// SyncNow is a public method to trigger a manual sync cycle.
func (rsm *ReplicaSyncManager) SyncNow() error {
	return rsm.syncNow()
}

// syncNow performs a single sync cycle: download latest backup from Primary
// and import via Teleporter.
func (rsm *ReplicaSyncManager) syncNow() error {
	ha := rsm.config.HighAvailability
	if !ha.Enabled || ha.Mode != "replica" {
		return fmt.Errorf("replica sync not enabled")
	}

	log.Info("[HA/Replica] Starting sync cycle")

	// Download backup from Primary's remote storage
	backupData, err := rsm.downloadPrimaryBackup()
	if err != nil {
		return fmt.Errorf("failed to download primary backup: %w", err)
	}

	// Import backup via Teleporter
	if err := rsm.importBackup(backupData); err != nil {
		return fmt.Errorf("failed to import backup: %w", err)
	}

	rsm.config.HighAvailability.LastSyncTime = time.Now().UTC()
	rsm.config.Save()

	log.Info("[HA/Replica] Sync cycle completed successfully")
	return nil
}

// downloadPrimaryBackup fetches the latest backup from the Primary's remote storage.
func (rsm *ReplicaSyncManager) downloadPrimaryBackup() ([]byte, error) {
	ha := rsm.config.HighAvailability
	provider := strings.ToLower(strings.TrimSpace(ha.PrimaryBackupProvider))

	switch provider {
	case "s3":
		return rsm.downloadFromS3()
	case "webdav":
		return rsm.downloadFromWebDAV()
	case "local":
		return rsm.downloadFromLocal()
	default:
		return nil, fmt.Errorf("unsupported backup provider: %s", provider)
	}
}

// downloadFromS3 retrieves the latest backup from an S3 bucket.
func (rsm *ReplicaSyncManager) downloadFromS3() ([]byte, error) {
	ha := rsm.config.HighAvailability
	if ha.PrimaryBackupBucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	if ha.PrimaryBackupAccessKey == "" || ha.PrimaryBackupSecretKey == "" {
		return nil, fmt.Errorf("s3 credentials are required")
	}

	endpoint := strings.TrimSpace(ha.PrimaryBackupEndpoint)
	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
	}

	secure := true
	if strings.HasPrefix(strings.ToLower(endpoint), "http://") {
		secure = false
	}
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(ha.PrimaryBackupAccessKey, ha.PrimaryBackupSecretKey, ""),
		Secure: secure,
		Region: ha.PrimaryBackupRegion,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize s3 client: %w", err)
	}

	// List objects to find the most recent backup (assuming naming convention: goaway-backup-TIMESTAMP.zip)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	objectList := client.ListObjects(ctx, ha.PrimaryBackupBucket, minio.ListObjectsOptions{
		Prefix:    "goaway-backup-",
		Recursive: false,
	})

	var latestObject minio.ObjectInfo
	for obj := range objectList {
		if obj.Err != nil {
			return nil, fmt.Errorf("s3 list error: %w", obj.Err)
		}
		if obj.Size > 0 && obj.LastModified.After(latestObject.LastModified) {
			latestObject = obj
		}
	}

	if latestObject.Key == "" {
		return nil, fmt.Errorf("no backup found in s3 bucket")
	}

	log.Info("[HA/Replica] Downloading backup from S3: %s", latestObject.Key)

	obj, err := client.GetObject(ctx, ha.PrimaryBackupBucket, latestObject.Key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get s3 object: %w", err)
	}
	defer obj.Close()

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to read s3 object: %w", err)
	}

	return data, nil
}

// downloadFromWebDAV retrieves the latest backup from a WebDAV server.
func (rsm *ReplicaSyncManager) downloadFromWebDAV() ([]byte, error) {
	ha := rsm.config.HighAvailability
	baseURL := strings.TrimSpace(ha.PrimaryBackupEndpoint)
	if baseURL == "" {
		return nil, fmt.Errorf("webdav endpoint is required")
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid webdav endpoint: %w", err)
	}
	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "https"
	}

	davPath := strings.TrimSpace(ha.PrimaryBackupBucket)
	if davPath != "" {
		parsedURL.Path = filepath.ToSlash(filepath.Join(parsedURL.Path, davPath))
	}
	if !strings.HasSuffix(parsedURL.Path, "/") {
		parsedURL.Path += "/"
	}

	propfindReq, err := http.NewRequest(http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create webdav request: %w", err)
	}
	propfindReq.Method = "PROPFIND"
	propfindReq.Header.Set("Depth", "1")
	propfindReq.Header.Set("Content-Type", "application/xml")
	if ha.PrimaryBackupUsername != "" || ha.PrimaryBackupPassword != "" {
		propfindReq.SetBasicAuth(ha.PrimaryBackupUsername, ha.PrimaryBackupPassword)
	}

	resp, err := http.DefaultClient.Do(propfindReq)
	if err != nil {
		return nil, fmt.Errorf("webdav propfind failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webdav propfind returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read webdav response: %w", err)
	}

	latestPath, err := latestBackupFromWebDAV(body)
	if err != nil {
		return nil, err
	}

	fileURL, err := parsedURL.Parse(latestPath)
	if err != nil {
		return nil, fmt.Errorf("invalid webdav backup path: %w", err)
	}

	log.Info("[HA/Replica] Downloading backup from WebDAV: %s", fileURL.String())

	getReq, err := http.NewRequest(http.MethodGet, fileURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create webdav get request: %w", err)
	}
	if ha.PrimaryBackupUsername != "" || ha.PrimaryBackupPassword != "" {
		getReq.SetBasicAuth(ha.PrimaryBackupUsername, ha.PrimaryBackupPassword)
	}

	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		return nil, fmt.Errorf("webdav get failed: %w", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webdav get returned status %d", getResp.StatusCode)
	}

	data, err := io.ReadAll(getResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read webdav backup: %w", err)
	}

	return data, nil
}

// downloadFromLocal retrieves the latest backup from a local/mounted directory.
func (rsm *ReplicaSyncManager) downloadFromLocal() ([]byte, error) {
	ha := rsm.config.HighAvailability
	baseDir := strings.TrimSpace(ha.PrimaryBackupEndpoint)
	if baseDir == "" {
		baseDir = strings.TrimSpace(ha.PrimaryBackupBucket)
	}
	if baseDir == "" {
		return nil, fmt.Errorf("local backup directory is required")
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read local backup directory: %w", err)
	}

	type candidate struct {
		path    string
		modTime time.Time
	}

	candidates := make([]candidate, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "goaway-backup-") || !strings.HasSuffix(name, ".zip") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		candidates = append(candidates, candidate{
			path:    filepath.Join(baseDir, name),
			modTime: info.ModTime(),
		})
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no backup found in local directory")
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	latest := candidates[0]
	log.Info("[HA/Replica] Downloading backup from local directory: %s", latest.path)

	data, err := os.ReadFile(latest.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read local backup file: %w", err)
	}

	return data, nil
}

// importBackup applies the downloaded backup via Teleporter import logic.
func (rsm *ReplicaSyncManager) importBackup(zipData []byte) error {
	if len(zipData) == 0 {
		return fmt.Errorf("backup data is empty")
	}

	// Use the importer's teleporter import logic
	// This reuses the same ZIP parsing and settings restoration code
	return rsm.importer.ImportTeleporterData(bytes.NewReader(zipData), int64(len(zipData)))
}

// Status returns current sync information.
func (rsm *ReplicaSyncManager) Status() map[string]interface{} {
	return map[string]interface{}{
		"enabled":      rsm.config.HighAvailability.Enabled,
		"mode":         rsm.config.HighAvailability.Mode,
		"interval":     rsm.config.HighAvailability.ReplicaSyncInterval,
		"lastSyncTime": rsm.config.HighAvailability.LastSyncTime,
		"configured":   rsm.config.HighAvailability.PrimaryBackupProvider != "",
	}
}

type webDAVMultiStatus struct {
	Responses []webDAVResponse `xml:"response"`
}

type webDAVResponse struct {
	Href     string            `xml:"href"`
	PropStat []webDAVPropStats `xml:"propstat"`
}

type webDAVPropStats struct {
	Prop webDAVProp `xml:"prop"`
}

type webDAVProp struct {
	ContentLength string `xml:"getcontentlength"`
	LastModified  string `xml:"getlastmodified"`
}

func latestBackupFromWebDAV(payload []byte) (string, error) {
	var result webDAVMultiStatus
	if err := xml.Unmarshal(payload, &result); err != nil {
		return "", fmt.Errorf("failed to parse webdav response: %w", err)
	}

	type candidate struct {
		href     string
		modTime  time.Time
		hasMTime bool
	}

	candidates := make([]candidate, 0)
	for _, response := range result.Responses {
		name := filepath.Base(response.Href)
		if !strings.HasPrefix(name, "goaway-backup-") || !strings.HasSuffix(name, ".zip") {
			continue
		}

		item := candidate{href: response.Href}
		for _, prop := range response.PropStat {
			if strings.TrimSpace(prop.Prop.ContentLength) == "" {
				continue
			}

			size, err := strconv.ParseInt(strings.TrimSpace(prop.Prop.ContentLength), 10, 64)
			if err != nil || size <= 0 {
				continue
			}

			if ts := strings.TrimSpace(prop.Prop.LastModified); ts != "" {
				if parsed, err := time.Parse(time.RFC1123, ts); err == nil {
					item.modTime = parsed
					item.hasMTime = true
				}
			}
		}

		candidates = append(candidates, item)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no backup found on webdav endpoint")
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].hasMTime && candidates[j].hasMTime {
			return candidates[i].modTime.After(candidates[j].modTime)
		}
		return candidates[i].href > candidates[j].href
	})

	return candidates[0].href, nil
}
