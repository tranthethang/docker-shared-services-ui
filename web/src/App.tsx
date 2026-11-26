import { useMemo, useState } from "react";
import { ErrorBanner } from "./components/ErrorBanner.tsx";
import { FilterBar } from "./components/FilterBar.tsx";
import { Header } from "./components/Header.tsx";
import { ServiceDrawer } from "./components/ServiceDrawer.tsx";
import { ServiceGroup } from "./components/ServiceGroup.tsx";
import { TelemetryBar } from "./components/TelemetryBar.tsx";
import {
  filterServices,
  groupServicesByCategory,
  useDashboardData,
} from "./hooks/useDashboardData.ts";
import { t } from "./i18n/messages.ts";

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

  const [query, setQuery] = useState("");
  const [selectedCategory, setSelectedCategory] = useState("");
  const [activeOnly, setActiveOnly] = useState(false);
  const [selectedServiceName, setSelectedServiceName] = useState<string | null>(null);

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

  const hasActiveFilters = query.trim().length > 0 || selectedCategory !== "" || activeOnly;

  const handleClearFilters = () => {
    setQuery("");
    setSelectedCategory("");
    setActiveOnly(false);
  };

  return (
    <div className="dash-page">
      <main className="dash-shell">
        <Header
          info={info}
          runningServices={runningServices}
          totalServices={services.length}
          connectionStatus={connectionStatus}
          refreshing={refreshing}
          onRefresh={() => void refresh()}
        />

        {error && (
          <ErrorBanner
            error={error}
            retrying={refreshing}
            onRetry={() => void refresh()}
          />
        )}

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

        {loading && services.length === 0 ? (
          <section className="empty-panel" aria-live="polite">
            <p className="empty-panel-text">{t("loadingDashboard")}</p>
          </section>
        ) : groups.length === 0 ? (
          <section className="empty-panel" aria-live="polite">
            <p className="empty-panel-text">
              {services.length === 0 ? t("noServicesInStack") : t("noMatchingServices")}
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
              />
            ))}
          </div>
        )}
      </main>

      <ServiceDrawer
        service={selectedService}
        telemetry={telemetry}
        onClose={() => setSelectedServiceName(null)}
      />
    </div>
  );
}
