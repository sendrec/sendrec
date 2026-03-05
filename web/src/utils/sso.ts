export function providerLabel(provider: string): string {
  switch (provider) {
    case "google": return "Google";
    case "microsoft": return "Microsoft";
    case "github": return "GitHub";
    default: return provider;
  }
}
