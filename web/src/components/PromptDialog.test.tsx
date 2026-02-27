import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { PromptDialog } from "./PromptDialog";

describe("PromptDialog", () => {
  it("renders title and input", () => {
    render(
      <PromptDialog
        title="Enter password"
        onSubmit={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    expect(screen.getByText("Enter password")).toBeTruthy();
    expect(screen.getByRole("textbox")).toBeTruthy();
  });

  it("calls onSubmit with trimmed input value", () => {
    const onSubmit = vi.fn();
    render(
      <PromptDialog
        title="Enter value"
        onSubmit={onSubmit}
        onCancel={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "  my-password  " },
    });
    fireEvent.submit(screen.getByRole("textbox"));

    expect(onSubmit).toHaveBeenCalledWith("my-password");
  });

  it("does not submit when input is empty", () => {
    const onSubmit = vi.fn();
    render(
      <PromptDialog
        title="Enter value"
        onSubmit={onSubmit}
        onCancel={vi.fn()}
      />,
    );

    fireEvent.submit(screen.getByRole("textbox"));
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("calls onCancel when cancel button is clicked", () => {
    const onCancel = vi.fn();
    render(
      <PromptDialog
        title="Enter value"
        onSubmit={vi.fn()}
        onCancel={onCancel}
      />,
    );

    fireEvent.click(screen.getByText("Cancel"));
    expect(onCancel).toHaveBeenCalledOnce();
  });

  it("calls onCancel when Escape is pressed", () => {
    const onCancel = vi.fn();
    render(
      <PromptDialog
        title="Enter value"
        onSubmit={vi.fn()}
        onCancel={onCancel}
      />,
    );

    fireEvent.keyDown(window, { key: "Escape" });
    expect(onCancel).toHaveBeenCalledOnce();
  });

  it("has dialog role", () => {
    render(
      <PromptDialog
        title="Enter value"
        onSubmit={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    expect(screen.getByRole("dialog")).toBeTruthy();
  });
});
