import type {
  ApiErrorBody,
  InfoResponse,
  PresetsResponse,
  ServiceDTO,
  ServicesResponse,
  Telemetry,
} from "./types.ts";

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
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

async function fetchJson<T>(url: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(url, {
    headers: { Accept: "application/json" },
    signal,
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

export function fetchServices(signal?: AbortSignal): Promise<ServicesResponse> {
  return fetchJson<ServicesResponse>("/api/services", signal);
}

export function fetchServiceDetail(name: string, signal?: AbortSignal): Promise<ServiceDTO> {
  return fetchJson<ServiceDTO>(`/api/services/${encodeURIComponent(name)}`, signal);
}

export function fetchTelemetry(signal?: AbortSignal): Promise<Telemetry> {
  return fetchJson<Telemetry>("/api/telemetry", signal);
}

export function fetchInfo(signal?: AbortSignal): Promise<InfoResponse> {
  return fetchJson<InfoResponse>("/api/info", signal);
}

export function fetchPresets(signal?: AbortSignal): Promise<PresetsResponse> {
  return fetchJson<PresetsResponse>("/api/presets", signal);
}
