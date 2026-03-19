# High Availability & Synchronization (Phase 1)

## Overview

GoAway now supports **High Availability (HA)** in Phase 1, enabling a **Primary-Replica passive synchronization** setup. This allows you to run multiple GoAway instances where a Replica automatically pulls configuration and database changes from a Primary instance for disaster recovery and redundancy.

## Concept

### Primary Instance
- **Role**: Master DNS sinkhole serving clients
- **Function**: Optionally pushes regular backups to remote storage (S3, WebDAV, or local)
- **Configuration**: Already supported via Remote Backup feature

### Replica Instance
- **Role**: Standby/secondary DNS sinkhole  
- **Function**: Periodically pulls latest backup from Primary's remote storage and imports settings
- **Auto-Sync**: Syncs on configurable intervals (default: 15 minutes)
- **Manual Sync**: Can be triggered via API endpoint anytime

## Configuration

### Step 1: Set Up Primary Instance

Configure Remote Backup on your Primary GoAway instance to push backups regularly:

```bash
POST /api/backup/config
{
  "enabled": true,
  "provider": "s3",  // or "webdav", "local"
  "endpoint": "s3.amazonaws.com",
  "bucket": "my-goaway-backups",
  "region": "us-west-1",
  "accessKey": "...",
  "secretKey": "...",
  "schedule": "daily"  // or "weekly", "manual"
}
```

**Supported providers:**
- `s3` - Amazon S3 or S3-compatible storage (MinIO, DigitalOcean Spaces, etc.)
- `webdav` - WebDAV servers (Nextcloud, OwnCloud, etc.)
- `local` - Local filesystem, NFS, or SMB mounted directories

### Step 2: Configure Replica Instance

On your Replica instance, enable HA mode and point it to the Primary's backup location:

```bash
POST /api/ha/config
{
  "enabled": true,
  "mode": "replica",  // Set to "replica" for passive sync
  "replicaSyncInterval": "15m",  // 5m, 15m, 1h, etc.
  "primaryBackupProvider": "s3",  // Must match Primary's provider
  "primaryBackupEndpoint": "s3.amazonaws.com",
  "primaryBackupBucket": "my-goaway-backups",  // Same bucket as Primary
  "primaryBackupRegion": "us-west-1",
  "primaryBackupAccessKey": "...",
  "primaryBackupSecretKey": "..."
}
```

### Step 3: Verify Configuration

Check Replica sync status via public endpoint:

```bash
GET /api/ha/status
```

Response:
```json
{
  "enabled": true,
  "mode": "replica",
  "configured": true,
  "lastSyncTime": "2026-03-18T10:45:00Z"
}
```

## API Endpoints

### Public Endpoint (no auth required)
- `GET /api/ha/status` - Get current HA status

### Protected Endpoints (authentication required)
- `GET /api/ha/config` - Get HA configuration (Primary + Secondary)
- `POST /api/ha/config` - Update HA configuration
- `POST /api/ha/sync-now` - Trigger manual synchronization

## How Synchronization Works

1. **Initial Sync**: When a Replica is enabled, it attempts an immediate sync on startup
2. **Scheduled Sync**: Background scheduler runs every `replicaSyncInterval` minutes
3. **Backup Download**: Latest `goaway-backup-*.zip` from Primary's remote storage
4. **Data Import**: ZIP contains:
  - `settings.json` - All configuration (DNS, DHCP, API, etc.)
  - `goaway.db` - Complete database with blocklists, leases, statistics, etc.
5. **Update Cycle**: Settings are applied immediately and database file is replaced for next startup; `LastSyncTime` is updated
6. **Error Handling**: Failed syncs are logged; retry on next cycle

## Usage Scenarios

### Scenario 1: Warm Standby
- **Primary**: Active, running `mode: "primary"` (default)
- **Replica**: Standby, syncing every 15 minutes
- **Failover**: If Primary fails, switch DNS clients to Replica IP manually or via load balancer

### Scenario 2: Geographic Redundancy
- **Primary**: Data center 1 (e.g., us-west)
- **Replica**: Data center 2 (e.g., us-east)
- **Network**: Both pull from shared S3 bucket for configuration sync

### Scenario 3: Regular Backups
- **Primary**: Pushes backups daily to S3
- **Replica**: Not needed initially; can be spun up anytime by pulling latest backup

## Limitations & Future (Phase 2+)

- **Passive Sync Only**: Replica does not push changes back to Primary  
- **Backup-Based**: All data flows through remote storage snapshots
- **Manual Setup**: No automatic failover yet; requires manual intervention
- **Single Provider**: Replica must use same remote storage provider as Primary
- **Security**: Credentials stored in `settings.yaml` (ensure proper file permissions)

### Planned for Phase 2:
- Automatic health monitoring & failover
- Active-Active bidirectional sync
- Real-time sync via WebSocket
- Peer-to-peer direct sync

## Troubleshooting

### Replica Not Syncing

1. Check replica HA status:
   ```bash
   curl http://replica-ip:8080/api/ha/status
   ```

2. Review logs for sync errors:
   ```bash
   docker logs <replica-container> | grep "\[HA/Replica\]"
   ```

3. Verify Primary backups exist in remote storage:
   - For S3: Check bucket for `goaway-backup-*.zip` files
   - For local: Check directory file list

4. Test credentials manually:
   - S3: Try uploading a test file with same access key
   - WebDAV: Test login with provided username/password

### Storage Issues

- **S3 Bucket Access Denied**: Ensure IAM policy allows `s3:GetObject`, `s3:ListBucket`
- **WebDAV Connection Timeout**: Check firewall rules; test with `curl` or `nc`
- **Local Path Not Found**: Verify mount is active; check permissions

## Example Deployment

See [docker-compose.yml](../docker-compose.yml) for basic Primary setup, and create a secondary with:

```yaml
goaway-replica:
  image: pommee/goaway:latest
  environment:
    - DNS_PORT=53
    - WEBSITE_PORT=8080
  volumes:
    - replica-config:/app/data
  command: >
    /bin/sh -c "
    curl -X POST http://primary:8080/api/ha/config \
      -H 'Content-Type: application/json' \
      -d '{...replica config...}' && \
    tail -f /dev/null"
```

## Notes

- Always test failover procedure in staging environment first
- Keep remote storage backups indefinitely or set retention per your policy
- Monitor Replica `lastSyncTime` to detect sync failures
- Authenticate replica config changes to prevent unauthorized HA setup
