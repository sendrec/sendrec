import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { Wordmark } from "./Wordmark";
import { brandLogoSrc } from "../utils/brand";

// The server writes these into index.html when a self-hosted install sets
// BRANDING_DEFAULT_NAME and _LOGO_URL. #267.
function setBrand(name: string | null, logo: string | null) {
  document.head.querySelectorAll('meta[name^="sendrec:brand-"]').forEach((m) => m.remove());
  for (const [key, value] of [["sendrec:brand-name", name], ["sendrec:brand-logo", logo]] as const) {
    if (value === null) continue;
    const meta = document.createElement("meta");
    meta.name = key;
    meta.content = value;
    document.head.appendChild(meta);
  }
}

afterEach(() => setBrand(null, null));

describe("Wordmark", () => {
  it.each(["nav", "auth"] as const)("shows SendRec's own wordmark by default (%s)", (variant) => {
    const { container } = render(<Wordmark variant={variant} />);
    expect(container).toHaveTextContent("SendRec");
    expect(screen.getByText("Send")).toBeInTheDocument();
    expect(screen.getByText("Rec")).toBeInTheDocument();
  });

  it.each(["nav", "auth"] as const)("shows the instance's own name when one is set (%s)", (variant) => {
    setBrand("Acme Video", null);
    const { container } = render(<Wordmark variant={variant} />);
    expect(container).toHaveTextContent("Acme Video");
    expect(screen.queryByText("Send")).not.toBeInTheDocument();
  });
});

describe("brandLogoSrc", () => {
  it("is SendRec's logo by default", () => {
    expect(brandLogoSrc()).toBe("/images/logo.png");
  });

  it("is the instance's logo when one is set", () => {
    setBrand(null, "https://cdn.acme.example/logo.png");
    expect(brandLogoSrc()).toBe("https://cdn.acme.example/logo.png");
  });
});
