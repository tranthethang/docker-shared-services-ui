import { ApiError } from "./client.ts";
import type {
  ApiErrorBody,
  BatchDiff,
  OpsJob,
  OpsStatusSnapshot,
  ServiceAction,
  ServiceDTO,
} from "./types.ts";

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

async function requestJson<T>(
  url: string,
  init?: RequestInit,
): Promise<T> {
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
      // Preserve fallback HTTP status message when error body is not JSON.
    }
    throw new ApiError(response.status, code, message);
  }

  return (await response.json()) as T;
}

/** Triggers a single-service lifecycle action (`up`, `stop`, `down`, or `restart`). */
export function postServiceAction(
  name: string,
  action: ServiceAction,
): Promise<OpsJob> {
  return requestJson<OpsJob>(
    `/api/services/${encodeURIComponent(name)}/${encodeURIComponent(action)}`,
    { method: "POST" },
  );
}

/** Triggers a batch apply operation with explicit `up` and `down` service queues. */
export function postOpsApply(
  up: readonly string[],
  down: readonly string[],
): Promise<OpsJob> {
  return requestJson<OpsJob>("/api/ops/apply", {
    method: "POST",
    body: JSON.stringify({ up, down }),
  });
}

/** Fetches the current operations runner status snapshot. */
export function fetchOpsStatus(signal?: AbortSignal): Promise<OpsStatusSnapshot> {
  return requestJson<OpsStatusSnapshot>("/api/ops/status", { signal });
}

/** Computes the batch diff: `up` = checked but not running; `down` = running/partial but unchecked. */
export function computeBatchDiff(
  services: readonly ServiceDTO[],
  checked: ReadonlySet<string>,
): BatchDiff {
  const up: string[] = [];
  const down: string[] = [];

  for (const svc of services) {
    const isChecked = checked.has(svc.name);
    const isRunning = svc.state === "running";
    const isActive = svc.state === "running" || svc.state === "partial";

    if (isChecked && !isRunning) {
      up.push(svc.name);
    } else if (!isChecked && isActive) {
      down.push(svc.name);
    }
  }

  return { up, down };
}
