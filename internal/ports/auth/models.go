package auth

// Claims representa la información extraída del token.
type Claims struct {
	UserID   string
	Email    string
	TenantID string
}
