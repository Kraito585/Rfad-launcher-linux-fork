import { onMounted, onUnmounted } from "vue";
import { Events } from "@wailsio/runtime";

export type NavDirection = "up" | "down" | "left" | "right";
export type GamepadAction = NavDirection | "confirm" | "back";

const FOCUSABLE_SELECTOR = [
  "button",
  "a[href]",
  "input",
  "select",
  "textarea",
  '[tabindex]:not([tabindex="-1"])',
  "[data-gp-focusable]",
].join(", ");

const MODAL_SELECTOR = '[data-gp-modal], [role="dialog"]';

const DEADZONE = 0.45;
const INITIAL_REPEAT_DELAY = 350;
const REPEAT_INTERVAL = 120;
const SCROLL_SPEED = 14;

function isVisible(el: HTMLElement): boolean {
  if (el.closest('[class*="-leave-active"]')) return false;
  const style = window.getComputedStyle(el);
  if (
    style.display === "none" ||
    style.visibility === "hidden" ||
    parseFloat(style.opacity) === 0
  ) {
    return false;
  }
  const rect = el.getBoundingClientRect();
  return rect.width > 0 && rect.height > 0;
}

function getEffectiveZIndex(el: HTMLElement): number {
  let cur: HTMLElement | null = el;
  while (cur && cur !== document.body) {
    const z = window.getComputedStyle(cur).zIndex;
    if (z !== "auto" && z !== "") {
      const parsed = Number.parseInt(z, 10);
      if (!Number.isNaN(parsed)) return parsed;
    }
    cur = cur.parentElement;
  }
  return 0;
}

function getActiveScope(): HTMLElement {
  const modals = Array.from(
    document.querySelectorAll<HTMLElement>(MODAL_SELECTOR),
  ).filter(isVisible);

  if (modals.length === 0) return document.body;

  let top = modals[0];
  let topZ = getEffectiveZIndex(top);
  for (let i = 1; i < modals.length; i++) {
    const z = getEffectiveZIndex(modals[i]);
    if (z >= topZ) {
      top = modals[i];
      topZ = z;
    }
  }
  return top;
}

function getFocusableElements(scope: HTMLElement): HTMLElement[] {
  return Array.from(
    scope.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR),
  ).filter((el) => {
    if (
      el.hasAttribute("disabled") ||
      el.getAttribute("aria-disabled") === "true"
    ) {
      return false;
    }
    if (el.hasAttribute("data-gp-ignore")) return false;
    if (el.closest('[aria-hidden="true"], [inert]')) return false;

    const style = window.getComputedStyle(el);
    if (style.pointerEvents === "none") return false;

    // Не дублируем фокус на вложенной кнопке, если обёртка уже помечена data-gp-focusable
    const focusableAncestor = el.parentElement?.closest<HTMLElement>(
      "[data-gp-focusable]",
    );
    if (focusableAncestor && scope.contains(focusableAncestor)) return false;

    return isVisible(el);
  });
}

function focusElement(el: HTMLElement) {
  if (el.tabIndex < 0) {
    el.setAttribute("tabindex", "0");
  }
  el.focus({ preventScroll: true });
  el.scrollIntoView({
    block: "nearest",
    inline: "nearest",
    behavior: "smooth",
  });
}

function findNextElement(
  current: HTMLElement,
  candidates: HTMLElement[],
  dir: NavDirection,
): HTMLElement | null {
  const a = current.getBoundingClientRect();
  const ax = a.left + a.width / 2;
  const ay = a.top + a.height / 2;

  const evaluate = (strictCone: boolean): HTMLElement | null => {
    let best: HTMLElement | null = null;
    let bestScore = Infinity;

    for (const el of candidates) {
      if (el === current) continue;
      const b = el.getBoundingClientRect();
      const bx = b.left + b.width / 2;
      const by = b.top + b.height / 2;

      const dx = bx - ax;
      const dy = by - ay;

      if (dir === "right" && dx <= 4) continue;
      if (dir === "left" && dx >= -4) continue;
      if (dir === "down" && dy <= 4) continue;
      if (dir === "up" && dy >= -4) continue;

      const horizontal = dir === "left" || dir === "right";
      const primaryDist = horizontal ? Math.abs(dx) : Math.abs(dy);
      const secondaryDist = horizontal ? Math.abs(dy) : Math.abs(dx);

      const crossGap = horizontal
        ? Math.max(0, b.top - a.bottom, a.top - b.bottom)
        : Math.max(0, b.left - a.right, a.left - b.right);

      if (strictCone && crossGap > 0 && secondaryDist > primaryDist * 2.2) {
        continue;
      }

      const primaryGap =
        dir === "right"
          ? Math.max(0, b.left - a.right)
          : dir === "left"
            ? Math.max(0, a.left - b.right)
            : dir === "down"
              ? Math.max(0, b.top - a.bottom)
              : Math.max(0, a.top - b.bottom);

      const score =
        primaryGap + primaryDist * 0.35 + crossGap * 3.5 + secondaryDist * 0.6;
      if (score < bestScore) {
        bestScore = score;
        best = el;
      }
    }

    return best;
  };

  return evaluate(true) ?? evaluate(false);
}

