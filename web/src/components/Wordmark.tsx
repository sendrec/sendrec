import { brandName } from "../utils/brand";

interface WordmarkProps {
  variant: "nav" | "auth";
}

// The product's name as this install presents it: SendRec's two-tone wordmark,
// or the instance's own name when a self-hosted install sets one.
export function Wordmark({ variant }: WordmarkProps) {
  const name = brandName();

  if (variant === "nav") {
    return name ? (
      <span className="logo-send">{name}</span>
    ) : (
      <>
        <span className="logo-send">Send</span>
        <span className="logo-rec">Rec</span>
      </>
    );
  }

  return (
    <span className="auth-logo">
      {name ?? (
        <>
          <span className="auth-logo-send">Send</span>
          <span className="auth-logo-rec">Rec</span>
        </>
      )}
    </span>
  );
}
