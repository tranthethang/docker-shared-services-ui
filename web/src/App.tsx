import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { EnvStatusResponse } from "./api/admin.ts";
import { fetchEnvStatus } from "./api/admin.ts";
import { fetchPresets } from "./api/client.ts";
import {
  computeBatchDiff,
  postOpsApply,
  postServiceAction,
} from "./api/ops.ts";
import type { Preset, ServiceAction, ServiceDTO } from "./api/types.ts";
import { BatchBar } from "./components/BatchBar.tsx";
import { ConfirmApplyModal } from "./components/ConfirmApplyModal.tsx";
import { EnvSetupView } from "./components/EnvSetupView.tsx";
import { ErrorBanner } from "./components/ErrorBanner.tsx";
import { FilterBar } from "./components/FilterBar.tsx";
import { Header } from "./components/Header.tsx";
import { LogViewer } from "./components/LogViewer.tsx";
import { MaintenancePanel } from "./components/MaintenancePanel.tsx";
import { OpsConsole } from "./components/OpsConsole.tsx";
import { PackageWizard } from "./components/PackageWizard.tsx";
import { PresetsModal } from "./components/PresetsModal.tsx";
import { ServiceDrawer } from "./components/ServiceDrawer.tsx";
import { ServiceGroup } from "./components/ServiceGroup.tsx";
import { ShortcutsModal } from "./components/ShortcutsModal.tsx";
import { TelemetryBar } from "./components/TelemetryBar.tsx";
import {
  filterServices,
  groupServicesByCategory,
  useDashboardData,
} from "./hooks/useDashboardData.ts";
import { useOpsStream } from "./hooks/useOpsStream.ts";
import { useShortcuts } from "./hooks/useShortcuts.ts";
import { useTheme } from "./hooks/useTheme.ts";
import { t } from "./i18n/messages.ts";

type AppTab = "services" | "env" | "exporter" | "maintenance";

function activeServiceNames(services: readonly ServiceDTO[]): Set<string> {
  const names = new Set<string>();
  for (const svc of services) {
    if (svc.state === "running" || svc.state === "partial") {
      names.add(svc.name);
    }
  }
  return names;
}

