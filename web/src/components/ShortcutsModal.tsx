import { useEffect } from "react";
import { type MessageKey, t } from "../i18n/messages.ts";

export interface ShortcutsModalProps {
  open: boolean;
  onClose: () => void;
}

const SHORTCUT_ITEMS: readonly { keyLabel: string; descKey: MessageKey }[] = [
  { keyLabel: "/", descKey: "shortcutFocusSearch" },
  { keyLabel: "r", descKey: "shortcutRefresh" },
  { keyLabel: "p", descKey: "shortcutOpenPresets" },
  { keyLabel: "?", descKey: "shortcutToggleHelp" },
  { keyLabel: "Esc", descKey: "shortcutCloseOverlay" },
];

export function ShortcutsModal({ open, onClose }: ShortcutsModalProps) {
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
        className="modal-panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby="shortcuts-modal-title"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="modal-header">
          <div>
            <h2 id="shortcuts-modal-title" className="modal-title">
              {t("shortcutsModalTitle")}
            </h2>
            <p className="modal-subtitle">{t("shortcutsModalSubtitle")}</p>
          </div>
          <button type="button" className="btn btn--ghost btn--sm" onClick={onClose}>
            {t("presetsCloseAction")}
          </button>
        </div>

        <div className="modal-body">
          <ul className="shortcuts-list">
            {SHORTCUT_ITEMS.map((item) => (
              <li key={item.keyLabel} className="shortcut-row">
                <span className="shortcut-desc">{t(item.descKey)}</span>
                <kbd className="shortcut-key mono-value">{item.keyLabel}</kbd>
              </li>
            ))}
          </ul>
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
