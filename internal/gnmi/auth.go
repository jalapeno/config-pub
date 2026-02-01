package gnmi

import "context"

type basicAuth struct {
	username   string
	password   string
	requireTLS bool
}

func newBasicAuth(username, password string, requireTLS bool) *basicAuth {
	return &basicAuth{
		username:   username,
		password:   password,
		requireTLS: requireTLS,
	}
}

func (b *basicAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"username": b.username,
		"password": b.password,
	}, nil
}

func (b *basicAuth) RequireTransportSecurity() bool {
	return b.requireTLS
}
