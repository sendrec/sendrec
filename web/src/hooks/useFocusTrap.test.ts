import { describe, it, expect, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useRef } from "react";
import { useFocusTrap } from "./useFocusTrap";

function createContainer() {
  const container = document.createElement("div");
  container.innerHTML = `
    <button id="first">First</button>
    <input id="middle" />
    <button id="last">Last</button>
  `;
  document.body.appendChild(container);
  return container;
}

describe("useFocusTrap", () => {
  it("focuses first focusable element on mount", () => {
    const container = createContainer();
    renderHook(() => {
      const ref = useRef(container);
      useFocusTrap(ref);
    });

    expect(document.activeElement).toBe(container.querySelector("#first"));
    document.body.removeChild(container);
  });

  it("traps Tab from last to first element", () => {
    const container = createContainer();
    renderHook(() => {
      const ref = useRef(container);
      useFocusTrap(ref);
    });

    const last = container.querySelector<HTMLElement>("#last")!;
    last.focus();

    const event = new KeyboardEvent("keydown", { key: "Tab", bubbles: true });
    const prevented = vi.spyOn(event, "preventDefault");
    document.dispatchEvent(event);

    expect(prevented).toHaveBeenCalled();
    document.body.removeChild(container);
  });

  it("traps Shift+Tab from first to last element", () => {
    const container = createContainer();
    renderHook(() => {
      const ref = useRef(container);
      useFocusTrap(ref);
    });

    const first = container.querySelector<HTMLElement>("#first")!;
    first.focus();

    const event = new KeyboardEvent("keydown", { key: "Tab", shiftKey: true, bubbles: true });
    const prevented = vi.spyOn(event, "preventDefault");
    document.dispatchEvent(event);

    expect(prevented).toHaveBeenCalled();
    document.body.removeChild(container);
  });

  it("restores focus on unmount", () => {
    const trigger = document.createElement("button");
    document.body.appendChild(trigger);
    trigger.focus();

    const container = createContainer();
    const { unmount } = renderHook(() => {
      const ref = useRef(container);
      useFocusTrap(ref);
    });

    expect(document.activeElement).toBe(container.querySelector("#first"));
    unmount();
    expect(document.activeElement).toBe(trigger);

    document.body.removeChild(container);
    document.body.removeChild(trigger);
  });
});