export function App() {
  const {
    services,
    categories,
    telemetry,
    info,
    cpuHistory,
    loading,
    refreshing,
    error,
    connectionStatus,
    refresh,
  } = useDashboardData();

  const [activeTab, setActiveTab] = useState<AppTab>("services");
  const [envStatus, setEnvStatus] = useState<EnvStatusResponse | null>(null);
  const [envLoading, setEnvLoading] = useState(false);
  const [envError, setEnvError] = useState<string | null>(null);

  const [query, setQuery] = useState("");
  const [selectedCategory, setSelectedCategory] = useState("");
  const [activeOnly, setActiveOnly] = useState(false);
  const [selectedServiceName, setSelectedServiceName] = useState<string | null>(null);

  const [checkedServices, setCheckedServices] = useState<Set<string>>(() => new Set());
  const selectionInitializedRef = useRef(false);
  const syncSelectionAfterApplyRef = useRef(false);

  const [consoleExpanded, setConsoleExpanded] = useState(false);
  const [confirmApplyOpen, setConfirmApplyOpen] = useState(false);
  const [presetsOpen, setPresetsOpen] = useState(false);
  const [presets, setPresets] = useState<Preset[]>([]);
  const [presetsLoading, setPresetsLoading] = useState(false);
  const [presetsError, setPresetsError] = useState<string | null>(null);
  const [shortcutsOpen, setShortcutsOpen] = useState(false);
  const [opsError, setOpsError] = useState<string | null>(null);
  const { theme, cycleTheme } = useTheme();

  const loadEnvStatus = useCallback(async () => {
    setEnvLoading(true);
    setEnvError(null);
    try {
      const next = await fetchEnvStatus();
      setEnvStatus(next);
    } catch (err: unknown) {
      setEnvError(err instanceof Error ? err.message : String(err));
    } finally {
      setEnvLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadEnvStatus();
  }, [loadEnvStatus]);

  const handleJobDone = useCallback(
    (job: { kind: string }) => {
      if (job.kind === "apply") {
        syncSelectionAfterApplyRef.current = true;
      }
      if (job.kind === "setup" || job.kind === "validate") {
        void loadEnvStatus();
      }
      void refresh();
    },
    [loadEnvStatus, refresh],
  );

  const ops = useOpsStream({ onJobDone: handleJobDone });
  const { markJobStarted } = ops;

  useEffect(() => {
    if (services.length === 0) {
      return;
    }
    if (!selectionInitializedRef.current || syncSelectionAfterApplyRef.current) {
      selectionInitializedRef.current = true;
      syncSelectionAfterApplyRef.current = false;
      setCheckedServices(activeServiceNames(services));
    }
  }, [services]);

  useEffect(() => {
    if (ops.busy) {
      setConsoleExpanded(true);
    }
  }, [ops.busy]);

  const runningServices = useMemo(
    () => services.filter((svc) => svc.state === "running" || svc.state === "partial").length,
    [services],
  );

  const filteredServices = useMemo(
    () => filterServices(services, query, selectedCategory, activeOnly),
    [services, query, selectedCategory, activeOnly],
  );

  const groups = useMemo(
    () => groupServicesByCategory(filteredServices, categories),
    [filteredServices, categories],
  );

  const selectedService = useMemo(
    () => services.find((svc) => svc.name === selectedServiceName) ?? null,
    [services, selectedServiceName],
  );

  const batchDiff = useMemo(
    () => computeBatchDiff(services, checkedServices),
    [services, checkedServices],
  );

  const hasActiveFilters = query.trim().length > 0 || selectedCategory !== "" || activeOnly;

  const handleClearFilters = () => {
    setQuery("");
    setSelectedCategory("");
    setActiveOnly(false);
  };

  const handleToggleChecked = useCallback((name: string) => {
    setCheckedServices((prev) => {
      const next = new Set(prev);
      if (next.has(name)) {
        next.delete(name);
      } else {
        next.add(name);
      }
      return next;
    });
  }, []);

  const handleResetSelection = useCallback(() => {
    setCheckedServices(activeServiceNames(services));
  }, [services]);

  const handleServiceAction = useCallback(
    async (serviceName: string, action: ServiceAction) => {
      setOpsError(null);
      try {
        const job = await postServiceAction(serviceName, action);
        markJobStarted(job);
        setConsoleExpanded(true);
      } catch (err: unknown) {
        setOpsError(err instanceof Error ? err.message : String(err));
      }
    },
    [markJobStarted],
  );

  const handleConfirmApply = useCallback(async () => {
    setOpsError(null);
    try {
      const job = await postOpsApply(batchDiff.up, batchDiff.down);
      markJobStarted(job);
      setConfirmApplyOpen(false);
      setConsoleExpanded(true);
    } catch (err: unknown) {
      setConfirmApplyOpen(false);
      setOpsError(err instanceof Error ? err.message : String(err));
    }
  }, [batchDiff.up, batchDiff.down, markJobStarted]);

  const handleOpenPresets = useCallback(async () => {
    setPresetsOpen(true);
    if (presets.length > 0) {
      return;
    }
    setPresetsLoading(true);
    setPresetsError(null);
    try {
      const resp = await fetchPresets();
      setPresets(resp.presets);
    } catch (err: unknown) {
      setPresetsError(err instanceof Error ? err.message : String(err));
    } finally {
      setPresetsLoading(false);
    }
  }, [presets.length]);

  const handleSelectPreset = useCallback((preset: Preset) => {
    setCheckedServices(new Set(preset.services));
    setPresetsOpen(false);
  }, []);

  const handleFocusSearch = useCallback(() => {
    setActiveTab("services");
    window.requestAnimationFrame(() => {
      document.getElementById("service-search-input")?.focus();
    });
  }, []);

  const handleEscape = useCallback(() => {
    if (shortcutsOpen) {
      setShortcutsOpen(false);
    } else if (confirmApplyOpen) {
      setConfirmApplyOpen(false);
    } else if (presetsOpen) {
      setPresetsOpen(false);
    } else if (selectedServiceName !== null) {
      setSelectedServiceName(null);
    }
  }, [confirmApplyOpen, presetsOpen, selectedServiceName, shortcutsOpen]);

  useShortcuts({
    onFocusSearch: handleFocusSearch,
    onRefresh: () => void refresh(),
    onOpenPresets: () => void handleOpenPresets(),
    onToggleHelp: () => setShortcutsOpen((prev) => !prev),
    onEscape: handleEscape,
  });

  return (
    <div className="dash-page">
      <main className="dash-shell dash-shell--with-console">
        <Header
          info={info}
          runningServices={runningServices}
          totalServices={services.length}
          connectionStatus={connectionStatus}
          refreshing={refreshing}
          theme={theme}
          onCycleTheme={cycleTheme}
          onOpenShortcuts={() => setShortcutsOpen(true)}
          onRefresh={() => void refresh()}
        />

        <nav className="app-nav" aria-label={t("navTabServices")}>
          {(
            [
              ["services", "navTabServices"],
              ["env", "navTabEnvSetup"],
              ["exporter", "navTabPackageExporter"],
              ["maintenance", "navTabMaintenance"],
            ] as const
          ).map(([tabKey, labelKey]) => (
            <button
              key={tabKey}
              type="button"
              className={
                activeTab === tabKey
                  ? "app-nav-tab app-nav-tab--active"
                  : "app-nav-tab"
              }
              onClick={() => setActiveTab(tabKey)}
            >
              {t(labelKey)}
            </button>
          ))}
        </nav>

        {error && (
          <ErrorBanner
            error={error}
            retrying={refreshing}
            onRetry={() => void refresh()}
          />
        )}

        {opsError && (
          <section className="error-banner" role="alert">
            <div>
              <h2 className="error-banner-title">{t("opsActionErrorTitle")}</h2>
              <p className="error-banner-msg">{opsError}</p>
            </div>
            <button
              type="button"
              className="btn btn--secondary btn--sm"
              onClick={() => setOpsError(null)}
            >
              {t("drawerClose")}
            </button>
          </section>
        )}

        {activeTab === "env" && (
          <EnvSetupView
            status={envStatus}
            loading={envLoading}
            error={envError}
            opsBusy={ops.busy}
            onRefresh={() => void loadEnvStatus()}
            onJobStarted={(job) => {
              markJobStarted(job);
              setConsoleExpanded(true);
            }}
            onError={setOpsError}
          />
        )}

        {activeTab === "exporter" && (
          <PackageWizard
            availableServices={
              envStatus?.availableServices ?? services.map((s) => s.name)
            }
            defaultServices={envStatus?.defaultExportServices ?? []}
            onError={setOpsError}
          />
        )}

        {activeTab === "maintenance" && (
          <MaintenancePanel
            stackName={envStatus?.stackName ?? (info?.stackName || "default")}
            telemetry={telemetry}
            opsBusy={ops.busy}
            onJobStarted={(job) => {
              markJobStarted(job);
              setConsoleExpanded(true);
            }}
            onError={setOpsError}
          />
        )}

        {activeTab === "services" && (
          <>
            <TelemetryBar
              telemetry={telemetry}
              cpuHistory={cpuHistory}
              loading={loading}
            />

            <FilterBar
              query={query}
              onQueryChange={setQuery}
              category={selectedCategory}
              onCategoryChange={setSelectedCategory}
              categories={categories}
              activeOnly={activeOnly}
              onActiveOnlyChange={setActiveOnly}
              onClear={handleClearFilters}
              hasActiveFilters={hasActiveFilters}
            />

            <BatchBar
              selectedCount={checkedServices.size}
              diff={batchDiff}
              opsBusy={ops.busy}
              onOpenPresets={() => void handleOpenPresets()}
              onReset={handleResetSelection}
              onApply={() => setConfirmApplyOpen(true)}
            />

            {loading && services.length === 0 ? (
              <section className="empty-panel" aria-live="polite">
                <p className="empty-panel-text">{t("loadingDashboard")}</p>
              </section>
            ) : groups.length === 0 ? (
              <section className="empty-panel" aria-live="polite">
                <p className="empty-panel-text">
                  {services.length === 0
                    ? t("noServicesInStack")
                    : t("noMatchingServices")}
                </p>
                {hasActiveFilters && (
                  <button
                    type="button"
                    className="btn btn--secondary"
                    onClick={handleClearFilters}
                  >
                    {t("clearFiltersAction")}
                  </button>
                )}
              </section>
            ) : (
              <div className="service-groups">
                {groups.map((group) => (
                  <ServiceGroup
                    key={group.category}
                    category={group.category}
                    services={group.services}
                    runningCount={group.runningCount}
                    totalCount={group.totalCount}
                    telemetry={telemetry}
                    selectedServiceName={selectedServiceName}
                    onSelectService={setSelectedServiceName}
                    checkedServices={checkedServices}
                    onToggleChecked={handleToggleChecked}
                    opsBusy={ops.busy}
                    onAction={(name, action) =>
                      void handleServiceAction(name, action)
                    }
                  />
                ))}
              </div>
            )}
          </>
        )}
      </main>

      <OpsConsole
        busy={ops.busy}
        currentJob={ops.currentJob}
        lastJob={ops.lastJob}
        lines={ops.lines}
        expanded={consoleExpanded}
        onToggleExpanded={() => setConsoleExpanded((prev) => !prev)}
        onClear={ops.clearLines}
      />

      <PresetsModal
        open={presetsOpen}
        presets={presets}
        selectedServices={Array.from(checkedServices)}
        loading={presetsLoading}
        error={presetsError}
        opsBusy={ops.busy}
        onSelectPreset={handleSelectPreset}
        onClose={() => setPresetsOpen(false)}
      />

      <ShortcutsModal
        open={shortcutsOpen}
        onClose={() => setShortcutsOpen(false)}
      />

      <ConfirmApplyModal
        open={confirmApplyOpen}
        diff={batchDiff}
        opsBusy={ops.busy}
        onCancel={() => setConfirmApplyOpen(false)}
        onConfirm={() => void handleConfirmApply()}
      />

      <ServiceDrawer
        service={selectedService}
        telemetry={telemetry}
        onClose={() => setSelectedServiceName(null)}
        opsBusy={ops.busy}
        onAction={(name, action) => void handleServiceAction(name, action)}
        renderLogs={(svc) => <LogViewer service={svc} />}
      />
    </div>
  );
}
