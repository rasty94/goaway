export type ClientEntry = {
  ip: string;
  lastSeen: string;
  name: string;
  mac: string;
  vendor: string;
  bypass: boolean;
  // Written by react-force-graph while simulating node positions.
  x?: number;
  y?: number;
};
