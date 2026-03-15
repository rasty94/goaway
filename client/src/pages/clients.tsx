import DNSServerVisualizer from "@/app/clients/map";

export type ClientEntry = {
  ip: string;
  lastSeen: string;
  name: string;
  mac: string;
  vendor: string;
  bypass: boolean;
  x?: number;
  y?: number;
};

export function Clients() {
  return (
    <div className="flex items-center justify-center min-h-[calc(100vh-280px)] sm:min-h-[calc(100vh-220px)]">
      <DNSServerVisualizer />
    </div>
  );
}