function findScrollableParent(
  el: HTMLElement | null,
  boundary: HTMLElement,
): HTMLElement | null {
  let cur = el;
  while (cur && cur !== boundary.parentElement) {
    const style = window.getComputedStyle(cur);
    const canScrollY =
      (style.overflowY === "auto" || style.overflowY === "scroll") &&
      cur.scrollHeight > cur.clientHeight + 2;
    if (canScrollY) return cur;
    if (cur === boundary) break;
    cur = cur.parentElement;
  }
  return null;
}

function stepRangeInput(input: HTMLInputElement, delta: -1 | 1): boolean {
  const step = Number.parseFloat(input.step) || 1;
  const min = input.min !== "" ? Number.parseFloat(input.min) : -Infinity;
  const max = input.max !== "" ? Number.parseFloat(input.max) : Infinity;
  const cur = Number.parseFloat(input.value) || 0;
  const next = Math.min(max, Math.max(min, cur + delta * step));
  if (next === cur) return false;

  input.value = String(next);
  input.dispatchEvent(new Event("input", { bubbles: true }));
  input.dispatchEvent(new Event("change", { bubbles: true }));
  return true;
}

export function useGamepadNav() {
  let observer: MutationObserver | null = null;
  let isGamepadMode = false;
  let lastMouseX = 0;
  let lastMouseY = 0;

  const modalStack: Array<{
    modal: HTMLElement;
    returnTo: HTMLElement | null;
  }> = [];

  const enableGamepadMode = () => {
    if (!isGamepadMode) {
      isGamepadMode = true;
      document.documentElement.classList.add("gamepad-mode");
    }
  };

  const disableGamepadMode = () => {
    if (isGamepadMode) {
      isGamepadMode = false;
      document.documentElement.classList.remove("gamepad-mode");
    }
  };

  const ensureFocusInScope = (scope: HTMLElement): HTMLElement | null => {
    const active = document.activeElement as HTMLElement | null;
    if (
      active &&
      active !== document.body &&
      scope.contains(active) &&
      isVisible(active) &&
      window.getComputedStyle(active).pointerEvents !== "none"
    ) {
      return active;
    }

    const preferred = scope.querySelector<HTMLElement>("[data-gp-autofocus]");
    if (
      preferred &&
      isVisible(preferred) &&
      window.getComputedStyle(preferred).pointerEvents !== "none"
    ) {
      focusElement(preferred);
      return preferred;
    }

    const items = getFocusableElements(scope);
    if (items.length > 0) {
      focusElement(items[0]);
      return items[0];
    }
    return null;
  };

  const handleAction = (action: GamepadAction) => {
    enableGamepadMode();
    const scope = getActiveScope();

    if (action === "back") {
      if (scope !== document.body) {
        const closeBtn = scope.querySelector<HTMLElement>("[data-gp-close]");
        if (closeBtn && isVisible(closeBtn)) {
          closeBtn.click();
          return;
        }
      }
      const ev = new CustomEvent("gp-back", { cancelable: true });
      window.dispatchEvent(ev);
      if (!ev.defaultPrevented) {
        window.dispatchEvent(
          new KeyboardEvent("keydown", { key: "Escape", bubbles: true }),
        );
      }
      return;
    }

    const active = ensureFocusInScope(scope);
    if (!active) return;

    if (action === "confirm") {
      if (
        active instanceof HTMLInputElement &&
        (active.type === "text" ||
          active.type === "search" ||
          active.type === "number")
      ) {
        active.focus();
        active.select();
        return;
      }
      active.click();
      return;
    }

    if (active instanceof HTMLInputElement && active.type === "range") {
      if (action === "left") {
        stepRangeInput(active, -1);
        return;
      }
      if (action === "right") {
        stepRangeInput(active, 1);
        return;
      }
    }

    const candidates = getFocusableElements(scope);
    const next = findNextElement(active, candidates, action);
    if (next) {
      focusElement(next);
    }
  };

  const handleScroll = (delta: number) => {
    enableGamepadMode();
    const scope = getActiveScope();
    const active = document.activeElement as HTMLElement | null;
    const scrollEl =
      findScrollableParent(
        active && scope.contains(active) ? active : scope,
        scope,
      ) ??
      (scope === document.body ? document.getElementById("patches") : scope);
    if (scrollEl) {
      scrollEl.scrollTop += delta;
    }
  };

  const syncModals = () => {
    while (modalStack.length > 0) {
      const top = modalStack[modalStack.length - 1];
      if (!top.modal.isConnected || !isVisible(top.modal)) {
        modalStack.pop();
        if (
          isGamepadMode &&
          top.returnTo &&
          top.returnTo.isConnected &&
          isVisible(top.returnTo)
        ) {
          focusElement(top.returnTo);
        }
      } else {
        break;
      }
    }

    const currentScope = getActiveScope();
    if (currentScope !== document.body) {
      const alreadyTracked = modalStack.some((m) => m.modal === currentScope);
      if (!alreadyTracked) {
        const prevFocused = document.activeElement as HTMLElement | null;
        modalStack.push({
          modal: currentScope,
          returnTo:
            prevFocused && !currentScope.contains(prevFocused)
              ? prevFocused
              : null,
        });
        if (isGamepadMode) {
          requestAnimationFrame(() => ensureFocusInScope(currentScope));
        }
        return;
      }
    }

    // Если внутри открытой модалки сменился шаг (v-if) и прежний элемент размонтировался
    if (isGamepadMode) {
      const active = document.activeElement as HTMLElement | null;
      if (
        !active ||
        active === document.body ||
        !currentScope.contains(active) ||
        !isVisible(active)
      ) {
        requestAnimationFrame(() => ensureFocusInScope(currentScope));
      }
    }
  };

  const onMouseMove = (e: MouseEvent) => {
    if (
      Math.abs(e.clientX - lastMouseX) > 6 ||
      Math.abs(e.clientY - lastMouseY) > 6
    ) {
      lastMouseX = e.clientX;
      lastMouseY = e.clientY;
      disableGamepadMode();
    }
  };

  // Клавиатурное тестирование (стрелки / Enter / Esc) после входа в режим геймпада
  const onKeyDown = (e: KeyboardEvent) => {
    if (!isGamepadMode) return;
    const active = document.activeElement;
    if (
      active instanceof HTMLInputElement &&
      (active.type === "text" || active.type === "search")
    ) {
      if (e.key === "Escape") {
        active.blur();
        handleAction("back");
      }
      return;
    }
    const map: Record<string, GamepadAction | undefined> = {
      ArrowUp: "up",
      ArrowDown: "down",
      ArrowLeft: "left",
      ArrowRight: "right",
      Enter: "confirm",
      Escape: "back",
    };
    const act = map[e.key];
    if (act) {
      e.preventDefault();
      handleAction(act);
    }
  };

  let unlistenAction: (() => void) | null = null;
  let unlistenScroll: (() => void) | null = null;

  onMounted(() => {
    window.addEventListener("mousemove", onMouseMove, { passive: true });
    window.addEventListener("keydown", onKeyDown);

    observer = new MutationObserver(() => syncModals());
    observer.observe(document.body, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: ["style", "class", "hidden"],
    });

    unlistenAction = Events.On("gamepad-action", (event: any) => {
      if (!document.hasFocus() || document.hidden) return;
      const action = (
        Array.isArray(event.data) ? event.data[0] : event.data
      ) as GamepadAction;
      if (action) handleAction(action);
    });

    unlistenScroll = Events.On("gamepad-scroll", (event: any) => {
      if (!document.hasFocus() || document.hidden) return;
      const delta = Number(
        Array.isArray(event.data) ? event.data[0] : event.data,
      );
      if (!Number.isNaN(delta) && delta !== 0) handleScroll(delta);
    });

    (window as any).__rfadGamepadAction = handleAction;
  });

  onUnmounted(() => {
    unlistenAction?.();
    unlistenScroll?.();
    observer?.disconnect();
    window.removeEventListener("mousemove", onMouseMove);
    window.removeEventListener("keydown", onKeyDown);
    delete (window as any).__rfadGamepadAction;
    disableGamepadMode();
  });

  return { handleAction };
}
