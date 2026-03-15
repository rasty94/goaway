import { GetRequest } from "@/util";
import {
  DatabaseIcon,
  Icon,
  ShieldIcon,
  TrashIcon,
  UsersIcon
} from "@phosphor-icons/react";
import clsx from "clsx";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Card } from "../../components/ui/card";

export type DNSMetrics = {
  allowed: number;
  blocked: number;
  cached: number;
  clients: number;
  domainBlockLen: number;
  percentageBlocked: number;
  total: number;
};

interface MetricsCardProps {
  title: string;
  valueKey: string;
  Icon: Icon;
  bgColor: string;
  type?: "number" | "percentage";
  metricsData: DNSMetrics | null;
  description?: string;
}

function MetricsCard({
  title,
  valueKey,
  Icon,
  bgColor,
  type = "number",
  metricsData,
  description = ""
}: MetricsCardProps) {
  const value = metricsData?.[valueKey as keyof DNSMetrics];

  const formattedValue =
    type === "percentage" && value !== undefined
      ? `${value.toFixed(1)}%`
      : value?.toLocaleString();

  return (
    <Card
      className={clsx(
        "border-none relative p-2 rounded-lg w-full overflow-hidden"
      )}
      style={{
        background: bgColor
      }}
    >
      <div className="relative z-10 flex items-center justify-between">
        <div>
          <p className="text-xs font-medium text-white">{title}</p>
          <p className="text-xl font-bold text-white">{formattedValue}</p>
          {description && (
            <p className="text-xs text-white/50 mt-0.5">{description}</p>
          )}
        </div>
        <Icon className="w-10 h-10 opacity-60" />
      </div>
    </Card>
  );
}

export default function MetricsCards() {
  const { t } = useTranslation();
  const [metricsData, setMetricsData] = useState<DNSMetrics | null>(null);

  useEffect(() => {
    async function fetchMetrics() {
      try {
        const [, data] = await GetRequest("dnsMetrics");
        setMetricsData(data);
      } catch (error) {
        console.error("Failed to fetch server statistics:", error);
      }
    }

    fetchMetrics();
    const interval = setInterval(fetchMetrics, 1000);

    return () => clearInterval(interval);
  }, []);

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      <MetricsCard
        title={t("home.metrics.total")}
        valueKey="total"
        Icon={ShieldIcon}
        bgColor="#166534"
        metricsData={metricsData}
      />
      <MetricsCard
        title={t("home.metrics.blocked")}
        valueKey="blocked"
        Icon={TrashIcon}
        bgColor="#991b1b"
        metricsData={metricsData}
      />
      <MetricsCard
        title={t("home.metrics.percentage")}
        valueKey="percentageBlocked"
        Icon={UsersIcon}
        bgColor="#1e40af"
        type="percentage"
        metricsData={metricsData}
      />
      <MetricsCard
        title={t("sidebar.blacklist")}
        valueKey="domainBlockLen"
        Icon={DatabaseIcon}
        bgColor="#6b21a8"
        metricsData={metricsData}
      />
    </div>
  );
}
