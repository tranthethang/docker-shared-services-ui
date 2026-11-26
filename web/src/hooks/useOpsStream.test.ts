import assert from "node:assert/strict";
import test from "node:test";
import { computeBatchDiff } from "../api/ops.ts";
import type { ServiceDTO } from "../api/types.ts";
import { LOG_MAX_LINES } from "./useLogStream.ts";
import {
  appendBoundedLine,
  appendBoundedLines,
  OPS_MAX_LINES,
} from "./useOpsStream.ts";

function makeService(overrides: Partial<ServiceDTO> = {}): ServiceDTO {
  return {
    name: "postgres",
    category: "Database",
    description: "Relational SQL database",
    expectedContainers: ["dss-postgres"],
    webUi: [],
    info: [],
    state: "running",
    health: "healthy",
    containerCount: 1,
    runningCount: 1,
    containers: [],
    ...overrides,
  };
}

void test("appendBoundedLine truncates ops buffer at 200 lines keeping newest entries", () => {
  let lines: string[] = [];
  for (let i = 1; i <= 250; i += 1) {
    lines = appendBoundedLine(lines, `line-${i}`, OPS_MAX_LINES);
  }
  assert.equal(lines.length, OPS_MAX_LINES);
  assert.equal(lines[0], "line-51");
  assert.equal(lines[lines.length - 1], "line-250");
});

void test("appendBoundedLines truncates log buffer at 2000 lines when flushing", () => {
  const initial = Array.from({ length: 1950 }, (_, idx) => `old-${idx + 1}`);
  const incoming = Array.from({ length: 100 }, (_, idx) => `new-${idx + 1}`);
  const merged = appendBoundedLines(initial, incoming, LOG_MAX_LINES);

  assert.equal(merged.length, LOG_MAX_LINES);
  assert.equal(merged[0], "old-51");
  assert.equal(merged[merged.length - 1], "new-100");
});

void test("computeBatchDiff computes up for checked non-running and down for unchecked active", () => {
  const services: ServiceDTO[] = [
    makeService({ name: "postgres", state: "running" }),
    makeService({ name: "redis", state: "stopped" }),
    makeService({ name: "traefik", state: "partial" }),
    makeService({ name: "minio", state: "absent" }),
  ];

  // Keep postgres checked, uncheck traefik (partial), check redis (stopped) and minio (absent).
  const checked = new Set(["postgres", "redis", "minio"]);
  const diff = computeBatchDiff(services, checked);

  assert.deepEqual(diff.up, ["redis", "minio"]);
  assert.deepEqual(diff.down, ["traefik"]);
});

void test("computeBatchDiff marks partial service for up when checked", () => {
  const services: ServiceDTO[] = [
    makeService({ name: "traefik", state: "partial" }),
    makeService({ name: "redis", state: "stopped" }),
  ];

  const checked = new Set(["traefik"]);
  const diff = computeBatchDiff(services, checked);

  assert.deepEqual(diff.up, ["traefik"]);
  assert.deepEqual(diff.down, []);
});
