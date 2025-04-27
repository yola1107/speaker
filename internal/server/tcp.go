package server

import (
	v1 "speaker/api/speaker/v1"
	"speaker/internal/conf"
	"speaker/internal/service"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/middleware/recovery"
	"github.com/yola1107/kratos/v2/transport/tcp"
)

// NewTCPServer new an TCP server.
func NewTCPServer(c *conf.Server, greeter *service.SpeakerService, logger log.Logger) *tcp.Server {
	var opts = []tcp.ServerOption{
		tcp.Middleware(
			recovery.Recovery(),
		),
	}
	if c.Tcp.Network != "" {
		opts = append(opts, tcp.Network(c.Tcp.Network))
	}
	if c.Tcp.Addr != "" {
		opts = append(opts, tcp.Address(c.Tcp.Addr))
	}
	if c.Tcp.Timeout != nil {
		opts = append(opts, tcp.Timeout(c.Tcp.Timeout.AsDuration()))
	}
	srv := tcp.NewServer(opts...)
	v1.RegisterSpeakerTCPServer(srv, greeter)
	return srv
}
