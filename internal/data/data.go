package data

import (
	"speaker/internal/conf"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	kredis "github.com/yola1107/kratos/v2/library/db/redis"
	kxorm "github.com/yola1107/kratos/v2/library/db/xorm"
	"github.com/yola1107/kratos/v2/library/mq/rabbitmq"
	"github.com/yola1107/kratos/v2/log"
	"xorm.io/xorm"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewRedis, NewMysql, NewRabbitMQ, NewSpeakerRepo)

// Data .
type Data struct {
	db  *xorm.Engine
	rdb redis.UniversalClient
	pub *rabbitmq.Publisher
}

// NewData .
func NewData(c *conf.Data, logger log.Logger, db *xorm.Engine, rdb redis.UniversalClient, pub *rabbitmq.Publisher) (*Data, func(), error) {
	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
	}
	return &Data{
		db:  db,
		rdb: rdb,
		pub: pub,
	}, cleanup, nil
}

func NewRedis(c *conf.Data, logger log.Logger) redis.UniversalClient {
	return kredis.NewClient(kredis.WithAddress(c.Redis.Addr))
}

func NewMysql(c *conf.Data, logger log.Logger) (*xorm.Engine, func(), error) {
	engine, err := kxorm.NewEngine(
		kxorm.WithDriver(c.Database.Driver),
		kxorm.WithDataSource(c.Database.Source),
	)
	if err != nil {
		return nil, nil, err
	}
	return engine, func() { engine.Close() }, nil
}

func NewRabbitMQ(c *conf.Data, logger log.Logger) (*rabbitmq.Publisher, func(), error) {
	pub, err := rabbitmq.NewPublisher(rabbitmq.Options{
		Host:     c.Rabbitmq.Host,
		Port:     c.Rabbitmq.Port,
		Username: c.Rabbitmq.Username,
		Password: c.Rabbitmq.Password,
		VHost:    c.Rabbitmq.Vhost,
	}, rabbitmq.PublisherOptions{})
	if err != nil {
		return nil, nil, err
	}
	return pub, func() {}, nil
}
