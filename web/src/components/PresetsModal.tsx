import { useEffect, useState } from "react";
import type { Preset } from "../api/types.ts";
import { type CustomPreset, useCustomPresets } from "../hooks/useCustomPresets.ts";
import { t } from "../i18n/messages.ts";

export interface PresetsModalProps {
  open: boolean;
  presets: readonly Preset[];
  selectedServices?: readonly string[];
  loading: boolean;
  error: string | null;
  opsBusy: boolean;
  onSelectPreset: (preset: Preset) => void;
  onClose: () => void;
}

export function PresetsModal({
  open,
  presets,
  selectedServices = [],
  loading,
  error,
  opsBusy,
  onSelectPreset,
  onClose,
}: PresetsModalProps) {
  const { customPresets, createPreset, updatePreset, deletePreset } = useCustomPresets();
  const [formOpen, setFormOpen] = useState(false);
  const [editingPreset, setEditingPreset] = useState<CustomPreset | null>(null);
  const [name, setName] = useState("");
  const [summary, setSummary] = useState("");
  const [replaceServices, setReplaceServices] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) {
      setFormOpen(false);
      setEditingPreset(null);
      setFormError(null);
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [open, onClose]);

  if (!open) {
    return null;
  }

  const openCreateForm = () => {
    setEditingPreset(null);
    setName("");
    setSummary("");
    setReplaceServices(true);
    setFormError(null);
    setFormOpen(true);
  };

  const openEditForm = (preset: CustomPreset) => {
    setEditingPreset(preset);
    setName(preset.name);
    setSummary(preset.summary);
    setReplaceServices(false);
    setFormError(null);
    setFormOpen(true);
  };

  const closeForm = () => {
    setFormOpen(false);
    setEditingPreset(null);
    setFormError(null);
  };

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    const targetServices = editingPreset
      ? replaceServices
        ? selectedServices
        : editingPreset.services
      : selectedServices;

    if (name.trim().length === 0 || targetServices.length === 0) {
      setFormError(t("presetsValidationError"));
      return;
    }

    if (editingPreset) {
      const ok = updatePreset(editingPreset.id, {
        name,
        summary,
        services: targetServices,
      });
      if (!ok) {
        setFormError(t("presetsValidationError"));
        return;
      }
    } else {
      const created = createPreset({
        name,
        summary,
        services: targetServices,
      });
      if (!created) {
        setFormError(t("presetsValidationError"));
        return;
      }
    }
    closeForm();
  };

  const hasAnyPresets = customPresets.length > 0 || presets.length > 0;

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div
        className="modal-panel modal-panel--wide"
        role="dialog"
        aria-modal="true"
        aria-labelledby="presets-modal-title"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="modal-header">
          <div>
            <h2 id="presets-modal-title" className="modal-title">
              {t("presetsModalTitle")}
            </h2>
            <p className="modal-subtitle">{t("presetsModalSubtitle")}</p>
          </div>
          <div className="preset-header-actions">
            {!formOpen && (
              <button
                type="button"
                className="btn btn--secondary btn--sm"
                onClick={openCreateForm}
              >
                {t("presetsCreateOpenAction")}
              </button>
            )}
            <button type="button" className="btn btn--ghost btn--sm" onClick={onClose}>
              {t("presetsCloseAction")}
            </button>
          </div>
        </div>

        <div className="modal-body">
          {formOpen && (
            <form className="preset-form" onSubmit={handleSubmit}>
              <h3 className="preset-form-title">
                {editingPreset ? t("presetsFormEditTitle") : t("presetsFormCreateTitle")}
              </h3>
              <div className="preset-form-grid">
                <label className="preset-form-field">
                  <span className="filter-label">{t("presetsNameLabel")}</span>
                  <input
                    type="text"
                    className="filter-input"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder={t("presetsNamePlaceholder")}
                    autoFocus
                  />
                </label>
                <label className="preset-form-field">
                  <span className="filter-label">{t("presetsSummaryLabel")}</span>
                  <input
                    type="text"
                    className="filter-input"
                    value={summary}
                    onChange={(e) => setSummary(e.target.value)}
                    placeholder={t("presetsSummaryPlaceholder")}
                  />
                </label>
              </div>

              {editingPreset ? (
                <label className="preset-form-check">
                  <input
                    type="checkbox"
                    checked={replaceServices}
                    onChange={(e) => setReplaceServices(e.target.checked)}
                  />
                  <span>
                    {t("presetsUseCurrentServices", { count: selectedServices.length })}
                  </span>
                </label>
              ) : (
                <p className="preset-form-hint">
                  {t("presetsCurrentSelectionInfo", { count: selectedServices.length })}
                </p>
              )}

              {formError && (
                <p className="modal-error" role="alert">
                  {formError}
                </p>
              )}

              <div className="preset-form-actions">
                <button type="button" className="btn btn--ghost btn--sm" onClick={closeForm}>
                  {t("presetsCancelFormAction")}
                </button>
                <button type="submit" className="btn btn--primary btn--sm">
                  {editingPreset ? t("presetsUpdateAction") : t("presetsSaveAction")}
                </button>
              </div>
            </form>
          )}

          {customPresets.length > 0 && (
            <section className="preset-section" aria-label={t("presetsCustomHeading")}>
              <h3 className="preset-section-heading">{t("presetsCustomHeading")}</h3>
              <div className="presets-grid">
                {customPresets.map((preset) => (
                  <article key={preset.id} className="preset-card">
                    <div className="preset-card-top">
                      <h4 className="preset-card-title">{preset.name}</h4>
                      <div className="preset-card-badges">
                        <span className="preset-custom-badge">{t("presetsCustomBadge")}</span>
                        <span className="preset-count-pill mono-value">
                          {t("presetsServiceCount", { count: preset.services.length })}
                        </span>
                      </div>
                    </div>
                    {preset.summary && (
                      <p className="preset-card-summary">{preset.summary}</p>
                    )}
                    <div className="preset-services">
                      {preset.services.map((svc) => (
                        <span key={svc} className="preset-service-chip mono-value">
                          {svc}
                        </span>
                      ))}
                    </div>
                    <div className="preset-card-actions">
                      <button
                        type="button"
                        className="btn btn--primary btn--sm"
                        disabled={opsBusy}
                        onClick={() => onSelectPreset(preset)}
                      >
                        {t("presetsApplyAction")}
                      </button>
                      <button
                        type="button"
                        className="btn btn--secondary btn--sm"
                        onClick={() => openEditForm(preset)}
                      >
                        {t("presetsEditAction")}
                      </button>
                      <button
                        type="button"
                        className="btn btn--ghost btn--sm"
                        onClick={() => deletePreset(preset.id)}
                      >
                        {t("presetsDeleteAction")}
                      </button>
                    </div>
                  </article>
                ))}
              </div>
            </section>
          )}

          {loading ? (
            <p className="modal-empty">{t("presetsLoading")}</p>
          ) : error ? (
            <p className="modal-error" role="alert">
              {error}
            </p>
          ) : !hasAnyPresets ? (
            <p className="modal-empty">{t("presetsEmpty")}</p>
          ) : presets.length > 0 ? (
            <section className="preset-section" aria-label={t("presetsBuiltinHeading")}>
              {customPresets.length > 0 && (
                <h3 className="preset-section-heading">{t("presetsBuiltinHeading")}</h3>
              )}
              <div className="presets-grid">
                {presets.map((preset) => (
                  <article key={preset.name} className="preset-card">
                    <div className="preset-card-top">
                      <h4 className="preset-card-title">{preset.name}</h4>
                      <span className="preset-count-pill mono-value">
                        {t("presetsServiceCount", { count: preset.services.length })}
                      </span>
                    </div>
                    <p className="preset-card-summary">{preset.summary}</p>
                    <div className="preset-services">
                      {preset.services.map((svc) => (
                        <span key={svc} className="preset-service-chip mono-value">
                          {svc}
                        </span>
                      ))}
                    </div>
                    <div className="preset-card-actions">
                      <button
                        type="button"
                        className="btn btn--primary btn--sm"
                        disabled={opsBusy}
                        onClick={() => onSelectPreset(preset)}
                      >
                        {t("presetsApplyAction")}
                      </button>
                    </div>
                  </article>
                ))}
              </div>
            </section>
          ) : null}
        </div>

        <div className="modal-footer">
          <button type="button" className="btn btn--secondary" onClick={onClose}>
            {t("presetsCloseAction")}
          </button>
        </div>
      </div>
    </div>
  );
}
