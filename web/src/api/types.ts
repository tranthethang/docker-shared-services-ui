export type ServiceState = "running" | "partial" | "stopped" | "absent";

export type ServiceHealth = "unhealthy" | "starting" | "healthy" | "none" | "-";

export interface ContainerInfo {
  id: string;
  name: string;
  status: string;
  health: string;
  image: string;
  composeService: string;
  completed: boolean;
  project: string;
  restartPolicy: string;
}

export interface ServiceDTO {
  name: string;
  category: string;
  description: string;
  expectedContainers: string[];
  webUi: string[];
  info: string[];
  state: string;
  health: string;
  containerCount: number;
  runningCount: number;
  containers: ContainerInfo[];
  locked?: boolean;
  lockedReason?: string;
}

export type ServiceAction = "up" | "stop" | "down" | "restart";

export type OpsJobStatus = "running" | "succeeded" | "failed";

export interface OpsJob {
  id: string;
  kind: string;
  service?: string;
  action: string;
  up?: string[];
  down?: string[];
  status: OpsJobStatus;
  message?: string;
  startedAt: string;
  finishedAt?: string;
}

export interface OpsStatusSnapshot {
  busy: boolean;
  current?: OpsJob;
  lines: string[];
  history: OpsJob[];
}

export interface OpsEvent {
  type: "status" | "line" | "done";
  jobId: string;
  line?: string;
  job?: OpsJob;
}

export interface BatchDiff {
  up: string[];
  down: string[];
}

export interface ServicesResponse {
  services: ServiceDTO[];
  categories: string[];
}

export interface Preset {
  name: string;
  summary: string;
  services: string[];
}

export interface PresetsResponse {
  presets: Preset[];
}

export interface NetworkPair {
  name: string;
  subnet: string;
  pair: string;
}

export interface InfoResponse {
  domain: string;
  stackName: string;
  prefix: string;
  subnetBase: number;
  networks: NetworkPair[];
  serviceCount: number;
}

export interface ContainerStat {
  id: string;
  name: string;
  cpuPct: number;
  memUsedBytes: number;
  memLimitBytes: number;
  memPct: number;
}

export interface Capacity {
  ncpu: number;
  memTotalBytes: number;
}

export interface DiskUsage {
  imagesBytes: number;
  containersBytes: number;
  volumesBytes: number;
  buildCacheBytes: number;
  totalBytes: number;
  reclaimableBytes: number;
}

export interface Telemetry {
  capacity: Capacity;
  diskUsage: DiskUsage;
  containers: Record<string, ContainerStat>;
  totalCpuPct: number;
  totalMemBytes: number;
}

export interface ApiErrorBody {
  code: string;
  error: string;
}
