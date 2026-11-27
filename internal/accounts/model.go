// Package accounts defines local administrator directory records shared by authentication and persistence.
package accounts

// LocalUser describes one account in the administration directory.
type LocalUser struct {
	// Username is the local sign-in identifier.
	Username string
	// Enabled determines whether the account can sign in.
	Enabled bool
	// Founder identifies the bootstrap administrator whose access is protected.
	Founder bool
}

// IdentityLink binds a verified provider subject to one local account.
type IdentityLink struct {
	// Issuer identifies the authority that verifies the subject.
	Issuer string
	// Subject is the immutable OIDC subject or trusted proxy username.
	Subject string
	// Username owns the mapping in the local directory.
	Username string
}
