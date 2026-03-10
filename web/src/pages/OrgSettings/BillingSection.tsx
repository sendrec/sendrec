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
      setBillingMessage(err instanceof Error ? err.message : "Failed to start checkout");
    } finally {
      setUpgrading(false);
    }
  }

  function handleUpgrade(plan: string) {
    if (billing?.subscriptionStatus && billing.subscriptionStatus !== "canceled") {
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
      message: "Cancel this workspace's Pro subscription? Access continues until the end of the billing period.",
      onConfirm: async () => {
        setConfirmDialog(null);
        setCanceling(true);
        setBillingMessage("");
        try {
          await apiFetch(`/api/organizations/${orgId}/billing`, {
            method: "DELETE",
          });
          setBillingMessage("Subscription canceled.");
          setBilling((b) => b ? { ...b, subscriptionStatus: "canceled" } : b);
        } catch (err) {
          setBillingMessage(err instanceof Error ? err.message : "Failed to cancel");
        } finally {
          setCanceling(false);
        }
      },
    });
  }

  return (
    <div className="card settings-section">
      <div className="card-header">
        <h2>Billing</h2>
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
          Subscription canceled. Access continues until the end of the billing period.
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
  );
}
