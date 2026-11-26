import { useEffect } from "react";

export interface ShortcutHandlers {
  onFocusSearch: () => void;
  onRefresh: () => void;
  onOpenPresets: () => void;
  onToggleHelp: () => void;
  onEscape: () => void;
}

/** Returns true when keyboard events originate inside an editable form control. */
export function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  const tag = target.tagName;
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") {
    return true;
  }
  return target.isContentEditable || target.closest("[contenteditable='true']") !== null;
}

/** Registers global dashboard keyboard shortcuts while ignoring form inputs. */
export function useShortcuts({
  onFocusSearch,
  onRefresh,
  onOpenPresets,
  onToggleHelp,
  onEscape,
}: ShortcutHandlers) {
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onEscape();
        return;
      }

      if (event.metaKey || event.ctrlKey || event.altKey || isTypingTarget(event.target)) {
        return;
      }

      if (event.key === "/") {
        event.preventDefault();
        onFocusSearch();
      } else if (event.key === "r" || event.key === "R") {
        event.preventDefault();
        onRefresh();
      } else if (event.key === "p" || event.key === "P") {
        event.preventDefault();
        onOpenPresets();
      } else if (event.key === "?") {
        event.preventDefault();
        onToggleHelp();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onEscape, onFocusSearch, onOpenPresets, onRefresh, onToggleHelp]);
}
