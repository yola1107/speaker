package data

import (
	"context"

	"speaker/internal/biz"

	"github.com/yola1107/kratos/v2/log"
)

type speakerRepo struct {
	data *Data
	log  *log.Helper
}

// NewSpeakerRepo .
func NewSpeakerRepo(data *Data, logger log.Logger) biz.SpeakerRepo {
	return &speakerRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *speakerRepo) Save(ctx context.Context, g *biz.Speaker) (*biz.Speaker, error) {
	return g, nil
}

func (r *speakerRepo) Update(ctx context.Context, g *biz.Speaker) (*biz.Speaker, error) {
	return g, nil
}

func (r *speakerRepo) FindByID(context.Context, int64) (*biz.Speaker, error) {
	return nil, nil
}

func (r *speakerRepo) ListByHello(context.Context, string) ([]*biz.Speaker, error) {
	return nil, nil
}

func (r *speakerRepo) ListAll(context.Context) ([]*biz.Speaker, error) {
	return nil, nil
}
