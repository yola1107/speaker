package biz

import (
	"context"

	v1 "speaker/api/speaker/v1"

	"github.com/yola1107/kratos/v2/errors"
	"github.com/yola1107/kratos/v2/log"
)

var (
	// ErrUserNotFound is user not found.
	ErrUserNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
)

// Greeter is a Greeter model.
type Speaker struct {
	Hello string
}

// GreeterRepo is a Greater repo.
type SpeakerRepo interface {
	Save(context.Context, *Speaker) (*Speaker, error)
	Update(context.Context, *Speaker) (*Speaker, error)
	FindByID(context.Context, int64) (*Speaker, error)
	ListByHello(context.Context, string) ([]*Speaker, error)
	ListAll(context.Context) ([]*Speaker, error)
}

// SpeakerUsecase is a Speaker usecase.
type SpeakerUsecase struct {
	repo SpeakerRepo
	log  *log.Helper
}

// NewSpeakerUsecase new a Speaker usecase.
func NewSpeakerUsecase(repo SpeakerRepo, logger log.Logger) *SpeakerUsecase {
	return &SpeakerUsecase{repo: repo, log: log.NewHelper(logger)}
}

// CreateSpeaker creates a Speaker, and returns the new Speaker.
func (uc *SpeakerUsecase) CreateSpeaker(ctx context.Context, g *Speaker) (*Speaker, error) {
	uc.log.WithContext(ctx).Infof("CreateSpeaker: %v", g.Hello)
	return uc.repo.Save(ctx, g)
}
