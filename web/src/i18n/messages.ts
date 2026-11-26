const messages = {
  en: {
    appTitle: "Docker Shared Services",
    stackLabel: "Stack",
    domainLabel: "Domain",
    servicesRunningSummary: "{running} of {total} running",
    connectionConnected: "Live",
    connectionConnecting: "Connecting",
    connectionDisconnected: "Offline",
    refreshAction: "Refresh",
    refreshingAction: "Refreshing",
    telemetryTitle: "System telemetry",
    telemetryUnavailable: "Telemetry unavailable",
    telemetryLoading: "Loading telemetry",
    cpuLabel: "CPU usage",
    cpuCoresSuffix: "{ncpu} cores",
    cpuHistoryLabel: "CPU history",
    cpuSampleTooltip: "Sample {index}: {value}",
    memoryLabel: "Memory",
    memoryOfLimit: "{used} of {total}",
    diskLabel: "Docker disk",
    diskTotal: "{total} total",
    diskImages: "Images",
    diskContainers: "Containers",
    diskVolumes: "Volumes",
    diskReclaimable: "Reclaimable",
    searchLabel: "Search services",
    searchPlaceholder: "Filter by name or description",
    categoryLabel: "Category",
    categoryAll: "All categories",
    activeOnlyLabel: "Active only",
    clearFiltersAction: "Clear filters",
    noMatchingServices: "No services match the current filters.",
    noServicesInStack: "No services configured in this stack.",
    groupRunningCount: "{running}/{total} active",
    collapseGroup: "Collapse {category}",
    expandGroup: "Expand {category}",
    stateRunning: "Running",
    statePartial: "Partial",
    stateStopped: "Stopped",
    stateAbsent: "Not created",
    healthHealthy: "Healthy",
    healthStarting: "Starting",
    healthUnhealthy: "Unhealthy",
    healthNone: "No check",
    healthDash: "Inactive",
    containersCount: "{running}/{total} containers",
    openWebUi: "Open UI",
    inspectService: "Inspect {name}",
    drawerClose: "Close",
    drawerCategoryLabel: "Category",
    drawerStateLabel: "State",
    drawerHealthLabel: "Health",
    drawerWebUiLabel: "Web interfaces",
    drawerInfoLabel: "Notes",
    drawerContainersLabel: "Containers",
    drawerNoContainers: "No containers found for this service.",
    drawerExpectedContainersLabel: "Expected containers",
    tableName: "Container",
    tableComposeService: "Compose service",
    tableImage: "Image",
    tableStatus: "Status",
    tableHealth: "Health",
    tableCpu: "CPU",
    tableMemory: "Memory",
    badgeOneShotCompleted: "Completed",
    errorBannerTitle: "Unable to load live Docker data",
    errorDockerUnavailable: "Docker daemon is unreachable. Start Docker and retry.",
    errorRetryAction: "Retry",
    loadingDashboard: "Loading services and telemetry",
  },
} as const;

export type Locale = keyof typeof messages;
export type MessageKey = keyof (typeof messages)[Locale];

/** Translates a message key with optional `{token}` interpolation. */
export function t(
  key: MessageKey,
  params?: Record<string, string | number>,
  locale: Locale = "en",
): string {
  const template: string = messages[locale][key];
  if (!params) {
    return template;
  }
  return template.replace(/\{(\w+)\}/g, (_, token: string) => {
    const value = params[token];
    return value !== undefined ? String(value) : `{${token}}`;
  });
}

/** Formats a byte count into a human-readable IEC unit (B, KiB, MiB, GiB, TiB). */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return "0 B";
  }
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  const exponent = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** exponent;
  const decimals = exponent === 0 ? 0 : value >= 100 ? 0 : 1;
  return `${value.toFixed(decimals)} ${units[exponent]}`;
}

/** Formats a percentage with one decimal place. */
export function formatPercent(pct: number): string {
  if (!Number.isFinite(pct) || pct < 0) {
    return "0.0%";
  }
  return `${pct.toFixed(1)}%`;
}
