package auth

// User is a verified Warden Web UI administrator identity.
type User struct {
	// Name is the account username in the local directory.
	Name string
	// Local identifies administrator sessions created through local credentials.
	Local bool
	// Issuer identifies the authority used for external authentication.
	Issuer string
	// Subject identifies the verified external principal.
	Subject string
}
