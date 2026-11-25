import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle
} from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from "@/components/ui/table";
import { getApiBaseUrl, PostRequest } from "@/util";
import {
  CheckCircleIcon,
  UploadSimpleIcon,
  WarningCircleIcon
} from "@phosphor-icons/react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

type Existing = { exists: boolean };
type Plan = {
  adlists: (Existing & { name: string; url: string; enabled: boolean })[];
  blacklist: (Existing & { pattern: string })[];
  whitelist: (Existing & { pattern: string })[];
  groups: (Existing & { name: string; description: string })[];
  groupDomains: (Existing & {
    group: string;
    pattern: string;
    blocked: boolean;
  })[];
  clients: (Existing & { identifier: string; type: string; groups: string[] })[];
  resolutions: (Existing & { domain: string; value: string; type: string })[];
  upstreams: (Existing & { address: string })[];
  staticLeases: (Existing & { mac: string; ip: string; hostname: string })[];
  skipped: string[];
};
type Section = Exclude<keyof Plan, "skipped">;
type Report = {
  created: Record<string, number>;
  skipped: Record<string, number>;
  errors: string[];
};

// One display line per item; the backend decides what actually gets written.
const DESCRIBE: { [K in Section]: (item: Plan[K][number]) => string } = {
  adlists: (a) => `${a.name} — ${a.url}${a.enabled ? "" : " (disabled)"}`,
  blacklist: (d) => d.pattern,
  whitelist: (d) => d.pattern,
  groups: (g) => (g.description ? `${g.name} — ${g.description}` : g.name),
  groupDomains: (g) =>
    `${g.group}: ${g.blocked ? "block" : "allow"} ${g.pattern}`,
  clients: (c) => `${c.identifier} (${c.type}) → ${c.groups.join(", ")}`,
  resolutions: (r) => `${r.domain} ${r.type} ${r.value}`,
  upstreams: (u) => u.address,
  staticLeases: (l) =>
    `${l.mac} → ${l.ip}${l.hostname ? ` (${l.hostname})` : ""}`
};

// Order must match DESCRIBE's declaration order (used for stable section iteration).
const SECTIONS = Object.keys(DESCRIBE) as Section[];

