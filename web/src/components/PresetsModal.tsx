import { useEffect } from "react";
import type { Preset } from "../api/types.ts";
import { t } from "../i18n/messages.ts";

export interface PresetsModalProps {
  open: boolean;
  presets: readonly Preset[];
  loading: boolean;
  error: string | null;
  opsBusy: boolean;
  onSelectPreset: (preset: Preset) => void;
  onClose: () => void;
}

export function PresetsModal({
  open,
  presets,
  loading,
  error,
  opsBusy,
  onSelectPreset,
  onClose,
}: PresetsModalProps) {
  useEffect(() => {
    if (!open) {
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
          <button type="button" className="btn btn--ghost btn--sm" onClick={onClose}>
            {t("presetsCloseAction")}
          </button>
        </div>

        <div className="modal-body">
          {loading ? (
            <p className="modal-empty">{t("presetsLoading")}</p>
          ) : error ? (
            <p className="modal-error" role="alert">
              {error}
            </p>
          ) : presets.length === 0 ? (
            <p className="modal-empty">{t("presetsEmpty")}</p>
          ) : (
            <div className="presets-grid">
              {presets.map((preset) => (
                <article key={preset.name} className="preset-card">
                  <div className="preset-card-top">
                    <h3 className="preset-card-title">{preset.name}</h3>
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
          )}
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
