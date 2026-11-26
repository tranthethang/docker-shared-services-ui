import assert from "node:assert/strict";
import test from "node:test";
import type { ServiceDTO } from "../api/types.ts";
import { formatBytes, formatPercent, t } from "../i18n/messages.ts";
import {
  appendCpuSample,
  CPU_HISTORY_CAPACITY,
  filterServices,
  groupServicesByCategory,
} from "./useDashboardData.ts";

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

void test("appendCpuSample bounds ring buffer to 16 samples and sanitizes invalid values", () => {
  let history: number[] = [];
  for (let i = 1; i <= 20; i += 1) {
    history = appendCpuSample(history, i);
  }
  assert.equal(history.length, CPU_HISTORY_CAPACITY);
  assert.equal(history[0], 5);
  assert.equal(history[history.length - 1], 20);

  const sanitized = appendCpuSample([10], Number.NaN);
  assert.deepEqual(sanitized, [10, 0]);
});

void test("filterServices filters by query, category, and activeOnly", () => {
  const services: ServiceDTO[] = [
    makeService({ name: "postgres", category: "Database", state: "running" }),
    makeService({
      name: "redis",
      category: "Cache & Queue",
      description: "In-memory cache",
      state: "stopped",
    }),
    makeService({
      name: "traefik",
      category: "Auth & Gateway",
      description: "Reverse proxy and TLS gateway",
      state: "partial",
    }),
  ];

  assert.equal(filterServices(services, "", "", false).length, 3);
  assert.equal(filterServices(services, "cache", "", false).length, 1);
  assert.equal(filterServices(services, "TRAEFIK", "", false)[0]?.name, "traefik");
  assert.equal(filterServices(services, "", "Database", false).length, 1);
  assert.deepEqual(
    filterServices(services, "", "", true).map((s) => s.name),
    ["postgres", "traefik"],
  );
});

void test("groupServicesByCategory preserves catalog order and omits empty groups", () => {
  const services: ServiceDTO[] = [
    makeService({ name: "traefik", category: "Auth & Gateway", state: "running" }),
    makeService({ name: "postgres", category: "Database", state: "running" }),
    makeService({ name: "mysql", category: "Database", state: "stopped" }),
  ];
  const categories = ["Database", "Cache & Queue", "Auth & Gateway"];

  const groups = groupServicesByCategory(services, categories);
  assert.equal(groups.length, 2);
  assert.equal(groups[0]?.category, "Database");
  assert.equal(groups[0]?.runningCount, 1);
  assert.equal(groups[0]?.totalCount, 2);
  assert.equal(groups[1]?.category, "Auth & Gateway");
  assert.equal(groups[1]?.runningCount, 1);
});

void test("i18n helpers format bytes, percentages, and interpolated messages", () => {
  assert.equal(formatBytes(0), "0 B");
  assert.equal(formatBytes(1024 * 1024 * 512), "512 MiB");
  assert.equal(formatPercent(12.34), "12.3%");
  assert.equal(
    t("servicesRunningSummary", { running: 3, total: 10 }),
    "3 of 10 running",
  );
});
