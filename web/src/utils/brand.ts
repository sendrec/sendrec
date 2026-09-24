// A self-hosted install's own name and logo, which the server writes into
// index.html from BRANDING_DEFAULT_NAME and _LOGO_URL. #267.
function meta(name: string): string | null {
  return document.querySelector(`meta[name="${name}"]`)?.getAttribute("content") || null;
}

export function brandName(): string | null {
  return meta("sendrec:brand-name");
}

export function brandLogoSrc(): string {
  return meta("sendrec:brand-logo") ?? "/images/logo.png";
}
