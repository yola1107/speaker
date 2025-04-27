package main

import (
	"context"
	"fmt"
	"time"

	v1 "speaker/api/speaker/v1"

	"github.com/yola1107/kratos/contrib/log/zap/v2"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket"
)

var (
	seed = int64(0)
)

func main() {

	zapLogger := zap.New(nil)
	defer zapLogger.Close()

	log.SetLogger(zapLogger)

	log.Infof("start websocket client")
	defer log.Infof("close websocket client")

	wsClient, err := websocket.NewClient(context.Background(),
		websocket.WithEndpoint("ws://0.0.0.0:3102"),
		websocket.WithToken(""),
		websocket.WithPushHandler(map[int32]websocket.PushHandler{
			int32(v1.GameCommand_SayHelloRsp):  func(data []byte) { log.Infof("ws-> 1002 cb. %v", data) },
			int32(v1.GameCommand_SayHello2Rsp): func(data []byte) { log.Infof("ws-> 1003 cb. %v", data) },
		}),
		websocket.WithResponseHandler(map[int32]websocket.ResponseHandler{
			int32(v1.GameCommand_SayHelloReq):  func(data []byte, code int32) { log.Infof("ws-> 1001. data=%+v code=%d", data, code) },
			int32(v1.GameCommand_SayHello2Req): func(data []byte, code int32) { log.Infof("ws-> 1003. data=%+v code=%d", data, code) },
			int32(6666):                        func(data []byte, code int32) { log.Infof("ws-> 6666. data=%+v code=%d", data, code) },
			int32(9999):                        func(data []byte, code int32) { log.Infof("ws-> 9999. data=%+v code=%d", data, code) },
		}),
		websocket.WithDisconnectFunc(func() { log.Infof("disconnect called") }),
	)
	if err != nil {
		panic(err)
	}
	defer wsClient.Close()

	for {
		seed++
		callWebsocket(wsClient)
		time.Sleep(time.Second * 10)
	}
}
func callWebsocket(c *websocket.Client) {
	if _, err := c.Request(int32(v1.GameCommand_SayHello2Req), &v1.Hello2Request{Name: fmt.Sprintf("ws:%d", seed)}); err != nil {
		log.Fatal("%+v", err)
	}
	if _, err := c.Request(6666, &v1.Hello2Request{Name: fmt.Sprintf("ws:%d", seed)}); err != nil {
		log.Errorf("%+v", err)
	}
	if _, err := c.Request(9999, &v1.HelloRequest{Name: fmt.Sprintf("ws:%d", seed)}); err != nil {
		log.Errorf("%+v", err)
	}
}
