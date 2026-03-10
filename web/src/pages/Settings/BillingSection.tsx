import { useState } from "react";
import { apiFetch } from "../../api/client";
import { ConfirmDialog, ConfirmDialogState } from "../../components/ConfirmDialog";
import { BillingData } from "./types";

interface BillingSectionProps {
  billing: BillingData;
}

export function BillingSection({ billing: initialBilling }: BillingSectionProps) {
  const [billing, setBilling] = useState(initialBilling);
  const [upgrading, setUpgrading] = useState(false);
  const [canceling, setCanceling] = useState(false);
  const [billingMessage, setBillingMessage] = useState(() => {
    const params = new URLSearchParams(window.location.search);
    if (params.get("billing") === "success") {
      window.history.replaceState({}, "", window.location.pathname);
      return "Subscription activated successfully!";
    }
    return "";
  });
  const [confirmDialog, setConfirmDialog] = useState<ConfirmDialogState | null>(null);

  async function doUpgrade(plan: string) {
    setUpgrading(true);
    setBillingMessage("");
    try {
      const resp = await apiFetch<{ checkoutUrl?: string; upgraded?: string }>("/api/settings/billing/checkout", {
        method: "POST",
        body: JSON.stringify({ plan }),
      });
      if (resp?.upgraded) {
        window.location.reload();
      } else if (resp?.checkoutUrl) {
        window.location.href = resp.checkoutUrl;
      }
    } catch (err: unknown) {
      setBillingMessage(err instanceof Error ? err.message : "Failed to start checkout");
    } finally {
      setUpgrading(false);
    }
  }

  function handleUpgrade(plan: string) {
    if (billing.subscriptionId) {
      const label = plan === "business" ? "Business" : "Pro";
      setConfirmDialog({
        message: `Upgrade to ${label}? Your remaining credit will be prorated.`,
        confirmLabel: `Upgrade to ${label}`,
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
      message: "Cancel your Pro subscription? You'll keep access until the end of your billing period.",
      onConfirm: async () => {
        setConfirmDialog(null);
        handleCancelSubscriptionConfirmed();
      },
    });
  }

  async function handleCancelSubscriptionConfirmed() {
    setCanceling(true);
    setBillingMessage("");
    try {
      await apiFetch("/api/settings/billing/cancel", { method: "POST" });
      setBillingMessage("Subscription canceled. Access continues until end of billing period.");
      setBilling((b) => ({ ...b, subscriptionStatus: "canceled" }));
    } catch (err: unknown) {
      setBillingMessage(err instanceof Error ? err.message : "Failed to cancel");
    } finally {
      setCanceling(false);
    }
  }

  return (
    <>
      <div className="card settings-section">
        <div className="card-header">
          <h2>Subscription</h2>
          <span className={`plan-badge ${billing.plan !== "free" ? "plan-badge--pro" : ""}`}>
            {billing.plan === "business" ? "Business" : billing.plan === "pro" ? "Pro" : "Free"}
          </span>
        </div>

        {billing.plan === "free" && !billing.subscriptionStatus && (
          <>
            <p className="card-description">
              Upgrade for unlimited videos and recording duration.
            </p>
            <div className="upgrade-card">
              <div className="upgrade-card-info">
                <span className="upgrade-card-plan">Pro</span>
                <span className="upgrade-card-desc">Unlimited videos and duration</span>
              </div>
              <div className="upgrade-card-actions">
                <span className="upgrade-card-price">&euro;8/mo</span>
                <button
                  type="button"
                  className="btn btn--primary"
                  onClick={() => handleUpgrade("pro")}
                  disabled={upgrading}
                >
                  {upgrading ? "Redirecting..." : "Upgrade to Pro"}
                </button>
              </div>
            </div>
            <div className="upgrade-card">
              <div className="upgrade-card-info">
                <span className="upgrade-card-plan">Business</span>
                <span className="upgrade-card-desc">Everything in Pro, plus SSO and workspace access controls</span>
              </div>
              <div className="upgrade-card-actions">
                <span className="upgrade-card-price">&euro;12/mo</span>
                <button
                  type="button"
                  className="btn btn--primary"
                  onClick={() => handleUpgrade("business")}
                  disabled={upgrading}
                >
                  {upgrading ? "Redirecting..." : "Upgrade to Business"}
                </button>
              </div>
            </div>
          </>
        )}

        {billing.plan === "pro" && billing.subscriptionStatus !== "canceled" && (
          <div className="upgrade-card">
            <div className="upgrade-card-info">
              <span className="upgrade-card-plan">Business</span>
              <span className="upgrade-card-desc">Everything in Pro, plus SSO and workspace access controls</span>
            </div>
            <div className="upgrade-card-actions">
              <span className="upgrade-card-price">&euro;12/mo</span>
              <button
                type="button"
                className="btn btn--primary"
                onClick={() => handleUpgrade("business")}
                disabled={upgrading}
              >
                {upgrading ? "Redirecting..." : "Upgrade to Business"}
              </button>
            </div>
          </div>
        )}

        {billing.subscriptionStatus === "canceled" && (
          <p className="card-description">
            Your subscription has been canceled. You have access to Pro features until the end of your billing period.
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
                Manage subscription
              </a>
            )}
            <button
              type="button"
              className="btn btn--danger"
              onClick={handleCancelSubscription}
              disabled={canceling}
            >
              {canceling ? "Canceling..." : "Cancel subscription"}
            </button>
          </div>
        )}

        {billingMessage && (
          <p className="status-message">{billingMessage}</p>
        )}
      </div>

      {confirmDialog && (
        <ConfirmDialog
          message={confirmDialog.message}
          confirmLabel={confirmDialog.confirmLabel}
          danger={confirmDialog.danger}
          onConfirm={confirmDialog.onConfirm}
          onCancel={() => setConfirmDialog(null)}
        />
      )}
    </>
  );
}
