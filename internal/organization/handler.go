package organization

import "github.com/sendrec/sendrec/internal/database"

type Handler struct {
	db      database.DBTX
	baseURL string
}

func NewHandler(db database.DBTX, baseURL string) *Handler {
	return &Handler{db: db, baseURL: baseURL}
}
