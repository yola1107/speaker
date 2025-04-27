//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"speaker/internal/biz"
	"speaker/internal/conf"
	"speaker/internal/data"
	"speaker/internal/server"
	"speaker/internal/service"

	"github.com/google/wire"
	"github.com/yola1107/kratos/v2"
	"github.com/yola1107/kratos/v2/log"
)

// wireApp init kratos application.
func wireApp(*conf.Server, *conf.Data, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, service.ProviderSet, newApp))
}
