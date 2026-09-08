import { useState } from "react";
import { apiFetch } from "../../api/client";
import type { OrgBilling, SharedSectionProps } from "./types";

interface BillingSectionProps extends SharedSectionProps {
  billing: OrgBilling;
  setBilling: React.Dispatch<React.SetStateAction<OrgBilling | null>>;
}

export function BillingSection({
  orgId,
  billing,
  setBilling,
  setConfirmDialog,
}: BillingSectionProps) {
  const [billingMessage, setBillingMessage] = useState("");
  const [upgrading, setUpgrading] = useState(false);
  const [canceling, setCanceling] = useState(false);

  async function doUpgrade(plan: string) {
    setUpgrading(true);
    setBillingMessage("");
    try {
      const resp = await apiFetch<{ checkoutUrl?: string; upgraded?: string }>(
        `/api/organizations/${orgId}/billing/checkout`,
        {
          method: "POST",
          body: JSON.stringify({ plan }),
        }
      );
      if (resp?.upgraded) {
        window.location.reload();
      } else if (resp?.checkoutUrl) {
        window.location.href = resp.checkoutUrl;
      }
    } catch (err) {
      setBillingMessage(err instanceof Error ? err.message : "Checkout konnte nicht gestartet werden");
    } finally {
      setUpgrading(false);
    }
  }

  function handleUpgrade(plan: string) {
    if (billing?.subscriptionStatus && billing.subscriptionStatus !== "canceled") {
      const label = plan === "business" ? "Business" : "Pro";
      setConfirmDialog({
        message: `Auf ${label} upgraden? Dein verbleibendes Guthaben wird anteilig verrechnet.`,
        confirmLabel: `Auf ${label} upgraden`,
        onConfirm: () => {
          setConfirmDialog(null);
          doUpgrade(plan);
        },
      });
    } else {
      doUpgrade(plan);
    }
  }

  function handleCancelSubscription() {
    setConfirmDialog({
      message: "Pro-Abonnement dieses Arbeitsbereichs kündigen? Der Zugriff bleibt bis zum Ende des Abrechnungszeitraums bestehen.",
      onConfirm: async () => {
        setConfirmDialog(null);
        setCanceling(true);
        setBillingMessage("");
        try {
          await apiFetch(`/api/organizations/${orgId}/billing`, {
            method: "DELETE",
          });
          setBillingMessage("Abonnement gekündigt.");
          setBilling((b) => b ? { ...b, subscriptionStatus: "canceled" } : b);
        } catch (err) {
          setBillingMessage(err instanceof Error ? err.message : "Kündigung fehlgeschlagen");
        } finally {
          setCanceling(false);
        }
      },
    });
  }

  return (
    <div className="card settings-section">
      <div className="card-header">
        <h2>Abrechnung</h2>
        <span className={`plan-badge ${billing.plan !== "free" ? "plan-badge--pro" : ""}`}>
          {billing.plan === "business" ? "Business" : billing.plan === "pro" ? "Pro" : "Kostenlos"}
        </span>
      </div>

      {billing.plan === "free" && !billing.subscriptionStatus && (
        <>
          <p className="card-description">
            Upgrade für unbegrenzte Videos und Aufnahmedauer.
          </p>
          <div className="upgrade-card">
            <div className="upgrade-card-info">
              <span className="upgrade-card-plan">Pro</span>
              <span className="upgrade-card-desc">Unbegrenzte Videos und Videolänge</span>
            </div>
            <div className="upgrade-card-actions">
              <span className="upgrade-card-price">&euro;8/mo</span>
              <button
                type="button"
                className="btn btn--primary"
                onClick={() => handleUpgrade("pro")}
                disabled={upgrading}
              >
                {upgrading ? "Weiterleitung..." : "Auf Pro upgraden"}
              </button>
            </div>
          </div>
          <div className="upgrade-card">
            <div className="upgrade-card-info">
              <span className="upgrade-card-plan">Business</span>
              <span className="upgrade-card-desc">Alles aus Pro plus SSO und Zugriffskontrollen für Arbeitsbereiche</span>
            </div>
            <div className="upgrade-card-actions">
              <span className="upgrade-card-price">&euro;12/mo</span>
              <button
                type="button"
                className="btn btn--primary"
                onClick={() => handleUpgrade("business")}
                disabled={upgrading}
              >
                {upgrading ? "Weiterleitung..." : "Auf Business upgraden"}
              </button>
            </div>
          </div>
        </>
      )}

      {billing.plan === "pro" && billing.subscriptionStatus !== "canceled" && (
        <div className="upgrade-card">
          <div className="upgrade-card-info">
            <span className="upgrade-card-plan">Business</span>
            <span className="upgrade-card-desc">Alles aus Pro plus SSO und Zugriffskontrollen für Arbeitsbereiche</span>
          </div>
          <div className="upgrade-card-actions">
            <span className="upgrade-card-price">&euro;12/mo</span>
            <button
              type="button"
              className="btn btn--primary"
              onClick={() => handleUpgrade("business")}
              disabled={upgrading}
            >
              {upgrading ? "Weiterleitung..." : "Auf Business upgraden"}
            </button>
          </div>
        </div>
      )}

      {billing.subscriptionStatus === "canceled" && (
        <p className="card-description">
          Abonnement gekündigt. Der Zugriff bleibt bis zum Ende des Abrechnungszeitraums bestehen.
        </p>
      )}

      {(billing.plan === "pro" || billing.plan === "business") && billing.subscriptionStatus !== "canceled" && (
        <div className="btn-row">
          {billing.portalUrl && (
            <a
              href={billing.portalUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="billing-portal-link"
            >
              Abonnement verwalten
            </a>
          )}
          <button
            type="button"
            className="btn btn--danger"
            onClick={handleCancelSubscription}
            disabled={canceling}
          >
            {canceling ? "Wird gekündigt..." : "Abonnement kündigen"}
          </button>
        </div>
      )}

      {billingMessage && (
        <p className="status-message">{billingMessage}</p>
      )}
    </div>
  );
}
