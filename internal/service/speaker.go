package service

import (
	"context"

	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/transport/websocket"

	v1 "speaker/api/speaker/v1"
	"speaker/internal/biz"

	"github.com/yola1107/kratos/v2/transport/tcp"
)

// SpeakerService is a greeter service.
type SpeakerService struct {
	v1.UnimplementedSpeakerServer

	uc *biz.SpeakerUsecase

	loop work.Loop
}

// NewSpeakerService new a greeter service.
func NewSpeakerService(uc *biz.SpeakerUsecase) *SpeakerService {
	loop := work.NewLoop()
	_ = loop.Start()
	return &SpeakerService{uc: uc, loop: loop}
}

// SayHelloReq implements helloworld.SpeakerServer.
func (s *SpeakerService) SayHelloReq(ctx context.Context, in *v1.HelloRequest) (*v1.HelloReply, error) {
	g, err := s.uc.CreateSpeaker(ctx, &biz.Speaker{Hello: in.Name})
	if err != nil {
		return nil, err
	}
	return &v1.HelloReply{Message: "Hello " + g.Hello}, nil
}

// SayHello2Req implements helloworld.SpeakerServer.
func (s *SpeakerService) SayHello2Req(ctx context.Context, in *v1.Hello2Request) (*v1.Hello2Reply, error) {
	g, err := s.uc.CreateSpeaker(ctx, &biz.Speaker{Hello: in.Name})
	if err != nil {
		return nil, err
	}
	return &v1.Hello2Reply{Message: "Hello " + g.Hello}, nil
}

func (s *SpeakerService) SetCometChan(cl *tcp.ChanList, cs *tcp.Server) {}
func (s *SpeakerService) GetTCPLoop() work.Loop                         { return s.loop }
func (s *SpeakerService) GetLoop() work.Loop                            { return s.loop }
func (s *SpeakerService) OnSessionOpen(*websocket.Session)              {}
func (s *SpeakerService) OnSessionClose(*websocket.Session)             {}
