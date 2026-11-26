import { useEffect, useState } from "react";
import type {
  ExportPreviewResponse,
  ExportResult,
} from "../api/admin.ts";
import { postExportPackage, postExportPreview } from "../api/admin.ts";
import { t } from "../i18n/messages.ts";

interface PackageWizardProps {
  availableServices: readonly string[];
  defaultServices: readonly string[];
  onError: (message: string) => void;
}

export function PackageWizard({
  availableServices,
  defaultServices,
  onError,
}: PackageWizardProps) {
  const [stackName, setStackName] = useState("");
  const [selected, setSelected] = useState<Set<string>>(
    () => new Set(defaultServices),
  );
  const [preview, setPreview] = useState<ExportPreviewResponse | null>(null);
  const [exportResult, setExportResult] = useState<ExportResult | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (selected.size === 0 && defaultServices.length > 0) {
      setSelected(new Set(defaultServices));
    }
  }, [defaultServices, selected.size]);

  const toggleService = (name: string) => {
    setExportResult(null);
    setPreview(null);
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(name)) {
        next.delete(name);
      } else {
        next.add(name);
      }
      return next;
    });
  };

  const handlePreview = async () => {
    setBusy(true);
    setExportResult(null);
    try {
      const res = await postExportPreview([...selected]);
      setPreview(res);
    } catch (err: unknown) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const handleExport = async () => {
    setBusy(true);
    try {
      const res = await postExportPackage(stackName.trim(), [...selected]);
      setExportResult(res);
      setPreview({
        checked: [...selected],
        added: res.added,
        resolved: res.services,
      });
    } catch (err: unknown) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="admin-view">
      <section className="admin-panel">
        <div className="admin-panel-head">
          <div>
            <h2 className="admin-panel-title">{t("pkgTitle")}</h2>
            <p className="admin-panel-desc">{t("pkgSubtitle")}</p>
          </div>
          <div className="admin-actions">
            <button
              type="button"
              className="btn btn--secondary"
              disabled={busy || selected.size === 0}
              onClick={() => void handlePreview()}
            >
              {t("pkgPreviewAction")}
            </button>
            <button
              type="button"
              className="btn btn--primary"
              disabled={busy || selected.size === 0 || stackName.trim().length === 0}
              onClick={() => void handleExport()}
            >
              {t("pkgExportAction")}
            </button>
          </div>
        </div>

        <label className="filter-field">
          <span className="filter-label">{t("pkgNameLabel")}</span>
          <input
            type="text"
            className="filter-input mono-value"
            value={stackName}
            onChange={(e) => {
              setStackName(e.target.value);
              setExportResult(null);
            }}
            placeholder={t("pkgNamePlaceholder")}
          />
          <span className="telemetry-sub">{t("pkgNameHint")}</span>
        </label>

        <div>
          <p className="filter-label">{t("pkgServicesLabel")}</p>
          <div className="admin-check-grid">
            {availableServices.map((svc) => (
              <label key={svc} className="admin-check-item">
                <input
                  type="checkbox"
                  checked={selected.has(svc)}
                  onChange={() => toggleService(svc)}
                />
                <span className="mono-value">{svc}</span>
              </label>
            ))}
          </div>
        </div>
      </section>

      {preview && (
        <section className="admin-panel" aria-live="polite">
          <h3 className="admin-panel-title">{t("pkgPreviewHeading")}</h3>
          <div className="admin-grid">
            <div className="admin-list-item">
              <span className="filter-label">
                {t("pkgCheckedLabel", { count: preview.checked.length })}
              </span>
              <span className="mono-value">{preview.checked.join(", ")}</span>
            </div>
            <div className="admin-list-item">
              <span className="filter-label">
                {t("pkgAddedLabel", { count: preview.added.length })}
              </span>
              <span className="mono-value">
                {preview.added.length > 0
                  ? preview.added.join(", ")
                  : t("pkgNoAddedDeps")}
              </span>
            </div>
          </div>
        </section>
      )}

      {exportResult && (
        <section className="admin-panel" role="status">
          <h3 className="admin-panel-title">{t("pkgSuccessTitle")}</h3>
          <p className="admin-panel-desc mono-value">
            {t("pkgSuccessPath", { path: exportResult.outputPath })}
          </p>
          <p className="admin-panel-desc mono-value">
            {exportResult.services.join(" -> ")}
          </p>
        </section>
      )}
    </div>
  );
}
