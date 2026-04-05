import { GetRequest } from "@/util";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { 
  CheckCircle, 
  Stack, 
  Desktop, 
  WarningCircle,
  ArrowsClockwise
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export type ClusterNode = {
  id: string;
  address: string;
  role: "primary" | "follower";
  status: string;
  priority: number;
  lastSeen: string;
  latencyMs: number;
  unreachable: boolean;
};

export type ClusterStatus = {
  selfRole: string;
  activeNodes: number;
  clusterId: string;
  nodes: ClusterNode[];
  proxyStats?: {
    totalRequests: number;
    nodeRequests: Record<string, number>;
    errorRequests: number;
  };
};

export function Cluster() {
  const [status, setStatus] = useState<ClusterStatus | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchStatus = async () => {
    setLoading(true);
    const [code, response] = await GetRequest("ha/cluster");
    if (code !== 200) {
      toast.error("Failed to fetch cluster status");
      setLoading(false);
      return;
    }
    setStatus(response);
    setLoading(false);
  };

  useEffect(() => {
    fetchStatus();
    const interval = setInterval(fetchStatus, 10000); // Auto refresh every 10s
    return () => clearInterval(interval);
  }, []);

  if (loading && !status) {
    return (
      <div className="flex items-center justify-center h-64">
        <ArrowsClockwise className="w-8 h-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Cluster Management</h1>
          <p className="text-muted-foreground">Monitor and manage High Availability nodes.</p>
        </div>
        <Button onClick={fetchStatus} disabled={loading}>
          <ArrowsClockwise className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground uppercase">Current Role</CardTitle>
            <span className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase border ${status?.selfRole === 'primary' ? 'bg-primary/20 text-primary border-primary/30' : 'bg-secondary/20 text-secondary border-secondary/30'}`}>
              {status?.selfRole || 'Unknown'}
            </span>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{status?.selfRole === 'primary' ? 'Leader' : 'Follower'}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground uppercase">Active Nodes</CardTitle>
            <Stack className="h-4 w-4 text-emerald-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{status?.activeNodes || 0}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground uppercase">Cluster ID</CardTitle>
            <Desktop className="h-4 w-4 text-blue-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold truncate">{status?.clusterId || 'N/A'}</div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Cluster Nodes</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-left">
              <thead>
                <tr className="border-b text-sm font-medium text-muted-foreground">
                  <th className="pb-3 px-4">Node ID</th>
                  <th className="pb-3 px-4">Address</th>
                  <th className="pb-3 px-4">Role</th>
                  <th className="pb-3 px-4">Status</th>
                  <th className="pb-3 px-4">Priority</th>
                  <th className="pb-3 px-4">Latency</th>
                  <th className="pb-3 px-4 text-right">Last Seen</th>
                </tr>
              </thead>
              <tbody className="divide-y text-sm">
                {(status?.nodes || []).map((node) => (
                  <tr key={node.id} className="hover:bg-muted/50 transition-colors">
                    <td className="py-4 px-4 font-mono">{node.id}</td>
                    <td className="py-4 px-4 text-muted-foreground">{node.address}</td>
                    <td className="py-4 px-4">
                      <span className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase border ${node.role === 'primary' ? 'bg-primary/20 text-primary border-primary/30' : 'bg-muted text-muted-foreground border-border'}`}>
                        {node.role}
                      </span>
                    </td>
                    <td className="py-4 px-4">
                      {node.unreachable ? (
                        <div className="flex items-center gap-2 text-red-500">
                          <WarningCircle className="w-4 h-4" />
                          Offline
                        </div>
                      ) : (
                        <div className="flex items-center gap-2 text-emerald-500">
                          <CheckCircle className="w-4 h-4" />
                          Online
                        </div>
                      )}
                    </td>
                    <td className="py-4 px-4 text-muted-foreground">{node.priority}</td>
                    <td className="py-4 px-4 text-muted-foreground">{node.latencyMs}ms</td>
                    <td className="py-4 px-4 text-right text-muted-foreground">
                      {new Date(node.lastSeen).toLocaleTimeString()}
                    </td>
                  </tr>
                ))}
                {(!status?.nodes || status.nodes.length === 0) && (
                  <tr>
                    <td colSpan={7} className="py-8 text-center text-muted-foreground italic">
                      No discovered peers for this cluster.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      {status?.proxyStats && (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          <Card className="bg-emerald-500/5 border-emerald-500/20">
            <CardHeader className="pb-2">
              <CardTitle className="text-xs font-medium text-emerald-500 uppercase">Total Proxied</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{status.proxyStats.totalRequests || 0}</div>
              <p className="text-[10px] text-muted-foreground mt-1">Queries balanced across cluster</p>
            </CardContent>
          </Card>
          <Card className={`${status.proxyStats.errorRequests > 0 ? 'bg-amber-500/5 border-amber-500/20' : 'bg-muted/50 border-border'}`}>
            <CardHeader className="pb-2">
              <CardTitle className="text-xs font-medium uppercase font-bold">Node Failovers</CardTitle>
            </CardHeader>
            <CardContent>
              <div className={`text-2xl font-bold ${status.proxyStats.errorRequests > 0 ? 'text-amber-500' : ''}`}>
                {status.proxyStats.errorRequests || 0}
              </div>
              <p className="text-[10px] text-muted-foreground mt-1">Retries on healthy nodes</p>
            </CardContent>
          </Card>
          
          <Card className="col-span-2">
            <CardHeader className="pb-2">
              <CardTitle className="text-xs font-medium uppercase">Node Traffic Distribution</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-3 mt-2">
                {Object.entries(status.proxyStats.nodeRequests || {}).map(([ip, count]) => {
                   const percentage = Math.round((count / (status?.proxyStats?.totalRequests || 1)) * 100);
                   return (
                    <div key={ip} className="space-y-1">
                      <div className="flex justify-between text-[11px] font-mono">
                        <span>{ip}</span>
                        <span className="font-bold">{count} q ({percentage}%)</span>
                      </div>
                      <div className="h-1.5 w-full bg-muted rounded-full overflow-hidden">
                        <div 
                          className="h-full bg-primary transition-all duration-500" 
                          style={{ width: `${percentage}%` }}
                        />
                      </div>
                    </div>
                  );
                })}
                {Object.keys(status.proxyStats.nodeRequests || {}).length === 0 && (
                  <div className="text-center py-4 text-xs text-muted-foreground italic">
                    No proxy traffic recorded yet.
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      <div className="bg-blue-500/5 border border-blue-500/20 rounded-lg p-4">
        <h3 className="font-semibold text-blue-500 mb-2">Clustering Note</h3>
        <p className="text-sm text-muted-foreground">
          GoAway HA Active uses priority-based leader election. The node with the highest priority becomes the primary
          and replicates all blacklist, whitelist, and group changes to followers in real-time.
        </p>
      </div>
    </div>
  );
}
