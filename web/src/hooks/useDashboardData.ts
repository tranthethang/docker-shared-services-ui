import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, fetchInfo, fetchServices, fetchTelemetry } from "../api/client.ts";
import type { InfoResponse, ServiceDTO, Telemetry } from "../api/types.ts";

export const CPU_HISTORY_CAPACITY = 16;
const REFRESH_DEBOUNCE_MS = 250;

export type ConnectionStatus = "connecting" | "connected" | "disconnected";

export interface DashboardError {
  code: string;
  message: string;
}

export interface ServiceCategoryGroup {
  category: string;
  services: ServiceDTO[];
  runningCount: number;
  totalCount: number;
}

export interface DashboardDataState {
  services: ServiceDTO[];
  categories: string[];
  telemetry: Telemetry | null;
  info: InfoResponse | null;
  cpuHistory: number[];
  loading: boolean;
  refreshing: boolean;
  error: DashboardError | null;
  connectionStatus: ConnectionStatus;
  lastUpdatedAt: number | null;
  refresh: () => Promise<void>;
}

/** Appends a CPU percentage sample into a bounded ring buffer of at most maxLen points. */
export function appendCpuSample(
  history: readonly number[],
  cpuPct: number,
  maxLen: number = CPU_HISTORY_CAPACITY,
): number[] {
  const safeValue = Number.isFinite(cpuPct) && cpuPct >= 0 ? cpuPct : 0;
  const next = [...history, safeValue];
  if (next.length <= maxLen) {
    return next;
  }
  return next.slice(next.length - maxLen);
}

/** Filters services by search query (name/description), category, and active-only state. */
export function filterServices(
  services: readonly ServiceDTO[],
  query: string,
  category: string,
  activeOnly: boolean,
): ServiceDTO[] {
  const needle = query.trim().toLowerCase();
  return services.filter((svc) => {
    if (category && svc.category !== category) {
      return false;
    }
    if (activeOnly && svc.state !== "running" && svc.state !== "partial") {
      return false;
    }
    if (!needle) {
      return true;
    }
    return (
      svc.name.toLowerCase().includes(needle) ||
      svc.description.toLowerCase().includes(needle)
    );
  });
}

/** Groups services according to the ordered catalog categories, omitting empty groups. */
export function groupServicesByCategory(
  services: readonly ServiceDTO[],
  categories: readonly string[],
): ServiceCategoryGroup[] {
  const byCategory = new Map<string, ServiceDTO[]>();
  for (const svc of services) {
    const list = byCategory.get(svc.category);
    if (list) {
      list.push(svc);
    } else {
      byCategory.set(svc.category, [svc]);
    }
  }

  const groups: ServiceCategoryGroup[] = [];
  const seen = new Set<string>();

  for (const category of categories) {
    const items = byCategory.get(category);
    if (!items || items.length === 0) {
      continue;
    }
    seen.add(category);
    groups.push({
      category,
      services: items,
      runningCount: items.filter((s) => s.state === "running" || s.state === "partial").length,
      totalCount: items.length,
    });
  }

  for (const [category, items] of byCategory.entries()) {
    if (seen.has(category) || items.length === 0) {
      continue;
    }
    groups.push({
      category,
      services: items,
      runningCount: items.filter((s) => s.state === "running" || s.state === "partial").length,
      totalCount: items.length,
    });
  }

  return groups;
}

function toDashboardError(err: unknown): DashboardError {
  if (err instanceof ApiError) {
    return { code: err.code, message: err.message };
  }
  if (err instanceof Error) {
    return { code: "network_error", message: err.message };
  }
  return { code: "unknown_error", message: "Unexpected error" };
}

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