export function PiholeImport() {
  const { t } = useTranslation();
  const [file, setFile] = useState<File | null>(null);
  const [plan, setPlan] = useState<Plan | null>(null);
  const [sections, setSections] = useState<Record<string, boolean>>({});
  const [report, setReport] = useState<Report | null>(null);
  const [busy, setBusy] = useState(false);

  const preview = async () => {
    if (!file) return;
    setBusy(true);
    setReport(null);
    try {
      const formData = new FormData();
      formData.append("backup", file);
      const res = await fetch(`${getApiBaseUrl()}/api/import/pihole/preview`, {
        method: "POST",
        body: formData,
        credentials: "include"
      });
      const data = await res.json();
      if (!res.ok) {
        toast.error(data.error || t("piholeImport.previewError"));
        return;
      }
      const parsed = data as Plan;
      for (const s of SECTIONS) (parsed as Record<Section, unknown[]>)[s] = parsed[s] ?? [];
      parsed.skipped = parsed.skipped ?? [];
      setPlan(parsed);
      const enabled: Record<string, boolean> = {};
      for (const s of SECTIONS) enabled[s] = parsed[s].length > 0;
      setSections(enabled);
    } catch {
      toast.error(t("piholeImport.previewError"));
    } finally {
      setBusy(false);
    }
  };

  const apply = async () => {
    if (!plan) return;
    setBusy(true);
    const [code, data] = await PostRequest("import/pihole/apply", {
      plan,
      sections
    });
    setBusy(false);
    if (code === 200) {
      setReport(data as Report);
      toast.success(t("piholeImport.applied"));
    }
  };

  const counts = useMemo(() => {
    const result = {} as Record<Section, { pending: number; conflicts: number }>;
    for (const s of SECTIONS) {
      let pendingCount = 0;
      let conflictCount = 0;
      for (const item of plan ? plan[s] : []) {
        if (item.exists) conflictCount++;
        else pendingCount++;
      }
      result[s] = { pending: pendingCount, conflicts: conflictCount };
    }
    return result;
  }, [plan]);
  const totalPending = SECTIONS.reduce(
    (n, s) => n + (sections[s] ? counts[s].pending : 0),
    0
  );

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">
          {t("piholeImport.title")}
        </h1>
        <p className="text-muted-foreground mt-1">
          {t("piholeImport.description")}
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <UploadSimpleIcon className="w-5 h-5" />
            {t("piholeImport.step1")}
          </CardTitle>
          <CardDescription>{t("piholeImport.step1Help")}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col sm:flex-row gap-3">
          <Input
            type="file"
            accept=".zip,application/zip"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
          <Button onClick={preview} disabled={!file || busy}>
            {busy && !plan ? t("piholeImport.parsing") : t("piholeImport.preview")}
          </Button>
        </CardContent>
      </Card>

      {plan && (
        <Card>
          <CardHeader>
            <CardTitle>{t("piholeImport.step2")}</CardTitle>
            <CardDescription>{t("piholeImport.step2Help")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {SECTIONS.filter((s) => plan[s].length > 0).map((s) => {
              const describeItem = DESCRIBE[s] as (
                item: Plan[Section][number]
              ) => string;
              return (
                <div key={s} className="space-y-2">
                  <label className="flex items-center gap-3 cursor-pointer">
                    <Checkbox
                      checked={!!sections[s]}
                      onCheckedChange={(v) =>
                        setSections((prev) => ({ ...prev, [s]: v === true }))
                      }
                    />
                    <span className="font-medium">
                      {t(`piholeImport.sections.${s}`)}
                    </span>
                    <span className="text-sm text-muted-foreground">
                      {t("piholeImport.counts", {
                        pending: counts[s].pending,
                        conflicts: counts[s].conflicts
                      })}
                    </span>
                  </label>
                  <div className="max-h-96 overflow-auto rounded-md border">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t("piholeImport.entry")}</TableHead>
                          <TableHead className="w-32">
                            {t("piholeImport.status")}
                          </TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {plan[s].map((item, i) => (
                          <TableRow
                            key={i}
                            className={item.exists ? "opacity-60" : ""}
                          >
                            <TableCell className="font-mono text-xs break-all">
                              {describeItem(item)}
                            </TableCell>
                            <TableCell>
                              {item.exists ? (
                                <span className="inline-flex items-center gap-1 text-amber-600">
                                  <WarningCircleIcon className="w-4 h-4" />
                                  {t("piholeImport.exists")}
                                </span>
                              ) : (
                                <span className="inline-flex items-center gap-1 text-emerald-600">
                                  <CheckCircleIcon className="w-4 h-4" />
                                  {t("piholeImport.new")}
                                </span>
                              )}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                </div>
              );
            })}

            {plan.skipped.length > 0 && (
              <div className="rounded-md border border-amber-500/40 bg-amber-500/5 p-4 text-sm space-y-1">
                <p className="font-medium">
                  {t("piholeImport.skippedTitle", { count: plan.skipped.length })}
                </p>
                <ul className="list-disc pl-5 text-muted-foreground">
                  {plan.skipped.map((line, i) => (
                    <li key={i}>{line}</li>
                  ))}
                </ul>
              </div>
            )}

            <Button onClick={apply} disabled={busy || totalPending === 0}>
              {t("piholeImport.apply", { count: totalPending })}
            </Button>
          </CardContent>
        </Card>
      )}

      {report && (
        <Card>
          <CardHeader>
            <CardTitle>{t("piholeImport.step3")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("piholeImport.section")}</TableHead>
                  <TableHead>{t("piholeImport.created")}</TableHead>
                  <TableHead>{t("piholeImport.skipped")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {SECTIONS.filter(
                  (s) => report.created[s] || report.skipped[s]
                ).map((s) => (
                  <TableRow key={s}>
                    <TableCell>{t(`piholeImport.sections.${s}`)}</TableCell>
                    <TableCell>{report.created[s] ?? 0}</TableCell>
                    <TableCell>{report.skipped[s] ?? 0}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {report.errors.length > 0 && (
              <div className="rounded-md border border-destructive/40 bg-destructive/5 p-4 text-sm space-y-1">
                <p className="font-medium">
                  {t("piholeImport.errorsTitle", { count: report.errors.length })}
                </p>
                <ul className="list-disc pl-5 font-mono text-xs">
                  {report.errors.map((line, i) => (
                    <li key={i}>{line}</li>
                  ))}
                </ul>
              </div>
            )}
            <p className="text-sm text-muted-foreground">
              {t("piholeImport.restartHint")}
            </p>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
