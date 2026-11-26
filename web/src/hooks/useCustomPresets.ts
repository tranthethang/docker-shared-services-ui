import { useCallback, useState } from "react";
import type { Preset } from "../api/types.ts";

export const CUSTOM_PRESETS_STORAGE_KEY = "dss-ui.custom-presets.v1";

export interface CustomPreset extends Preset {
  id: string;
  updatedAt: string;
}

export interface CustomPresetInput {
  name: string;
  summary: string;
  services: readonly string[];
}

function normalizeServices(services: readonly string[]): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const item of services) {
    if (typeof item !== "string") {
      continue;
    }
    const trimmed = item.trim();
    if (trimmed.length > 0 && !seen.has(trimmed)) {
      seen.add(trimmed);
      result.push(trimmed);
    }
  }
  return result;
}

/** Parses and validates persisted custom presets from JSON. */
export function parseCustomPresets(raw: string | null): CustomPreset[] {
  if (!raw) {
    return [];
  }
  try {
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      return [];
    }
    const valid: CustomPreset[] = [];
    for (const entry of parsed) {
      if (typeof entry !== "object" || entry === null) {
        continue;
      }
      const record = entry as Record<string, unknown>;
      if (
        typeof record.id !== "string" ||
        record.id.trim().length === 0 ||
        typeof record.name !== "string" ||
        record.name.trim().length === 0 ||
        !Array.isArray(record.services)
      ) {
        continue;
      }
      const services = normalizeServices(
        record.services.filter((s): s is string => typeof s === "string"),
      );
      if (services.length === 0) {
        continue;
      }
      valid.push({
        id: record.id.trim(),
        name: record.name.trim(),
        summary: typeof record.summary === "string" ? record.summary.trim() : "",
        services,
        updatedAt:
          typeof record.updatedAt === "string" && record.updatedAt.length > 0
            ? record.updatedAt
            : new Date(0).toISOString(),
      });
    }
    return valid;
  } catch {
    return [];
  }
}

function readStoredPresets(): CustomPreset[] {
  if (typeof window === "undefined" || !window.localStorage) {
    return [];
  }
  try {
    return parseCustomPresets(window.localStorage.getItem(CUSTOM_PRESETS_STORAGE_KEY));
  } catch {
    return [];
  }
}

function writeStoredPresets(presets: readonly CustomPreset[]): void {
  if (typeof window === "undefined" || !window.localStorage) {
    return;
  }
  try {
    window.localStorage.setItem(CUSTOM_PRESETS_STORAGE_KEY, JSON.stringify(presets));
  } catch {
    // Ignore storage quota or private browsing write failures.
  }
}

function makePresetId(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `preset-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

/** Manages user-defined custom presets persisted in localStorage. */
export function useCustomPresets() {
  const [customPresets, setCustomPresets] = useState<CustomPreset[]>(() => readStoredPresets());

  const createPreset = useCallback((input: CustomPresetInput): CustomPreset | null => {
    const name = input.name.trim();
    const summary = input.summary.trim();
    const services = normalizeServices(input.services);
    if (name.length === 0 || services.length === 0) {
      return null;
    }
    const created: CustomPreset = {
      id: makePresetId(),
      name,
      summary,
      services,
      updatedAt: new Date().toISOString(),
    };
    setCustomPresets((prev) => {
      const next = [created, ...prev];
      writeStoredPresets(next);
      return next;
    });
    return created;
  }, []);

  const updatePreset = useCallback((id: string, input: CustomPresetInput): boolean => {
    const name = input.name.trim();
    const summary = input.summary.trim();
    const services = normalizeServices(input.services);
    if (name.length === 0 || services.length === 0) {
      return false;
    }
    let updated = false;
    setCustomPresets((prev) => {
      const next = prev.map((item) => {
        if (item.id !== id) {
          return item;
        }
        updated = true;
        return {
          ...item,
          name,
          summary,
          services,
          updatedAt: new Date().toISOString(),
        };
      });
      if (updated) {
        writeStoredPresets(next);
      }
      return next;
    });
    return updated;
  }, []);

  const deletePreset = useCallback((id: string): void => {
    setCustomPresets((prev) => {
      const next = prev.filter((item) => item.id !== id);
      writeStoredPresets(next);
      return next;
    });
  }, []);

  return {
    customPresets,
    createPreset,
    updatePreset,
    deletePreset,
  };
}
