import { useEffect, useState } from "react";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";

import { GetRequest, PostRequest } from "@/util";
import { SettingRow } from "./SettingsRow";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from "@/components/ui/select";

type RemoteBackupProvider = "s3" | "webdav" | "local";
type RemoteBackupSchedule = "manual" | "daily" | "weekly";

type RemoteBackupConfig = {
  enabled: boolean;
  provider: RemoteBackupProvider;
  endpoint: string;
  bucket: string;
  region: string;
  accessKey: string;
  secretKey: string;
  username: string;
  password: string;
  schedule: RemoteBackupSchedule;
};

const defaultConfig: RemoteBackupConfig = {
  enabled: false,
  provider: "local",
  endpoint: "",
  bucket: "",
  region: "",
  accessKey: "",
  secretKey: "",
  username: "",
  password: "",
  schedule: "manual"
};

export function RemoteBackupSection() {
  const { t } = useTranslation();
  const [config, setConfig] = useState<RemoteBackupConfig>(defaultConfig);
  const [isSaving, setIsSaving] = useState(false);
  const [isSyncing, setIsSyncing] = useState(false);

  useEffect(() => {
    const fetchRemoteBackupConfig = async () => {
      const [status, response] = await GetRequest("backup/config", true);
      if (status !== 200 || !response) {
        return;
      }

      setConfig((prev) => ({
        ...prev,
        ...response,
        provider:
          response.provider === "s3" ||
          response.provider === "webdav" ||
          response.provider === "local"
            ? response.provider
            : "local",
        schedule:
          response.schedule === "daily" ||
          response.schedule === "weekly" ||
          response.schedule === "manual"
            ? response.schedule
            : "manual"
      }));
    };

    fetchRemoteBackupConfig();
  }, []);

  const saveConfig = async () => {
    setIsSaving(true);
    const payload = {
      ...config,
      endpoint: config.endpoint.trim(),
      bucket: config.bucket.trim(),
      region: config.region.trim(),
      username: config.username.trim()
    };

    const [status] = await PostRequest("backup/config", payload);
    setIsSaving(false);

    if (status === 200) {
      toast.success(t("settings.remoteBackup.toasts.configSaved"));
    }
  };

  const pushNow = async () => {
    setIsSyncing(true);
    const [status, response] = await PostRequest("backup/push", {});
    setIsSyncing(false);

    if (status === 200) {
      toast.success(t("settings.remoteBackup.toasts.syncDone"), {
        description: response?.filename
          ? t("settings.remoteBackup.toasts.file", { filename: response.filename })
          : undefined
      });
    }
  };

  const endpointPlaceholder =
    config.provider === "s3"
      ? "s3.amazonaws.com or custom endpoint"
      : config.provider === "webdav"
      ? "https://webdav.example.com/backups"
      : "/mnt/backup-share";

  return (
    <div className="space-y-4">
      <SettingRow
        title={t("settings.remoteBackup.enableTitle")}
        description={t("settings.remoteBackup.enableDescription")}
        action={
          <div className="flex w-full justify-start md:justify-end">
            <Switch
              checked={config.enabled}
              onCheckedChange={(enabled) =>
                setConfig((prev) => ({ ...prev, enabled }))
              }
            />
          </div>
        }
      />

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="remote-provider">{t("settings.remoteBackup.provider")}</Label>
          <Select
            value={config.provider}
            onValueChange={(provider: RemoteBackupProvider) =>
              setConfig((prev) => ({ ...prev, provider }))
            }
          >
            <SelectTrigger id="remote-provider">
              <SelectValue placeholder={t("settings.remoteBackup.selectProvider")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="s3">AWS S3</SelectItem>
              <SelectItem value="webdav">WebDAV</SelectItem>
              <SelectItem value="local">
                {t("settings.remoteBackup.providerLocal")}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="space-y-2">
          <Label htmlFor="remote-schedule">{t("settings.remoteBackup.schedule")}</Label>
          <Select
            value={config.schedule}
            onValueChange={(schedule: RemoteBackupSchedule) =>
              setConfig((prev) => ({ ...prev, schedule }))
            }
          >
            <SelectTrigger id="remote-schedule">
              <SelectValue placeholder={t("settings.remoteBackup.selectSchedule")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="manual">{t("settings.remoteBackup.manualOnly")}</SelectItem>
              <SelectItem value="daily">{t("settings.remoteBackup.daily")}</SelectItem>
              <SelectItem value="weekly">{t("settings.remoteBackup.weekly")}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor="remote-endpoint">
          {config.provider === "local"
            ? t("settings.remoteBackup.directoryPath")
            : t("settings.remoteBackup.endpoint")}
        </Label>
        <Input
          id="remote-endpoint"
          placeholder={endpointPlaceholder}
          value={config.endpoint}
          onChange={(event) =>
            setConfig((prev) => ({ ...prev, endpoint: event.target.value }))
          }
        />
      </div>

      {config.provider === "s3" && (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="remote-bucket">{t("settings.remoteBackup.bucket")}</Label>
            <Input
              id="remote-bucket"
              placeholder="goaway-backups"
              value={config.bucket}
              onChange={(event) =>
                setConfig((prev) => ({ ...prev, bucket: event.target.value }))
              }
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="remote-region">{t("settings.remoteBackup.region")}</Label>
            <Input
              id="remote-region"
              placeholder="eu-west-1"
              value={config.region}
              onChange={(event) =>
                setConfig((prev) => ({ ...prev, region: event.target.value }))
              }
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="remote-access-key">{t("settings.remoteBackup.accessKey")}</Label>
            <Input
              id="remote-access-key"
              placeholder="AKIA..."
              value={config.accessKey}
              onChange={(event) =>
                setConfig((prev) => ({ ...prev, accessKey: event.target.value }))
              }
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="remote-secret-key">{t("settings.remoteBackup.secretKey")}</Label>
            <Input
              id="remote-secret-key"
              type="password"
              placeholder="********"
              value={config.secretKey}
              onChange={(event) =>
                setConfig((prev) => ({ ...prev, secretKey: event.target.value }))
              }
            />
          </div>
        </div>
      )}

      {config.provider === "webdav" && (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="remote-username">{t("settings.remoteBackup.username")}</Label>
            <Input
              id="remote-username"
              placeholder="webdav user"
              value={config.username}
              onChange={(event) =>
                setConfig((prev) => ({ ...prev, username: event.target.value }))
              }
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="remote-password">{t("settings.remoteBackup.password")}</Label>
            <Input
              id="remote-password"
              type="password"
              placeholder="********"
              value={config.password}
              onChange={(event) =>
                setConfig((prev) => ({ ...prev, password: event.target.value }))
              }
            />
          </div>
        </div>
      )}

      <div className="flex flex-col gap-2 pt-2 sm:flex-row">
        <Button onClick={saveConfig} disabled={isSaving} className="sm:min-w-40">
          {isSaving
            ? t("settings.remoteBackup.saving")
            : t("settings.remoteBackup.save")}
        </Button>
        <Button
          variant="outline"
          onClick={pushNow}
          disabled={isSyncing || !config.enabled}
          className="sm:min-w-40"
        >
          {isSyncing
            ? t("settings.remoteBackup.syncing")
            : t("settings.remoteBackup.pushNow")}
        </Button>
      </div>
    </div>
  );
}
