import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { CaptureWarning } from "./CaptureWarning";

describe("CaptureWarning", () => {
  it("tells the owner what went wrong with the capture", () => {
    render(<CaptureWarning warning="This recording has sound but no moving picture." />);

    expect(screen.getByRole("note")).toHaveTextContent(
      "This recording has sound but no moving picture.",
    );
  });

  it.each([
    ["nothing to report", null],
    ["not probed yet", undefined],
    ["an empty reason", ""],
  ])("renders nothing for %s", (_name, warning) => {
    const { container } = render(<CaptureWarning warning={warning} />);

    expect(container).toBeEmptyDOMElement();
  });
});
