package organization

import (
	"context"

	"github.com/sendrec/sendrec/internal/database"
)

type EmailSender interface {
	SendOrgInvite(ctx context.Context, toEmail, orgName, inviterName, acceptLink string) error
}

type Handler struct {
	db          database.DBTX
	baseURL     string
	emailSender EmailSender
}

func NewHandler(db database.DBTX, baseURL string) *Handler {
	return &Handler{db: db, baseURL: baseURL}
}

func (h *Handler) SetEmailSender(sender EmailSender) {
	h.emailSender = sender
}
