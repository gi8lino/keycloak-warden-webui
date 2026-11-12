// Package server provides Warden Web UI's passive application and management HTTP adapters.
package server

import appkithttpserver "github.com/containeroo/appkit/httpserver"

// New composes the route trees without owning application infrastructure.
func New(config Config) appkithttpserver.Handlers {
	return appkithttpserver.New(appkithttpserver.Config{
		Application: newApplication(config),
		Management:  newManagement(config.Ready),
		RoutePrefix: config.RoutePrefix,
	})
}
