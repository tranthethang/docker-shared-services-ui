import { useEffect } from "react";
import type { BatchDiff } from "../api/types.ts";
import { t } from "../i18n/messages.ts";

export interface ConfirmApplyModalProps {
  open: boolean;
  diff: BatchDiff;
  opsBusy: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}

export function ConfirmApplyModal({
  open,
  diff,
  opsBusy,
  onCancel,
  onConfirm,
}: ConfirmApplyModalProps) {
  useEffect(() => {
    if (!open) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onCancel();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [open, onCancel]);

  if (!open) {
    return null;
  }

  const hasChanges = diff.up.length > 0 || diff.down.length > 0;

  return (
    <div className="modal-backdrop" onClick={onCancel}>
      <div
        className="modal-panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirm-apply-title"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="modal-header">
          <div>
            <h2 id="confirm-apply-title" className="modal-title">
              {t("confirmApplyTitle")}
            </h2>
            <p className="modal-subtitle">{t("confirmApplySubtitle")}</p>
          </div>
          <button type="button" className="btn btn--ghost btn--sm" onClick={onCancel}>
            {t("drawerClose")}
          </button>
        </div>

        <div className="modal-body">
          {!hasChanges ? (
            <p className="modal-empty">{t("confirmApplyEmpty")}</p>
          ) : (
            <div className="diff-queue-grid">
              {diff.up.length > 0 && (
                <section className="diff-queue-section">
                  <h3 className="diff-queue-heading diff-queue-heading--up">
                    {t("confirmApplyUpHeading", { count: diff.up.length })}
                  </h3>
                  <ul className="diff-queue-list">
                    {diff.up.map((name) => (
                      <li key={name} className="diff-queue-item mono-value">
                        {name}
                      </li>
                    ))}
                  </ul>
                </section>
              )}

              {diff.down.length > 0 && (
                <section className="diff-queue-section">
                  <h3 className="diff-queue-heading diff-queue-heading--down">
                    {t("confirmApplyDownHeading", { count: diff.down.length })}
                  </h3>
                  <ul className="diff-queue-list">
                    {diff.down.map((name) => (
                      <li key={name} className="diff-queue-item mono-value">
                        {name}
                      </li>
                    ))}
                  </ul>
                </section>
              )}
            </div>
          )}
        </div>

        <div className="modal-footer">
          <button type="button" className="btn btn--secondary" onClick={onCancel}>
            {t("confirmApplyCancel")}
          </button>
          <button
            type="button"
            className="btn btn--primary"
            disabled={opsBusy || !hasChanges}
            onClick={onConfirm}
          >
            {t("confirmApplySubmit")}
          </button>
        </div>
      </div>
    </div>
  );
}
