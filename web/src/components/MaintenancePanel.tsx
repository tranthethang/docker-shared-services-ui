import { useState } from "react";
import {
  postMaintenancePrune,
  postMaintenanceTeardown,
  postValidate,
} from "../api/admin.ts";
import type { OpsJob, Telemetry } from "../api/types.ts";
import { formatBytes, t } from "../i18n/messages.ts";

interface MaintenancePanelProps {
  stackName: string;
  telemetry: Telemetry | null;
  opsBusy: boolean;
  onJobStarted: (job: OpsJob) => void;
  onError: (message: string) => void;
}

type ConfirmAction = "prune" | "teardown" | null;

export function MaintenancePanel({
  stackName,
  telemetry,
  opsBusy,
  onJobStarted,
  onError,
}: MaintenancePanelProps) {
  const [confirmAction, setConfirmAction] = useState<ConfirmAction>(null);
  const [typedName, setTypedName] = useState("");

  const reclaimableBytes = telemetry?.diskUsage.reclaimableBytes ?? 0;
  const confirmMatches = typedName === stackName && stackName.length > 0;

  const handleValidate = async () => {
    try {
      const job = await postValidate();
      onJobStarted(job);
    } catch (err: unknown) {
      onError(err instanceof Error ? err.message : String(err));
    }
  };

  const openConfirm = (action: Exclude<ConfirmAction, null>) => {
    setTypedName("");
    setConfirmAction(action);
  };

  const handleConfirmSubmit = async () => {
    if (!confirmAction || !confirmMatches) {
      return;
    }
    try {
      const job =
        confirmAction === "prune"
          ? await postMaintenancePrune(typedName)
          : await postMaintenanceTeardown(typedName);
      setConfirmAction(null);
      setTypedName("");
      onJobStarted(job);
    } catch (err: unknown) {
      setConfirmAction(null);
      onError(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="admin-view">
      <section className="admin-panel">
        <div className="admin-panel-head">
          <div>
            <h2 className="admin-panel-title">{t("maintTitle")}</h2>
            <p className="admin-panel-desc">{t("maintSubtitle")}</p>
          </div>
        </div>
      </section>

      <div className="admin-grid">
        <section className="admin-panel">
          <h3 className="admin-panel-title">{t("maintValidateHeading")}</h3>
          <p className="admin-panel-desc">{t("maintValidateDesc")}</p>
          <div className="admin-actions">
            <button
              type="button"
              className="btn btn--secondary"
              disabled={opsBusy}
              onClick={() => void handleValidate()}
            >
              {t("maintValidateAction")}
            </button>
          </div>
        </section>

        <section className="admin-panel">
          <h3 className="admin-panel-title">{t("maintPruneHeading")}</h3>
          <p className="admin-panel-desc">{t("maintPruneDesc")}</p>
          <div className="admin-list-item">
            <span className="telemetry-label">{t("maintReclaimableLabel")}</span>
            <span className="mono-value">{formatBytes(reclaimableBytes)}</span>
          </div>
          <div className="admin-actions">
            <button
              type="button"
              className="btn btn--secondary"
              disabled={opsBusy}
              onClick={() => openConfirm("prune")}
            >
              {t("maintPruneAction")}
            </button>
          </div>
        </section>

        <section className="admin-panel">
          <h3 className="admin-panel-title">{t("maintTeardownHeading")}</h3>
          <p className="admin-panel-desc">{t("maintTeardownDesc")}</p>
          <div className="admin-actions">
            <button
              type="button"
              className="btn btn--primary"
              disabled={opsBusy}
              onClick={() => openConfirm("teardown")}
            >
              {t("maintTeardownAction")}
            </button>
          </div>
        </section>
      </div>

      {confirmAction && (
        <div
          className="drawer-backdrop"
          role="dialog"
          aria-modal="true"
          aria-labelledby="maint-confirm-title"
        >
          <div className="service-drawer">
            <div className="drawer-header">
              <div>
                <h2 id="maint-confirm-title" className="drawer-title">
                  {t("maintConfirmTitle")}
                </h2>
                <p className="drawer-desc">
                  {t("maintConfirmPrompt", {
                    stackName,
                    action:
                      confirmAction === "prune"
                        ? t("maintPruneAction")
                        : t("maintTeardownAction"),
                  })}
                </p>
              </div>
            </div>

            <label className="filter-field">
              <span className="filter-label">{t("stackLabel")}</span>
              <input
                type="text"
                className="filter-input mono-value"
                value={typedName}
                onChange={(e) => setTypedName(e.target.value)}
                placeholder={t("maintConfirmPlaceholder")}
              />
            </label>

            <div className="admin-actions">
              <button
                type="button"
                className="btn btn--secondary"
                onClick={() => {
                  setConfirmAction(null);
                  setTypedName("");
                }}
              >
                {t("maintConfirmCancel")}
              </button>
              <button
                type="button"
                className="btn btn--primary"
                disabled={!confirmMatches || opsBusy}
                onClick={() => void handleConfirmSubmit()}
              >
                {t("maintConfirmSubmit")}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
