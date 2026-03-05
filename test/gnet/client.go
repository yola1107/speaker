package main

import (
	"context"
	"fmt"
	"time"

	v1 "speaker/api/speaker/v1"

	"github.com/yola1107/kratos/contrib/log/zap/v2"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/gnet"
)

func main() {
	logger := zap.New(nil)
	defer logger.Close()
	log.SetLogger(logger)

	log.Info("start gnet echo client")

	// Create gnet client
	client := gnet.NewClient(
		gnet.WithEndpoint("127.0.0.1:3103"),
		gnet.WithTimeout(5*time.Second),
	)

	// Test SayHelloReq (Ops: 1001)
	log.Info("=== Testing SayHelloReq ===")
	for i := 0; i < 10; i++ {
		req := &v1.HelloRequest{
			Name: fmt.Sprintf("Hello from gnet client %d", i),
		}

		var reply v1.HelloReply
		err := client.Invoke(context.Background(), 1001, req, &reply)
		if err != nil {
			log.Errorf("SayHelloReq failed: %v", err)
			continue
		}

		log.Infof("[gnet] SayHelloReq response: %s", reply.Message)
		time.Sleep(time.Second)
	}

	// Test SayHello2Req (Ops: 1003)
	log.Info("=== Testing SayHello2Req ===")
	for i := 0; i < 65536; i++ {
		req := &v1.Hello2Request{
			Name: fmt.Sprintf("Hello2 from gnet client %d", i),
		}

		var reply v1.Hello2Reply
		err := client.Invoke(context.Background(), 1003, req, &reply)
		if err != nil {
			log.Errorf("SayHello2Req failed: %v", err)
			continue
		}

		log.Infof("[gnet] SayHello2Req response: %s", reply.Message)
		time.Sleep(time.Second)
		i = i % 65536
	}

	log.Info("gnet echo client finished")
}
