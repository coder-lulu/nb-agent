//go:build wireinject

package main

import (
	"github.com/google/wire"

	"github.com/coder-lulu/nb-agent/internal/app"
	"github.com/coder-lulu/nb-agent/internal/apps"
	"github.com/coder-lulu/nb-agent/internal/bootstrap"
	"github.com/coder-lulu/nb-agent/internal/data"
	"github.com/coder-lulu/nb-agent/internal/route"
	"github.com/coder-lulu/nb-agent/internal/service"
)

// initCli init command line.
func initCli() (*app.Cli, error) {
	panic(wire.Build(bootstrap.ProviderSet, route.ProviderSet, service.ProviderSet, data.ProviderSet, apps.ProviderSet, app.NewCli))
}
