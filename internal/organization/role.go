package organization

// IsAdminOrOwner reports whether the given workspace role grants elevated
// permissions (owner or admin).
func IsAdminOrOwner(role string) bool {
	return role == "owner" || role == "admin"
}
