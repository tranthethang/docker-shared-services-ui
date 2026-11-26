import { ApiError } from "./client.ts";
import type { ApiErrorBody, OpsJob } from "./types.ts";

export interface EnvFileStatus {
  service: string;
  hasEnv: boolean;
  hasExample: boolean;
  missing: boolean;
}

export interface EnvProblem {
  service: string;
  code: string;
  key?: string;
  message: string;
  remediation?: string;
}

export interface NetworkStatus {
  name: string;
  subnet: string;
  exists: boolean;
}

export interface EnvStatusResponse {
  stackName: string;
  files: EnvFileStatus[];
  problems: EnvProblem[];
  networks: NetworkStatus[];
  availableServices: string[];
  defaultExportServices: string[];
}

export interface MergeEnvResponse {
  added: number;
  files: EnvFileStatus[];
}

export interface ExportPreviewResponse {
  checked: string[];
  added: string[];
  resolved: string[];
}

export interface ExportResult {
  name: string;
  outputPath: string;
  services: string[];
  added: string[];
}

function isApiErrorBody(value: unknown): value is ApiErrorBody {
  return (
    typeof value === "object" &&
    value !== null &&
    "code" in value &&
    "error" in value &&
    typeof (value as { code: unknown }).code === "string" &&
    typeof (value as { error: unknown }).error === "string"
  );
}

async function adminRequest<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    ...init,
    headers: {
      Accept: "application/json",
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...init?.headers,
    },
  });

  if (!response.ok) {
    let code = "http_error";
    let message = `HTTP ${response.status}`;
    try {
      const body: unknown = await response.json();
      if (isApiErrorBody(body)) {
        code = body.code;
        message = body.error;
      }
    } catch {
      // Keep HTTP status fallback when response is not JSON.
    }
    throw new ApiError(response.status, code, message);
  }

  return (await response.json()) as T;
}

/** Fetches current .env file status, prerequisite warnings, and shared network state. */
export function fetchEnvStatus(signal?: AbortSignal): Promise<EnvStatusResponse> {
  return adminRequest<EnvStatusResponse>("/api/env/status", { signal });
}

/** Triggers the single-flight Setup operation. */
export function postSetup(password?: string, force = false): Promise<OpsJob> {
  return adminRequest<OpsJob>("/api/setup", {
    method: "POST",
    body: JSON.stringify({ password: password ?? "", force }),
  });
}

/** Merges missing keys from .env.example into existing .env files. */
export function postEnvMerge(): Promise<MergeEnvResponse> {
  return adminRequest<MergeEnvResponse>("/api/env/merge", {
    method: "POST",
  });
}

/** Triggers Compose config validation across active services. */
export function postValidate(): Promise<OpsJob> {
  return adminRequest<OpsJob>("/api/validate", {
    method: "POST",
  });
}

/** Previews auto-included backend dependencies for an export selection. */
export function postExportPreview(
  services: readonly string[],
): Promise<ExportPreviewResponse> {
  return adminRequest<ExportPreviewResponse>("/api/export/preview", {
    method: "POST",
    body: JSON.stringify({ services }),
  });
}

/** Exports selected services and resolved dependencies to dist/<name>/. */
export function postExportPackage(
  name: string,
  services: readonly string[],
): Promise<ExportResult> {
  return adminRequest<ExportResult>("/api/export", {
    method: "POST",
    body: JSON.stringify({ name, services }),
  });
}

/** Triggers Docker resource prune after stack name confirmation. */
export function postMaintenancePrune(confirmStackName: string): Promise<OpsJob> {
  return adminRequest<OpsJob>("/api/maintenance/prune", {
    method: "POST",
    body: JSON.stringify({ confirmStackName }),
  });
}

/** Triggers reverse-order stack teardown and volume removal after stack name confirmation. */
export function postMaintenanceTeardown(confirmStackName: string): Promise<OpsJob> {
  return adminRequest<OpsJob>("/api/maintenance/teardown", {
    method: "POST",
    body: JSON.stringify({ confirmStackName }),
  });
}
