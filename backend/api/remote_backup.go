package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"goaway/backend/settings"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func (api *API) registerRemoteBackupRoutes() {
	api.routes.GET("/backup/config", api.getRemoteBackupConfig)
	api.routes.POST("/backup/config", api.saveRemoteBackupConfig)
	api.routes.POST("/backup/push", api.pushRemoteBackup)
	api.startRemoteBackupScheduler()
}

func (api *API) getRemoteBackupConfig(c *gin.Context) {
	c.JSON(http.StatusOK, api.Config.RemoteBackup)
}

func (api *API) saveRemoteBackupConfig(c *gin.Context) {
	type RemoteBackupInput struct {
		Enabled   bool   `json:"enabled"`
		Provider  string `json:"provider"`
		Endpoint  string `json:"endpoint"`
		Bucket    string `json:"bucket"`
		Region    string `json:"region"`
		AccessKey string `json:"accessKey"`
		SecretKey string `json:"secretKey"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		Schedule  string `json:"schedule"`
	}

	var input RemoteBackupInput
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid backup configuration"})
		return
	}

	provider, err := normalizeProvider(input.Provider)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	schedule, err := normalizeSchedule(input.Schedule)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rb := &api.Config.RemoteBackup
	rb.Enabled = input.Enabled
	rb.Provider = provider
	rb.Endpoint = strings.TrimSpace(input.Endpoint)
	rb.Bucket = strings.TrimSpace(input.Bucket)
	rb.Region = strings.TrimSpace(input.Region)
	rb.Username = strings.TrimSpace(input.Username)
	rb.Schedule = schedule

	// Only overwrite secrets if provided.
	if input.AccessKey != "" {
		rb.AccessKey = input.AccessKey
	}
	if input.SecretKey != "" {
		rb.SecretKey = input.SecretKey
	}
	if input.Password != "" {
		rb.Password = input.Password
	}

	api.Config.Save()
	c.JSON(http.StatusOK, gin.H{"message": "Remote backup configuration saved"})
}

// pushRemoteBackup creates the teleporter ZIP and pushes to the configured remote.
//
//	@Summary Push Remote Backup
//	@Description Trigger an immediate backup push to the configured remote storage
//	@Tags backup
//	@Success 200
//	@Router /backup/push [post]
func (api *API) pushRemoteBackup(c *gin.Context) {
	provider, filename, err := api.pushRemoteBackupNow()
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not enabled") || strings.Contains(err.Error(), "unsupported") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Backup pushed successfully",
		"provider": provider,
		"filename": filename,
	})
}

func (api *API) pushRemoteBackupNow() (string, string, error) {
	rb := api.Config.RemoteBackup
	if !rb.Enabled {
		return "", "", fmt.Errorf("remote backup is not enabled")
	}

	provider, err := normalizeProvider(rb.Provider)
	if err != nil {
		return "", "", err
	}

	rb.Provider = provider

	zipData, filename, err := api.buildTeleporterZip()
	if err != nil {
		return "", "", err
	}

	switch provider {
	case "s3":
		if err := api.pushToS3(rb, zipData, filename); err != nil {
			return "", "", fmt.Errorf("S3 upload failed: %w", err)
		}
	case "webdav":
		if err := api.pushToWebDAV(rb, zipData, filename); err != nil {
			return "", "", fmt.Errorf("WebDAV upload failed: %w", err)
		}
	case "local":
		if err := api.pushToLocal(rb, zipData, filename); err != nil {
			return "", "", fmt.Errorf("local backup failed: %w", err)
		}
	default:
		return "", "", fmt.Errorf("unsupported provider: %s", provider)
	}

	log.Info("Remote backup pushed successfully to %s:%s", provider, rb.Endpoint)
	return provider, filename, nil
}

func (api *API) startRemoteBackupScheduler() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		lastSlot := ""
		for range ticker.C {
			rb := api.Config.RemoteBackup
			if !rb.Enabled {
				continue
			}

			slot, shouldRun := scheduledSlot(rb.Schedule, time.Now())
			if !shouldRun || slot == lastSlot {
				continue
			}

			if _, _, err := api.pushRemoteBackupNow(); err != nil {
				log.Error("Scheduled remote backup failed: %v", err)
				continue
			}

			lastSlot = slot
		}
	}()
}

func scheduledSlot(schedule string, now time.Time) (string, bool) {
	normalized, err := normalizeSchedule(schedule)
	if err != nil {
		return "", false
	}

	switch normalized {
	case "daily":
		return now.UTC().Format("2006-01-02"), true
	case "weekly":
		year, week := now.UTC().ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week), true
	default:
		return "", false
	}
}

func normalizeProvider(provider string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "s3":
		return "s3", nil
	case "webdav":
		return "webdav", nil
	case "local", "nfs", "smb":
		return "local", nil
	default:
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}
}

func normalizeSchedule(schedule string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(schedule)) {
	case "", "manual":
		return "manual", nil
	case "daily":
		return "daily", nil
	case "weekly":
		return "weekly", nil
	default:
		return "", fmt.Errorf("unsupported schedule: %s", schedule)
	}
}

// buildTeleporterZip generates the backup ZIP bytes and filename.
func (api *API) buildTeleporterZip() ([]byte, string, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	settingsBytes, err := json.MarshalIndent(api.Config, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("failed to serialize settings: %w", err)
	}

	sf, err := w.Create("settings.json")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create settings entry: %w", err)
	}
	if _, err := sf.Write(settingsBytes); err != nil {
		return nil, "", fmt.Errorf("failed to write settings: %w", err)
	}

	tempPath := fmt.Sprintf("/tmp/goaway_teleporter_%d.db", time.Now().UnixNano())
	if err := api.DBConn.Exec(fmt.Sprintf("VACUUM INTO '%s';", tempPath)).Error; err != nil {
		return nil, "", fmt.Errorf("failed to export database: %w", err)
	}
	defer func() { _ = removeFile(tempPath) }()

	dbBytes, err := readFile(tempPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read database export: %w", err)
	}

	df, err := w.Create("goaway.db")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create db entry: %w", err)
	}
	if _, err := df.Write(dbBytes); err != nil {
		return nil, "", fmt.Errorf("failed to write db: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to finalize zip: %w", err)
	}

	filename := fmt.Sprintf("goaway-backup-%s.zip", time.Now().Format("2006-01-02T15-04-05"))
	return buf.Bytes(), filename, nil
}

// pushToS3 uploads to an S3 bucket using static credentials.
func (api *API) pushToS3(rb settings.RemoteBackupConfig, data []byte, filename string) error {
	if rb.Bucket == "" {
		return fmt.Errorf("bucket is required for s3 provider")
	}
	if rb.AccessKey == "" || rb.SecretKey == "" {
		return fmt.Errorf("accessKey and secretKey are required for s3 provider")
	}

	endpoint := strings.TrimSpace(rb.Endpoint)
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
		Creds:  credentials.NewStaticV4(rb.AccessKey, rb.SecretKey, ""),
		Secure: secure,
		Region: rb.Region,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize s3 client: %w", err)
	}

	_, err = client.PutObject(
		context.Background(),
		rb.Bucket,
		filename,
		bytes.NewReader(data),
		int64(len(data)),
		minio.PutObjectOptions{ContentType: "application/zip"},
	)
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	return nil
}

// pushToWebDAV uploads via HTTP PUT to a WebDAV server.
func (api *API) pushToWebDAV(rb settings.RemoteBackupConfig, data []byte, filename string) error {
	if rb.Endpoint == "" {
		return fmt.Errorf("endpoint is required for webdav provider")
	}

	url := strings.TrimSuffix(rb.Endpoint, "/") + "/" + filename

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/zip")
	req.ContentLength = int64(len(data))

	if rb.Username != "" {
		req.SetBasicAuth(rb.Username, rb.Password)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("WebDAV PUT failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("WebDAV returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// pushToLocal copies the backup ZIP to a local directory (NFS/SMB mount path).
func (api *API) pushToLocal(rb settings.RemoteBackupConfig, data []byte, filename string) error {
	if rb.Endpoint == "" {
		return fmt.Errorf("local endpoint (directory path) is not configured")
	}

	if err := os.MkdirAll(rb.Endpoint, 0750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	dest := filepath.Join(rb.Endpoint, filename)
	if err := os.WriteFile(dest, data, 0640); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}
