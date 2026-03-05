package server

import (
	v1 "speaker/api/speaker/v1"
	"speaker/internal/conf"
	"speaker/internal/service"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware/recovery"
	"github.com/yola1107/kratos/v2/transport/gnet"
)

// NewGNETServer new a GNET server.
func NewGNETServer(c *conf.Server, greeter *service.SpeakerService, logger log.Logger) *gnet.Server {
	var opts = []gnet.ServerOption{
		gnet.Middleware(
			recovery.Recovery(),
		),
	}
	if c.Gnet.Network != "" {
		opts = append(opts, gnet.Network(c.Gnet.Network))
	}
	if c.Gnet.Addr != "" {
		opts = append(opts, gnet.Address(c.Gnet.Addr))
	}
	if c.Gnet.Timeout != nil {
		opts = append(opts, gnet.Timeout(c.Gnet.Timeout.AsDuration()))
	}
	srv := gnet.NewServer(opts...)
	v1.RegisterSpeakerGNETServer(srv, greeter)
	return srv
}
