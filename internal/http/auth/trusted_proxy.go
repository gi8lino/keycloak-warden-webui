package auth

import (
	"context"
	"strings"

	appkitauth "github.com/containeroo/appkit/auth"
	"github.com/containeroo/appkit/auth/proxy"
)

// proxyIssuer namespaces trusted-proxy subjects in the local account directory.
const proxyIssuer = "warden-webui-proxy"

// newTrustedProxy validates trusted peers and resolves externally verified administrators.
func newTrustedProxy(cfg proxy.Config, adminGroup string, resolve func(context.Context, string, string) (User, error)) (*proxy.Authenticator[User], error) {
	authenticator, err := proxy.New[User](cfg, proxyIssuer, func(ctx context.Context, identity appkitauth.ExternalIdentity) (User, error) {
		if !hasGroup(identity.Groups, adminGroup) {
			return User{}, appkitauth.ErrForbidden
		}

		return resolve(ctx, identity.Issuer, identity.Subject)
	})
	if err != nil {
		return nil, err
	}

	return &authenticator, nil
}

// hasGroup matches an exact trusted external group without substring matching.
func hasGroup(groups []string, expected string) bool {
	for _, group := range groups {
		if strings.TrimSpace(group) == expected {
			return true
		}
	}

	return false
}
