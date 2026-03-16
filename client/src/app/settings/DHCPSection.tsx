import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { GetRequest, PostRequest, PutRequest, getApiBaseUrl } from "@/util";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from "@/components/ui/table";

import type { Dhcp } from "./types";

type DHCPStatus = {
  enabled: boolean;
  running: boolean;
  ipv4Enabled: boolean;
  ipv6Enabled: boolean;
  leaseCount: number;
};

type ActiveDHCPLease = {
  id: number;
  mac: string;
  ip: string;
  hostname: string;
  expiresAt: string;
};


type StaticLease = {
  id: number;
  mac: string;
  ip: string;
  hostname: string;
  enabled: boolean;
};

type LeaseForm = {
  mac: string;
  ip: string;
  hostname: string;
  enabled: boolean;
};

const emptyLease: LeaseForm = {
  mac: "",
  ip: "",
  hostname: "",
  enabled: true
};

type Props = {
  dhcp: Dhcp;
  onDhcpChange: (update: Partial<Dhcp>) => void;
  onSaveConfig: () => Promise<void>;
};

export function DHCPSection({ dhcp, onDhcpChange, onSaveConfig }: Props) {
  const { t } = useTranslation();
  const [status, setStatus] = useState<DHCPStatus | null>(null);
  const [leases, setLeases] = useState<StaticLease[]>([]);
  const [activeLeases, setActiveLeases] = useState<ActiveDHCPLease[]>([]);
  const [loading, setLoading] = useState(true);

  const [busyAction, setBusyAction] = useState<"start" | "stop" | "save" | null>(null);
  const [leaseForm, setLeaseForm] = useState<LeaseForm>(emptyLease);
  const [editingLeaseId, setEditingLeaseId] = useState<number | null>(null);
  const [savingLease, setSavingLease] = useState(false);

  const refresh = async () => {
    setLoading(true);
    const [[statusCode, statusResponse], [leasesCode, leasesResponse], [activeLeasesCode, activeLeasesResponse]] = await Promise.all([
      GetRequest("dhcp/status", true),
      GetRequest("dhcp/leases", true),
      GetRequest("dhcp/activeLeases", true)
    ]);

    if (statusCode === 200 && statusResponse) {
      setStatus(statusResponse as DHCPStatus);
    }
    if (leasesCode === 200 && Array.isArray(leasesResponse)) {
      setLeases(leasesResponse as StaticLease[]);
    }
    if (activeLeasesCode === 200 && Array.isArray(activeLeasesResponse)) {
      setActiveLeases(activeLeasesResponse as ActiveDHCPLease[]);
    }

    setLoading(false);
  };

  useEffect(() => {
    refresh();
  }, []);

  const saveDhcpConfig = async () => {
    setBusyAction("save");
    try {
      await onSaveConfig();
      toast.success(t("settings.dhcp.toasts.settingsSaved"));
    } finally {
      setBusyAction(null);
    }
  };

  const startDhcp = async () => {
    setBusyAction("start");
    const [statusCode, response] = await PostRequest("dhcp/start", {}, false, true);
    setBusyAction(null);
    if (statusCode === 200) {
      toast.success(t("settings.dhcp.toasts.started"));
      refresh();
      return;
    }
    toast.error(response?.error || t("settings.dhcp.toasts.startFailed"));
  };

  const stopDhcp = async () => {
    setBusyAction("stop");
    const [statusCode, response] = await PostRequest("dhcp/stop", {}, false, true);
    setBusyAction(null);
    if (statusCode === 200) {
      toast.success(t("settings.dhcp.toasts.stopped"));
      refresh();
      return;
    }
    toast.error(response?.error || t("settings.dhcp.toasts.stopFailed"));
  };

  const saveLease = async () => {
    setSavingLease(true);
    const payload = {
      mac: leaseForm.mac.trim(),
      ip: leaseForm.ip.trim(),
      hostname: leaseForm.hostname.trim(),
      enabled: leaseForm.enabled
    };

    const result = editingLeaseId
      ? await PutRequest(`dhcp/leases/${editingLeaseId}`, payload, true)
      : await PostRequest("dhcp/leases", payload, false, true);

    setSavingLease(false);

    if (result[0] === 200 || result[0] === 201) {
      toast.success(
        editingLeaseId
          ? t("settings.dhcp.toasts.leaseUpdated")
          : t("settings.dhcp.toasts.leaseCreated")
      );
      setLeaseForm(emptyLease);
      setEditingLeaseId(null);
      refresh();
      return;
    }

    const errorMessage =
      typeof result[1] === "object" && result[1] !== null && "error" in result[1]
        ? String((result[1] as { error: unknown }).error)
        : t("settings.dhcp.toasts.leaseFailed");
    toast.error(errorMessage);
  };

  const deleteLease = async (leaseId: number) => {
    const response = await fetch(`${getApiBaseUrl()}/api/dhcp/leases/${leaseId}`, {
      method: "DELETE",
      credentials: "include"
    });

    if (response.ok) {
      toast.success(t("settings.dhcp.toasts.leaseDeleted"));
      if (editingLeaseId === leaseId) {
        setEditingLeaseId(null);
        setLeaseForm(emptyLease);
      }
      refresh();
      return;
    }

    try {
      const body = await response.json();
      toast.error(body.error || t("settings.dhcp.toasts.leaseDeleteFailed"));
    } catch {
      toast.error(t("settings.dhcp.toasts.leaseDeleteFailed"));
    }
  };

  const dnsServersValue = dhcp.dnsServers.join(", ");

  return (
    <div className="space-y-6">
      <div className="rounded-lg border border-border/60 bg-muted/20 p-4">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <p className="text-sm font-medium">{t("settings.dhcp.status.title")}</p>
            <p className="text-sm text-muted-foreground">
              {loading
                ? t("settings.dhcp.status.loading")
                : status?.running
                ? t("settings.dhcp.status.running")
                : t("settings.dhcp.status.stopped")}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" onClick={refresh} disabled={loading}>
              {t("settings.dhcp.actions.refresh")}
            </Button>
            <Button variant="outline" onClick={saveDhcpConfig} disabled={busyAction !== null}>
              {busyAction === "save"
                ? t("settings.dhcp.actions.saving")
                : t("settings.dhcp.actions.saveConfig")}
            </Button>
            {status?.running ? (
              <Button variant="destructive" onClick={stopDhcp} disabled={busyAction !== null}>
                {busyAction === "stop"
                  ? t("settings.dhcp.actions.stopping")
                  : t("settings.dhcp.actions.stop")}
              </Button>
            ) : (
              <Button onClick={startDhcp} disabled={busyAction !== null}>
                {busyAction === "start"
                  ? t("settings.dhcp.actions.starting")
                  : t("settings.dhcp.actions.start")}
              </Button>
            )}
          </div>
        </div>
        <p className="mt-3 text-xs text-muted-foreground">
          {t("settings.dhcp.status.hint")}
        </p>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="dhcp-enabled">{t("settings.dhcp.fields.enabled")}</Label>
          <div className="flex h-10 items-center rounded-md border px-3">
            <Switch
              id="dhcp-enabled"
              checked={dhcp.enabled}
              onCheckedChange={(enabled) => onDhcpChange({ enabled })}
            />
          </div>
        </div>

        <div className="space-y-2">
          <Label htmlFor="dhcp-address">{t("settings.dhcp.fields.address")}</Label>
          <Input
            id="dhcp-address"
            value={dhcp.address}
            onChange={(event) => onDhcpChange({ address: event.target.value })}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="dhcp-interface">{t("settings.dhcp.fields.interface")}</Label>
          <Input
            id="dhcp-interface"
            value={dhcp.interface}
            placeholder="eth0"
            onChange={(event) => onDhcpChange({ interface: event.target.value })}
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2">
            <Label htmlFor="dhcp-ipv4-enabled">{t("settings.dhcp.fields.ipv4Enabled")}</Label>
            <div className="flex h-10 items-center rounded-md border px-3">
              <Switch
                id="dhcp-ipv4-enabled"
                checked={dhcp.ipv4Enabled}
                onCheckedChange={(ipv4Enabled) => onDhcpChange({ ipv4Enabled })}
              />
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="dhcp-ipv6-enabled">{t("settings.dhcp.fields.ipv6Enabled")}</Label>
            <div className="flex h-10 items-center rounded-md border px-3">
              <Switch
                id="dhcp-ipv6-enabled"
                checked={dhcp.ipv6Enabled}
                onCheckedChange={(ipv6Enabled) => onDhcpChange({ ipv6Enabled })}
              />
            </div>
          </div>
        </div>

        <div className="space-y-2">
          <Label htmlFor="dhcp-range-start">{t("settings.dhcp.fields.rangeStart")}</Label>
          <Input
            id="dhcp-range-start"
            value={dhcp.rangeStart}
            onChange={(event) => onDhcpChange({ rangeStart: event.target.value })}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="dhcp-range-end">{t("settings.dhcp.fields.rangeEnd")}</Label>
          <Input
            id="dhcp-range-end"
            value={dhcp.rangeEnd}
            onChange={(event) => onDhcpChange({ rangeEnd: event.target.value })}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="dhcp-router">{t("settings.dhcp.fields.router")}</Label>
          <Input
            id="dhcp-router"
            value={dhcp.router}
            onChange={(event) => onDhcpChange({ router: event.target.value })}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="dhcp-dns-servers">{t("settings.dhcp.fields.dnsServers")}</Label>
          <Input
            id="dhcp-dns-servers"
            value={dnsServersValue}
            onChange={(event) =>
              onDhcpChange({
                dnsServers: event.target.value
                  .split(",")
                  .map((entry) => entry.trim())
                  .filter(Boolean)
              })
            }
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="dhcp-domain-search">{t("settings.dhcp.fields.domainSearch")}</Label>
          <Input
            id="dhcp-domain-search"
            value={dhcp.domainSearch}
            onChange={(event) => onDhcpChange({ domainSearch: event.target.value })}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="dhcp-lease-duration">{t("settings.dhcp.fields.leaseDuration")}</Label>
          <Input
            id="dhcp-lease-duration"
            type="number"
            value={dhcp.leaseDuration}
            onChange={(event) => onDhcpChange({ leaseDuration: Number(event.target.value) || 0 })}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="dhcp-port-v4">{t("settings.dhcp.fields.portIPv4")}</Label>
          <Input
            id="dhcp-port-v4"
            type="number"
            value={dhcp.ports.ipv4}
            onChange={(event) =>
              onDhcpChange({ ports: { ...dhcp.ports, ipv4: Number(event.target.value) || 0 } })
            }
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="dhcp-port-v6">{t("settings.dhcp.fields.portIPv6")}</Label>
          <Input
            id="dhcp-port-v6"
            type="number"
            value={dhcp.ports.ipv6}
            onChange={(event) =>
              onDhcpChange({ ports: { ...dhcp.ports, ipv6: Number(event.target.value) || 0 } })
            }
          />
        </div>
      </div>

      <div id="static-lease-form" className="space-y-4 rounded-lg border border-border/60 p-4">
        <div>
          <h3 className="text-base font-semibold">{t("settings.dhcp.leases.title")}</h3>
          <p className="text-sm text-muted-foreground">
            {t("settings.dhcp.leases.description")}
          </p>
        </div>

        <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
          <div className="space-y-2">
            <Label htmlFor="lease-mac">{t("settings.dhcp.leases.mac")}</Label>
            <Input
              id="lease-mac"
              placeholder="aa:bb:cc:dd:ee:ff"
              value={leaseForm.mac}
              onChange={(event) =>
                setLeaseForm((prev) => ({ ...prev, mac: event.target.value }))
              }
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="lease-ip">{t("settings.dhcp.leases.ip")}</Label>
            <Input
              id="lease-ip"
              placeholder="192.168.0.10"
              value={leaseForm.ip}
              onChange={(event) =>
                setLeaseForm((prev) => ({ ...prev, ip: event.target.value }))
              }
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="lease-hostname">{t("settings.dhcp.leases.hostname")}</Label>
            <Input
              id="lease-hostname"
              placeholder="office-printer"
              value={leaseForm.hostname}
              onChange={(event) =>
                setLeaseForm((prev) => ({ ...prev, hostname: event.target.value }))
              }
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="lease-enabled">{t("settings.dhcp.leases.enabled")}</Label>
            <div className="flex h-10 items-center rounded-md border px-3">
              <Switch
                id="lease-enabled"
                checked={leaseForm.enabled}
                onCheckedChange={(enabled) =>
                  setLeaseForm((prev) => ({ ...prev, enabled }))
                }
              />
            </div>
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          <Button onClick={saveLease} disabled={savingLease}>
            {savingLease
              ? t("settings.dhcp.leases.saving")
              : editingLeaseId
              ? t("settings.dhcp.leases.update")
              : t("settings.dhcp.leases.create")}
          </Button>
          {editingLeaseId !== null && (
            <Button
              variant="outline"
              onClick={() => {
                setEditingLeaseId(null);
                setLeaseForm(emptyLease);
              }}
            >
              {t("settings.dhcp.leases.cancelEdit")}
            </Button>
          )}
        </div>

        <div className="overflow-x-auto rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("settings.dhcp.leases.mac")}</TableHead>
                <TableHead>{t("settings.dhcp.leases.ip")}</TableHead>
                <TableHead>{t("settings.dhcp.leases.hostname")}</TableHead>
                <TableHead>{t("settings.dhcp.leases.state")}</TableHead>
                <TableHead className="text-right">{t("settings.dhcp.leases.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {leases.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-muted-foreground">
                    {t("settings.dhcp.leases.empty")}
                  </TableCell>
                </TableRow>
              ) : (
                leases.map((lease) => (
                  <TableRow key={lease.id}>
                    <TableCell className="font-mono text-xs md:text-sm">{lease.mac}</TableCell>
                    <TableCell className="font-mono text-xs md:text-sm">{lease.ip}</TableCell>
                    <TableCell>{lease.hostname || "-"}</TableCell>
                    <TableCell>
                      {lease.enabled
                        ? t("settings.dhcp.leases.active")
                        : t("settings.dhcp.leases.disabled")}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setEditingLeaseId(lease.id);
                            setLeaseForm({
                              mac: lease.mac,
                              ip: lease.ip,
                              hostname: lease.hostname,
                              enabled: lease.enabled
                            });
                          }}
                        >
                          {t("settings.dhcp.leases.edit")}
                        </Button>
                        <Button
                          variant="destructive"
                          size="sm"
                          onClick={() => deleteLease(lease.id)}
                        >
                          {t("settings.dhcp.leases.delete")}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </div>

      <div className="space-y-4 rounded-lg border border-border/60 p-4">
        <div>
          <h3 className="text-base font-semibold">{t("settings.dhcp.activeLeases.title")}</h3>
          <p className="text-sm text-muted-foreground">
            {t("settings.dhcp.activeLeases.description")}
          </p>
        </div>

        <div className="overflow-x-auto rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("settings.dhcp.activeLeases.mac")}</TableHead>
                <TableHead>{t("settings.dhcp.activeLeases.ip")}</TableHead>
                <TableHead>{t("settings.dhcp.activeLeases.hostname")}</TableHead>
                <TableHead>{t("settings.dhcp.activeLeases.expires")}</TableHead>
                <TableHead className="text-right">{t("settings.dhcp.leases.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {activeLeases.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-muted-foreground">
                    {t("settings.dhcp.activeLeases.empty")}
                  </TableCell>
                </TableRow>
              ) : (
                activeLeases.map((lease) => (
                  <TableRow key={lease.id}>
                    <TableCell className="font-mono text-xs md:text-sm">{lease.mac}</TableCell>
                    <TableCell className="font-mono text-xs md:text-sm">{lease.ip}</TableCell>
                    <TableCell>{lease.hostname || "-"}</TableCell>
                    <TableCell className="text-xs">
                      {new Date(lease.expiresAt).toLocaleString()}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => {
                          setLeaseForm({
                            mac: lease.mac,
                            ip: lease.ip,
                            hostname: lease.hostname || "",
                            enabled: true
                          });
                          document.getElementById('static-lease-form')?.scrollIntoView({ behavior: 'smooth' });
                        }}
                      >
                        {t("settings.dhcp.activeLeases.convertToStatic")}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </div>
    </div>

  );
}
