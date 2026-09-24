package organization

import (
	"context"

	"github.com/sendrec/sendrec/internal/database"
	"github.com/sendrec/sendrec/internal/plans"
)

type EmailSender interface {
	SendOrgInvite(ctx context.Context, toEmail, orgName, inviterName, acceptLink string) error
}

type Handler struct {
	db           database.DBTX
	baseURL      string
	emailSender  EmailSender
	maxOrgsOwned int
}

func NewHandler(db database.DBTX, baseURL string) *Handler {
	return &Handler{db: db, baseURL: baseURL, maxOrgsOwned: plans.Free.MaxOrgsOwned}
}

// SetMaxOrgsOwned sets how many workspaces a free-plan user may own; 0 lifts the
// cap. A self-hosted install without billing never leaves the free plan, so this
// is the only way its operator can allow more than one.
func (h *Handler) SetMaxOrgsOwned(n int) {
	h.maxOrgsOwned = n
}

func (h *Handler) SetEmailSender(sender EmailSender) {
	h.emailSender = sender
}
