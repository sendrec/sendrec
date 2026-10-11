package plans

import (
	"context"

	"github.com/sendrec/sendrec/internal/database"
)

// HasWorkspaceBusiness reports whether a workspace gets Business features:
// either the workspace is subscribed to Business itself, or its owner holds a
// personal Business subscription, which covers the workspaces they own.
func HasWorkspaceBusiness(ctx context.Context, db database.DBTX, orgID, orgPlan string) (bool, error) {
	if orgPlan == "business" {
		return true, nil
	}
	var ownerHasBusiness bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM organization_members om
		   JOIN users u ON u.id = om.user_id
		   WHERE om.organization_id = $1 AND om.role = 'owner' AND u.subscription_plan = 'business')`,
		orgID,
	).Scan(&ownerHasBusiness)
	return ownerHasBusiness, err
}
