// Command warden-webui serves an optional authenticated Warden administration UI.
package main

import (
	"context"
	"os"

	"github.com/gi8lino/warden-webui/internal/app"
	"github.com/gi8lino/warden-webui/web"
)

// Version is injected by the release build.
var Version = "dev"

func main() {
	if err := app.Run(context.Background(), os.Args[1:], web.Assets, Version, os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}
