// Package github provides GitHub App authentication and API client wrappers.
package github

import (
	"fmt"
	"net/http"

	ghinstallation "github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v70/github"
)

// AppAuth manages GitHub App authentication and creates per-installation clients.
type AppAuth struct {
	appID     int64
	transport *ghinstallation.AppsTransport
}

// NewAppAuth creates a new AppAuth from the given app ID and PEM private key path.
func NewAppAuth(appID int64, privateKeyPath string) (*AppAuth, error) {
	transport, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, appID, privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create app transport: %w", err)
	}
	return &AppAuth{
		appID:     appID,
		transport: transport,
	}, nil
}

// NewAppAuthFromTransport creates an AppAuth with a custom AppsTransport (useful for testing).
func NewAppAuthFromTransport(appID int64, transport *ghinstallation.AppsTransport) *AppAuth {
	return &AppAuth{
		appID:     appID,
		transport: transport,
	}
}

// ClientForInstallation returns a go-github client authenticated with a short-lived
// installation access token for the given installation ID.
func (a *AppAuth) ClientForInstallation(installationID int64) *github.Client {
	installTransport := ghinstallation.NewFromAppsTransport(a.transport, installationID)
	return github.NewClient(&http.Client{Transport: installTransport})
}
