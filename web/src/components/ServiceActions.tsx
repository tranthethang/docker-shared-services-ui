import type { ServiceAction, ServiceDTO } from "../api/types.ts";
import { t, type MessageKey } from "../i18n/messages.ts";

export interface ServiceActionsProps {
  service: ServiceDTO;
  opsBusy: boolean;
  onAction: (serviceName: string, action: ServiceAction) => void;
  compact?: boolean;
}

interface ActionSpec {
  action: ServiceAction;
  labelKey: MessageKey;
  variant: "primary" | "secondary";
}

const ACTION_SPECS: readonly ActionSpec[] = [
  { action: "up", labelKey: "actionUp", variant: "primary" },
  { action: "stop", labelKey: "actionStop", variant: "secondary" },
  { action: "down", labelKey: "actionDown", variant: "secondary" },
  { action: "restart", labelKey: "actionRestart", variant: "secondary" },
];

export function ServiceActions({
  service,
  opsBusy,
  onAction,
  compact = false,
}: ServiceActionsProps) {
  const isLocked = Boolean(service.locked);
  const lockReason = service.lockedReason || t("serviceLockedFallback");
  const isAbsent = service.state === "absent";

  return (
    <div className={compact ? "service-actions service-actions--compact" : "service-actions"}>
      {ACTION_SPECS.map(({ action, labelKey, variant }) => {
        const blockedByLock = isLocked && (action === "up" || action === "restart");
        const blockedByAbsent = isAbsent && (action === "stop" || action === "restart");
        const disabled = opsBusy || blockedByLock || blockedByAbsent;

        let title: string | undefined;
        if (opsBusy) {
          title = t("opsBusyTooltip");
        } else if (blockedByLock) {
          title = lockReason;
        }

        return (
          <button
            key={action}
            type="button"
            className={`btn btn--${variant} btn--sm`}
            disabled={disabled}
            title={title}
            onClick={(event) => {
              event.stopPropagation();
              onAction(service.name, action);
            }}
          >
            {t(labelKey)}
          </button>
        );
      })}
    </div>
  );
}
