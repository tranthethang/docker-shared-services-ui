import { useCallback, useEffect, useRef, useState } from "react";
import type { ConnectionStatus } from "./useDashboardData.ts";
import { appendBoundedLine, appendBoundedLines } from "./useOpsStream.ts";

export const LOG_MAX_LINES = 2000;

export interface UseLogStreamOptions {
  serviceName: string | null;
  container?: string;
  tail?: number;
  enabled?: boolean;
}

export interface LogStreamState {
  lines: string[];
  paused: boolean;
  bufferedCount: number;
  error: string | null;
  connectionStatus: ConnectionStatus;
  togglePause: () => void;
  clear: () => void;
}

function parseLogPayload(raw: string): string {
  try {
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed === "string") {
      return parsed;
    }
  } catch {
    // Fall back to raw text when payload is not JSON-encoded.
  }
  return raw;
}

/** Subscribes to `/api/services/{name}/logs` SSE stream with pause/resume buffering and 2000-line cap. */
export function useLogStream({
  serviceName,
  container = "",
  tail = 200,
  enabled = true,
}: UseLogStreamOptions): LogStreamState {
  const [lines, setLines] = useState<string[]>([]);
  const [paused, setPaused] = useState(false);
  const [bufferedCount, setBufferedCount] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>("disconnected");

  const pausedRef = useRef(false);
  pausedRef.current = paused;

  const bufferRef = useRef<string[]>([]);

  const clear = useCallback(() => {
    bufferRef.current = [];
    setBufferedCount(0);
    setPaused(false);
    setLines([]);
  }, []);

  const togglePause = useCallback(() => {
    setPaused((prev) => {
      const next = !prev;
      if (!next && bufferRef.current.length > 0) {
        const pending = bufferRef.current;
        bufferRef.current = [];
        setBufferedCount(0);
        setLines((curr) => appendBoundedLines(curr, pending, LOG_MAX_LINES));
      }
      return next;
    });
  }, []);

  useEffect(() => {
    if (!enabled || !serviceName || typeof EventSource === "undefined") {
      setConnectionStatus("disconnected");
      return;
    }

    bufferRef.current = [];
    setBufferedCount(0);
    setPaused(false);
    setLines([]);
    setError(null);
    setConnectionStatus("connecting");

    const params = new URLSearchParams();
    params.set("tail", String(tail));
    if (container) {
      params.set("container", container);
    }

    const url = `/api/services/${encodeURIComponent(serviceName)}/logs?${params.toString()}`;
    const source = new EventSource(url);

    source.onopen = () => {
      setConnectionStatus("connected");
      setError(null);
    };

    source.onerror = () => {
      setConnectionStatus("disconnected");
    };

    const handleLog = (event: MessageEvent<string>) => {
      const line = parseLogPayload(event.data);
      if (pausedRef.current) {
        bufferRef.current = appendBoundedLine(bufferRef.current, line, LOG_MAX_LINES);
        setBufferedCount(bufferRef.current.length);
        return;
      }
      setLines((prev) => appendBoundedLine(prev, line, LOG_MAX_LINES));
    };

    const handleStreamError = (event: MessageEvent<string>) => {
      try {
        const parsed = JSON.parse(event.data) as { error?: string };
        if (typeof parsed.error === "string" && parsed.error.length > 0) {
          setError(parsed.error);
        }
      } catch {
        setError("Log stream error");
      }
    };

    source.addEventListener("log", handleLog);
    source.addEventListener("error", handleStreamError);

    return () => {
      source.removeEventListener("log", handleLog);
      source.removeEventListener("error", handleStreamError);
      source.close();
    };
  }, [serviceName, container, tail, enabled]);

  return {
    lines,
    paused,
    bufferedCount,
    error,
    connectionStatus,
    togglePause,
    clear,
  };
}
