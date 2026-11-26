import { useCallback, useEffect, useRef, useState } from "react";
import type { OpsEvent, OpsJob, OpsStatusSnapshot } from "../api/types.ts";
import type { ConnectionStatus } from "./useDashboardData.ts";

export const OPS_MAX_LINES = 200;

/** Appends one line to a bounded ring buffer, keeping at most `maxLines` trailing entries. */
export function appendBoundedLine(
  lines: readonly string[],
  incoming: string,
  maxLines: number = OPS_MAX_LINES,
): string[] {
  if (maxLines <= 0) {
    return [];
  }
  const next = [...lines, incoming];
  if (next.length <= maxLines) {
    return next;
  }
  return next.slice(next.length - maxLines);
}

/** Appends multiple lines to a bounded ring buffer, keeping at most `maxLines` trailing entries. */
export function appendBoundedLines(
  lines: readonly string[],
  incoming: readonly string[],
  maxLines: number = OPS_MAX_LINES,
): string[] {
  if (maxLines <= 0) {
    return [];
  }
  if (incoming.length === 0) {
    return lines.length <= maxLines ? [...lines] : lines.slice(lines.length - maxLines);
  }
  const next = [...lines, ...incoming];
  if (next.length <= maxLines) {
    return next;
  }
  return next.slice(next.length - maxLines);
}

export interface UseOpsStreamOptions {
  onJobDone?: (job: OpsJob) => void;
}

export interface OpsStreamState {
  busy: boolean;
  currentJob: OpsJob | null;
  lastJob: OpsJob | null;
  lines: string[];
  connectionStatus: ConnectionStatus;
  clearLines: () => void;
  markJobStarted: (job: OpsJob) => void;
}

/** Subscribes to `/api/ops/stream` for real-time operation progress and lock status. */
export function useOpsStream(options: UseOpsStreamOptions = {}): OpsStreamState {
  const [busy, setBusy] = useState(false);
  const [currentJob, setCurrentJob] = useState<OpsJob | null>(null);
  const [lastJob, setLastJob] = useState<OpsJob | null>(null);
  const [lines, setLines] = useState<string[]>([]);
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>("connecting");

  const onJobDoneRef = useRef(options.onJobDone);
  onJobDoneRef.current = options.onJobDone;

  const clearLines = useCallback(() => {
    setLines([]);
  }, []);

  const markJobStarted = useCallback((job: OpsJob) => {
    setBusy(true);
    setCurrentJob(job);
  }, []);

  useEffect(() => {
    if (typeof EventSource === "undefined") {
      setConnectionStatus("disconnected");
      return;
    }

    setConnectionStatus("connecting");
    const source = new EventSource("/api/ops/stream");

    source.onopen = () => {
      setConnectionStatus("connected");
    };

    source.onerror = () => {
      setConnectionStatus("disconnected");
    };

    const handleStatus = (event: MessageEvent<string>) => {
      try {
        const snap = JSON.parse(event.data) as OpsStatusSnapshot;
        setBusy(Boolean(snap.busy));
        setCurrentJob(snap.current ?? null);
        if (Array.isArray(snap.lines)) {
          setLines(appendBoundedLines([], snap.lines, OPS_MAX_LINES));
        }
        if (Array.isArray(snap.history) && snap.history.length > 0) {
          setLastJob(snap.history[0] ?? null);
        }
      } catch {
        // Ignore malformed status event payloads.
      }
    };

    const handleLine = (event: MessageEvent<string>) => {
      try {
        const payload = JSON.parse(event.data) as OpsEvent;
        if (typeof payload.line === "string") {
          setLines((prev) => appendBoundedLine(prev, payload.line as string, OPS_MAX_LINES));
        }
      } catch {
        // Ignore malformed line event payloads.
      }
    };

    const handleDone = (event: MessageEvent<string>) => {
      try {
        const payload = JSON.parse(event.data) as OpsEvent;
        setBusy(false);
        setCurrentJob(null);
        if (payload.job) {
          setLastJob(payload.job);
          onJobDoneRef.current?.(payload.job);
        }
      } catch {
        // Ignore malformed done event payloads.
      }
    };

    source.addEventListener("status", handleStatus);
    source.addEventListener("line", handleLine);
    source.addEventListener("done", handleDone);

    return () => {
      source.removeEventListener("status", handleStatus);
      source.removeEventListener("line", handleLine);
      source.removeEventListener("done", handleDone);
      source.close();
    };
  }, []);

  return {
    busy,
    currentJob,
    lastJob,
    lines,
    connectionStatus,
    clearLines,
    markJobStarted,
  };
}
