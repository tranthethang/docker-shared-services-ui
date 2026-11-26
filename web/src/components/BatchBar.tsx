import type { BatchDiff } from "../api/types.ts";
import { t } from "../i18n/messages.ts";

export interface BatchBarProps {
  selectedCount: number;
  diff: BatchDiff;
  opsBusy: boolean;
  onOpenPresets: () => void;
  onReset: () => void;
  onApply: () => void;
}

export function BatchBar({
  selectedCount,
  diff,
  opsBusy,
  onOpenPresets,
  onReset,
  onApply,
}: BatchBarProps) {
  const hasChanges = diff.up.length > 0 || diff.down.length > 0;

  return (
    <section className="batch-bar" aria-label={t("batchApplyAction")}>
      <div className="batch-bar-summary">
        <span className="batch-selected-pill mono-value">
          {t("batchSelectedCount", { count: selectedCount })}
        </span>

        <div className="batch-diff-pills">
          {hasChanges ? (
            <>
              {diff.up.length > 0 && (
                <span className="diff-pill diff-pill--up mono-value">
                  {t("batchWillStart", { count: diff.up.length })}
                </span>
              )}
              {diff.down.length > 0 && (
                <span className="diff-pill diff-pill--down mono-value">
                  {t("batchWillStop", { count: diff.down.length })}
                </span>
              )}
            </>
          ) : (
            <span className="batch-no-changes">{t("batchNoChanges")}</span>
          )}
        </div>
      </div>

      <div className="batch-bar-actions">
        <button
          type="button"
          className="btn btn--secondary btn--sm"
          disabled={opsBusy}
          onClick={onOpenPresets}
        >
          {t("presetsOpenAction")}
        </button>

        <button
          type="button"
          className="btn btn--ghost btn--sm"
          disabled={opsBusy || !hasChanges}
          onClick={onReset}
        >
          {t("batchResetAction")}
        </button>

        <button
          type="button"
          className="btn btn--primary btn--sm"
          disabled={opsBusy || !hasChanges}
          onClick={onApply}
        >
          {t("batchApplyAction")}
        </button>
      </div>
    </section>
  );
}
