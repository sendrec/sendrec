import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DocumentModal } from "./DocumentModal";

describe("DocumentModal", () => {
  it("renders markdown content", () => {
    render(
      <DocumentModal document={"## Hello\n\n- Item 1"} onClose={vi.fn()} />
    );
    expect(screen.getByText("Hello")).toBeInTheDocument();
    expect(screen.getByText("Item 1")).toBeInTheDocument();
  });

  it("renders modal title", () => {
    render(
      <DocumentModal document="Content" onClose={vi.fn()} />
    );
    expect(screen.getByText("Generated Document")).toBeInTheDocument();
  });

  it("calls onClose when close button clicked", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<DocumentModal document="Content" onClose={onClose} />);
    await user.click(screen.getByRole("button", { name: /Close/ }));
    expect(onClose).toHaveBeenCalled();
  });

  it("closes on Escape key", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<DocumentModal document="Content" onClose={onClose} />);
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalled();
  });

  it("copies document to clipboard", async () => {
    const user = userEvent.setup();
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      writable: true,
      configurable: true,
    });

    render(<DocumentModal document="## Doc content" onClose={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: /Copy to clipboard/ }));
    expect(writeText).toHaveBeenCalledWith("## Doc content");
  });
});
