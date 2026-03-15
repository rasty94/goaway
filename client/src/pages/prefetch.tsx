import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from "@/components/ui/table";
import { DeleteRequest, GetRequest, PostRequest } from "@/util";
import {
  ClockIcon,
  DatabaseIcon,
  GlobeIcon,
  PlusIcon,
  SpinnerIcon,
  TrashIcon
} from "@phosphor-icons/react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { validateFQDN } from "./validation";
import { NoContent } from "@/shared";
import { useTranslation } from "react-i18next";

type PrefetchEntry = {
  domain: string;
  refresh: number;
  queryType: number;
};

function queryTypeExpanded(queryType: number) {
  switch (queryType) {
    case 1:
      return "A";
    case 28:
      return "AAAA";
    case 5:
      return "CNAME";
    case 12:
      return "PTR";
  }
}

async function CreatePrefetch(
  domain: string,
  refresh: number,
  queryType: number,
  t: (key: string, options?: any) => string
) {
  const [code, response] = await PostRequest("prefetch", {
    domain,
    refresh,
    queryType
  });
  if (code === 200) {
    toast.success(t("prefetch.toasts.added", { domain }));
    return true;
  } else {
    toast.error(response.error);
    return false;
  }
}

async function DeletePrefetch(
  domain: string,
  t: (key: string, options?: any) => string
) {
  const [code, response] = await DeleteRequest(
    `prefetch?domain=${domain}`,
    null
  );
  if (code === 200) {
    toast.success(t("prefetch.toasts.removed", { domain }));
    return true;
  } else {
    toast.error(response.error);
    return false;
  }
}

