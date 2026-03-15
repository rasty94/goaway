package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"goaway/backend/settings"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (api *API) registerConditionalForwarderRoutes() {
	api.routes.GET("/dns/forwarders", api.getConditionalForwarders)
	api.routes.POST("/dns/forwarders", api.addConditionalForwarder)
	api.routes.DELETE("/dns/forwarders", api.deleteConditionalForwarder)
}

func (api *API) getConditionalForwarders(c *gin.Context) {
	forwarders := api.Config.DNS.ConditionalForwarders
	if forwarders == nil {
		forwarders = []settings.ConditionalForwarder{}
	}
	c.JSON(http.StatusOK, forwarders)
}

func (api *API) addConditionalForwarder(c *gin.Context) {
	var cf settings.ConditionalForwarder
	if err := c.BindJSON(&cf); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid forwarder data"})
		return
	}
	if cf.Domain == "" || cf.Upstream == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain and upstream are required"})
		return
	}

	api.Config.DNS.ConditionalForwarders = append(api.Config.DNS.ConditionalForwarders, cf)
	api.Config.Save()
	log.Info("Added conditional forwarder: %s -> %s", cf.Domain, cf.Upstream)
	c.JSON(http.StatusCreated, cf)
}

func (api *API) deleteConditionalForwarder(c *gin.Context) {
	domain := c.Query("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain query param is required"})
		return
	}

	forwarders := api.Config.DNS.ConditionalForwarders
	updated := forwarders[:0]
	for _, cf := range forwarders {
		if cf.Domain != domain {
			updated = append(updated, cf)
		}
	}
	api.Config.DNS.ConditionalForwarders = updated
	api.Config.Save()
	log.Info("Removed conditional forwarder for domain: %s", domain)
	c.Status(http.StatusOK)
}

// ─── Backup & Restore (Teleporter) ──────────────────────────────────────────

func (api *API) registerTeleporterRoutes() {
	api.routes.GET("/teleporter/export", api.teleporterExport)
	api.routes.POST("/teleporter/import", api.teleporterImport)
}

// teleporterExport creates a ZIP archive containing the database and settings.yaml
//
//	@Summary Teleporter Export
//	@Description Export settings and database as a ZIP backup
//	@Tags teleporter
//	@Produce  application/zip
//	@Success 200
//	@Router /teleporter/export [get]
func (api *API) teleporterExport(c *gin.Context) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// -- Settings
	settingsBytes, err := json.MarshalIndent(api.Config, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize settings"})
		return
	}
	sf, err := w.Create("settings.json")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create settings entry"})
		return
	}
	if _, err := sf.Write(settingsBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write settings"})
		return
	}

	// -- Database (SQLite via VACUUM INTO temp and stream)
	tempPath := fmt.Sprintf("/tmp/goaway_teleporter_%d.db", time.Now().UnixNano())
	if err := api.DBConn.Exec(fmt.Sprintf("VACUUM INTO '%s';", tempPath)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export database"})
		return
	}
	defer func() { _ = removeFile(tempPath) }()

	dbBytes, err := readFile(tempPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read database export"})
		return
	}

	df, err := w.Create("goaway.db")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create db entry"})
		return
	}
	if _, err := df.Write(dbBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write db"})
		return
	}

	if err := w.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to finalize zip"})
		return
	}

	filename := fmt.Sprintf("goaway-backup-%s.zip", time.Now().Format("2006-01-02T15-04-05"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

// teleporterImport restores settings from an uploaded ZIP backup
//
//	@Summary Teleporter Import
//	@Description Import settings from a ZIP backup
//	@Tags teleporter
//	@Accept  multipart/form-data
//	@Param   backup  formData  file   true  "Backup ZIP file"
//	@Success 200
//	@Router /teleporter/import [post]
func (api *API) teleporterImport(c *gin.Context) {
	file, header, err := c.Request.FormFile("backup")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "backup file is required"})
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read uploaded file"})
		return
	}

	zr, err := zip.NewReader(bytes.NewReader(data), header.Size)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid zip file"})
		return
	}

	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to open %s in zip", f.Name)})
			return
		}
		content, _ := io.ReadAll(rc)
		_ = rc.Close()

		if f.Name == "settings.json" {
			var imported settings.Config
			if err := json.Unmarshal(content, &imported); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid settings.json in backup"})
				return
			}
			api.Config.Update(imported)
			log.Info("Settings restored from teleporter backup")
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Backup imported. Settings restored. Restart may be required for full effect."})
}
