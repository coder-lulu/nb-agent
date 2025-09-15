//go:build wireinject

package main

import (
	"github.com/google/wire"

	"github.com/coder-lulu/nb-agent/internal/app"
	"github.com/coder-lulu/nb-agent/internal/apps"
	"github.com/coder-lulu/nb-agent/internal/bootstrap"
	"github.com/coder-lulu/nb-agent/internal/data"
	"github.com/coder-lulu/nb-agent/internal/http/middleware"
	"github.com/coder-lulu/nb-agent/internal/job"
	"github.com/coder-lulu/nb-agent/internal/route"
	"github.com/coder-lulu/nb-agent/internal/service"
)

// initWeb init application.
func initWeb() (*app.Web, error) {
	panic(wire.Build(bootstrap.ProviderSet, middleware.ProviderSet, route.ProviderSet, service.ProviderSet, data.ProviderSet, apps.ProviderSet, job.ProviderSet, app.NewWeb))
}