export function Prefetch() {
  const { t } = useTranslation();
  const [prefetches, setPrefetches] = useState<PrefetchEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [domainName, setDomainName] = useState("");
  const [refresh, setrefresh] = useState(0);
  const [queryType, setQueryType] = useState("1");
  const [searchTerm, setSearchTerm] = useState("");
  const [domainError, setDomainError] = useState<string>("");

  const fetchPrefetches = async () => {
    setLoading(true);
    const [code, response] = await GetRequest("prefetch");
    if (code !== 200) {
      toast.error(t("prefetch.toasts.fetchError"));
      setLoading(false);
      return;
    }

    setPrefetches(response || []);
    setLoading(false);
  };

  useEffect(() => {
    const id = setTimeout(() => {
      void fetchPrefetches();
    }, 0);
    return () => clearTimeout(id);
  }, []);

  useEffect(() => {
    if (domainName) {
      const validation = validateFQDN(domainName);
      setTimeout(() => {
        setDomainError(validation.error || "");
      }, 0);
    } else {
      setTimeout(() => {
        setDomainError("");
      }, 0);
    }
  }, [domainName]);

  const handleSave = async () => {
    const validation = validateFQDN(domainName);

    if (!validation.isValid) {
      toast.error(validation.error);
      setDomainError(validation.error || "");
      return;
    }

    setSubmitting(true);
    const success = await CreatePrefetch(
      domainName,
      refresh,
      parseInt(queryType),
      t
    );
    if (success) {
      await fetchPrefetches();
      setDomainName("");
      setDomainError("");
    }
    setSubmitting(false);
  };

  const handleDelete = async (domain: string) => {
    const success = await DeletePrefetch(domain, t);
    if (success) {
      await fetchPrefetches();
    }
  };

  const formatRefresh = (seconds: number) => {
    if (seconds === 0) return t("prefetch.refresh.onTTL");
    if (seconds < 60) return t("prefetch.refresh.seconds", { count: seconds });
    if (seconds < 3600)
      return t("prefetch.refresh.minutes", { count: Math.floor(seconds / 60) });
    if (seconds < 86400)
      return t("prefetch.refresh.hours", { count: Math.floor(seconds / 3600) });
    return t("prefetch.refresh.days", { count: Math.floor(seconds / 86400) });
  };

  const filteredPrefetches = searchTerm
    ? prefetches.filter((prefetch) =>
        prefetch.domain.toLowerCase().includes(searchTerm.toLowerCase())
      )
    : prefetches;

  const isFormValid = domainName && !domainError;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:justify-between sm:items-center">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            {t("prefetch.title")}
          </h1>
          <p className="text-muted-foreground mt-1">
            {t("prefetch.description")}
          </p>
        </div>
        <div className="flex items-center gap-2 text-sm">
          <DatabaseIcon className="h-3 w-3" />
          {prefetches.length} {prefetches.length === 1 ? t("prefetch.entry") : t("prefetch.entries")}
        </div>
      </div>

      <Card className="shadow-md">
        <CardHeader className="pb-2">
          <CardTitle className="flex items-center gap-2">
            <PlusIcon className="h-5 w-5 text-green-500" />
            {t("prefetch.addTitle")}
          </CardTitle>
          <CardDescription>
            {t("prefetch.addDescription")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div className="grid gap-4 md:grid-cols-3">
              <div className="space-y-2">
                <Label htmlFor="domain" className="font-medium">
                  {t("prefetch.domainLabel")}
                </Label>
                <div className="relative">
                  <GlobeIcon className="absolute left-3 top-3 h-4 w-4 text-gray-400" />
                  <Input
                    id="domain"
                    placeholder={t("prefetch.domainPlaceholder")}
                    className={`pl-9 ${
                      domainError ? "border-destructive" : ""
                    }`}
                    value={domainName}
                    onChange={(e) => setDomainName(e.target.value)}
                  />
                </div>
                {domainError && (
                  <span className="text-xs text-red-500 font-medium">
                    {domainError}
                    <br />
                  </span>
                )}
                <span className="text-xs text-muted-foreground">
                  {t("prefetch.domainHint")}
                </span>
                <span className="text-xs text-muted-foreground font-bold">
                  <br />
                  {t("prefetch.note")}:{" "}
                </span>
                <span className="text-xs text-muted-foreground">
                  {t("prefetch.fqdnHint")}
                </span>
              </div>
              <div className="space-y-2">
                <Label htmlFor="refresh" className="font-medium">
                  {t("prefetch.refreshLabel")}
                </Label>
                <Select
                  value={refresh.toString()}
                  onValueChange={(value) => setrefresh(parseInt(value))}
                >
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder={t("prefetch.refreshSelect")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="0">{t("prefetch.refresh.onTTL")}</SelectItem>
                  </SelectContent>
                </Select>
                <div>
                  <span className="text-xs text-muted-foreground">
                    {t("prefetch.refreshHint")}
                    <br />
                    {t("prefetch.refreshHintTTL")}
                  </span>
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="queryType" className="font-medium">
                  {t("prefetch.queryType")}
                </Label>
                <Select
                  value={queryType}
                  onValueChange={(value) => setQueryType(value)}
                >
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder={t("prefetch.queryTypeSelect")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="1">A (IPv4 address)</SelectItem>
                    <SelectItem value="28">AAAA (IPv6 address)</SelectItem>
                    <SelectItem value="5">CNAME (Canonical name)</SelectItem>
                    <SelectItem value="12">PTR (Pointer record)</SelectItem>
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">
                  {t("prefetch.queryTypeHint")}
                </p>
              </div>
            </div>
          </div>
        </CardContent>
        <div className="flex justify-end p-4 pt-0 sm:pt-4">
          <Button
            variant="default"
            onClick={handleSave}
            disabled={submitting || !isFormValid}
            className="w-full sm:w-auto"
          >
            {submitting ? (
              <>
                <SpinnerIcon className="h-4 w-4 mr-2 animate-spin" />
                {t("prefetch.adding")}
              </>
            ) : (
              t("prefetch.add")
            )}
          </Button>
        </div>
      </Card>

      <Card className="shadow-md">
        <CardHeader className="pb-4 border-b">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <CardTitle className="flex items-center gap-2">
              <ClockIcon className="h-5 w-5 text-blue-500" />
              {t("prefetch.activeTitle")}
            </CardTitle>
            <div className="w-full lg:w-auto mt-1 lg:mt-0">
              <Input
                placeholder={t("prefetch.searchPlaceholder")}
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="text-sm"
              />
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-4">
          {loading ? (
            <div className="p-6 space-y-4">
              {[1, 2, 3].map((i) => (
                <div key={i} className="flex items-center justify-between">
                  <div className="space-y-2">
                    <Skeleton className="h-4 w-48" />
                    <Skeleton className="h-4 w-24" />
                  </div>
                  <Skeleton className="h-8 w-8 rounded-full" />
                </div>
              ))}
            </div>
          ) : filteredPrefetches.length > 0 ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("prefetch.columns.domain")}</TableHead>
                  <TableHead>{t("prefetch.columns.refresh")}</TableHead>
                  <TableHead>{t("prefetch.columns.queryType")}</TableHead>
                  <TableHead className="text-right">{t("prefetch.columns.action")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredPrefetches.map((prefetch) => (
                  <TableRow
                    key={prefetch.domain}
                    className="hover:bg-accent text-sm"
                  >
                    <TableCell className="font-medium max-w-[220px] truncate sm:max-w-none sm:whitespace-normal">
                      {prefetch.domain}
                    </TableCell>
                    <TableCell className="font-mono">
                      {formatRefresh(prefetch.refresh)}
                    </TableCell>
                    <TableCell className="">
                      {queryTypeExpanded(prefetch.queryType)}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-8 w-8 p-0 text-red-500 hover:text-red-700 hover:font-bold cursor-pointer"
                          onClick={() => handleDelete(prefetch.domain)}
                        >
                          <TrashIcon className="h-4 w-4" />
                          <span className="sr-only">{t("prefetch.delete")}</span>
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            <div className="flex flex-col items-center justify-center text-center text-muted-foreground">
              {searchTerm ? (
                t("prefetch.noMatch")
              ) : (
                <NoContent text={t("prefetch.emptyState")} />
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
