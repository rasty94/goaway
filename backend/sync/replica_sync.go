package sync

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
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
	close(rsm.stopCh)
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
	// TODO: Implement WebDAV download logic
	// For now, return a placeholder error
	return nil, fmt.Errorf("webdav replica sync not yet implemented")
}

// downloadFromLocal retrieves the latest backup from a local/mounted directory.
func (rsm *ReplicaSyncManager) downloadFromLocal() ([]byte, error) {
	// TODO: Implement local/NFS/SMB download logic
	// For now, return a placeholder error
	return nil, fmt.Errorf("local directory replica sync not yet implemented")
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
