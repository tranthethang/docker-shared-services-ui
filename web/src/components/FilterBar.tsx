import { t } from "../i18n/messages.ts";

export interface FilterBarProps {
  query: string;
  onQueryChange: (value: string) => void;
  category: string;
  onCategoryChange: (value: string) => void;
  categories: readonly string[];
  activeOnly: boolean;
  onActiveOnlyChange: (value: boolean) => void;
  activeCount?: number;
  totalCount?: number;
  onClear: () => void;
  hasActiveFilters: boolean;
}

export function FilterBar({
  query,
  onQueryChange,
  category,
  onCategoryChange,
  categories,
  activeOnly,
  onActiveOnlyChange,
  activeCount,
  totalCount,
  onClear,
  hasActiveFilters,
}: FilterBarProps) {
  const showCounts = activeCount !== undefined && totalCount !== undefined;

  return (
    <section className="filter-bar" aria-label={t("searchLabel")}>
      <div className="filter-field filter-field--search">
        <label className="filter-label" htmlFor="service-search-input">
          {t("searchLabel")}
        </label>
        <input
          id="service-search-input"
          type="search"
          className="filter-input"
          value={query}
          onChange={(e) => onQueryChange(e.target.value)}
          placeholder={t("searchPlaceholder")}
        />
      </div>

      <div className="filter-field filter-field--category">
        <label className="filter-label" htmlFor="service-category-select">
          {t("categoryLabel")}
        </label>
        <select
          id="service-category-select"
          className="filter-select"
          value={category}
          onChange={(e) => onCategoryChange(e.target.value)}
        >
          <option value="">{t("categoryAll")}</option>
          {categories.map((cat) => (
            <option key={cat} value={cat}>
              {cat}
            </option>
          ))}
        </select>
      </div>

      <div className="filter-actions">
        <button
          type="button"
          role="switch"
          aria-checked={activeOnly}
          className={
            activeOnly ? "active-switch active-switch--on" : "active-switch"
          }
          onClick={() => onActiveOnlyChange(!activeOnly)}
        >
          <span className="active-switch-track" aria-hidden="true">
            <span className="active-switch-thumb" />
          </span>
          <span className="active-switch-label">{t("activeOnlyLabel")}</span>
          {showCounts && (
            <span className="active-switch-count mono-value">
              {activeCount}/{totalCount}
            </span>
          )}
        </button>

        {hasActiveFilters && (
          <button type="button" className="btn btn--ghost" onClick={onClear}>
            {t("clearFiltersAction")}
          </button>
        )}
      </div>
    </section>
  );
}
