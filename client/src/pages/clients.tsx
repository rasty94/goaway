import DNSServerVisualizer from "@/app/clients/map";

export function Clients() {
  return (
    <div className="flex items-center justify-center min-h-[calc(100vh-280px)] sm:min-h-[calc(100vh-220px)]">
      <DNSServerVisualizer />
    </div>
  );
}