export function useDashboardData(): DashboardDataState {
  const [services, setServices] = useState<ServiceDTO[]>([]);
  const [categories, setCategories] = useState<string[]>([]);
  const [telemetry, setTelemetry] = useState<Telemetry | null>(null);
  const [info, setInfo] = useState<InfoResponse | null>(null);
  const [cpuHistory, setCpuHistory] = useState<number[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [refreshing, setRefreshing] = useState<boolean>(false);
  const [error, setError] = useState<DashboardError | null>(null);
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>("connecting");
  const [lastUpdatedAt, setLastUpdatedAt] = useState<number | null>(null);

  const activeRequestRef = useRef<AbortController | null>(null);
  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const loadSnapshot = useCallback(async (isInitial: boolean) => {
    activeRequestRef.current?.abort();
    const controller = new AbortController();
    activeRequestRef.current = controller;

    if (isInitial) {
      setLoading(true);
    } else {
      setRefreshing(true);
    }

    try {
      const [infoResult, servicesResult, telemetryResult] = await Promise.allSettled([
        fetchInfo(controller.signal),
        fetchServices(controller.signal),
        fetchTelemetry(controller.signal),
      ]);

      if (controller.signal.aborted) {
        return;
      }

      if (infoResult.status === "fulfilled") {
        setInfo(infoResult.value);
      }

      if (servicesResult.status === "fulfilled") {
        setServices(servicesResult.value.services);
        setCategories(servicesResult.value.categories);
      }

      if (telemetryResult.status === "fulfilled") {
        const nextTelemetry = telemetryResult.value;
        setTelemetry(nextTelemetry);
        setCpuHistory((prev) => appendCpuSample(prev, nextTelemetry.totalCpuPct));
      }

      if (servicesResult.status === "rejected" && !isAbortError(servicesResult.reason)) {
        setError(toDashboardError(servicesResult.reason));
      } else if (telemetryResult.status === "rejected" && !isAbortError(telemetryResult.reason)) {
        setError(toDashboardError(telemetryResult.reason));
      } else if (infoResult.status === "rejected" && !isAbortError(infoResult.reason)) {
        setError(toDashboardError(infoResult.reason));
      } else {
        setError(null);
        setLastUpdatedAt(Date.now());
      }
    } finally {
      if (!controller.signal.aborted) {
        setLoading(false);
        setRefreshing(false);
      }
    }
  }, []);

  const refresh = useCallback(async () => {
    await loadSnapshot(false);
  }, [loadSnapshot]);

  useEffect(() => {
    void loadSnapshot(true);

    if (typeof EventSource === "undefined") {
      setConnectionStatus("disconnected");
      return () => {
        activeRequestRef.current?.abort();
      };
    }

    const source = new EventSource("/api/events");

    const handleOpen = () => {
      setConnectionStatus("connected");
    };

    const handleTelemetry = (event: MessageEvent<string>) => {
      try {
        const parsed = JSON.parse(event.data) as Telemetry;
        setTelemetry(parsed);
        setCpuHistory((prev) => appendCpuSample(prev, parsed.totalCpuPct));
        setLastUpdatedAt(Date.now());
        setError((prev) => (prev?.code === "docker_unavailable" ? null : prev));
      } catch {
        // Ignore malformed SSE telemetry frames.
      }
    };

    const handleRefresh = () => {
      if (debounceTimerRef.current !== null) {
        clearTimeout(debounceTimerRef.current);
      }
      debounceTimerRef.current = setTimeout(() => {
        debounceTimerRef.current = null;
        void loadSnapshot(false);
      }, REFRESH_DEBOUNCE_MS);
    };

    const handleError = () => {
      setConnectionStatus("disconnected");
    };

    source.addEventListener("open", handleOpen);
    source.addEventListener("telemetry", handleTelemetry);
    source.addEventListener("refresh", handleRefresh);
    source.addEventListener("error", handleError);

    return () => {
      if (debounceTimerRef.current !== null) {
        clearTimeout(debounceTimerRef.current);
        debounceTimerRef.current = null;
      }
      source.removeEventListener("open", handleOpen);
      source.removeEventListener("telemetry", handleTelemetry);
      source.removeEventListener("refresh", handleRefresh);
      source.removeEventListener("error", handleError);
      source.close();
      activeRequestRef.current?.abort();
    };
  }, [loadSnapshot]);

  return {
    services,
    categories,
    telemetry,
    info,
    cpuHistory,
    loading,
    refreshing,
    error,
    connectionStatus,
    lastUpdatedAt,
    refresh,
  };
}
