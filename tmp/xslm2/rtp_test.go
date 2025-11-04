package xslm2

import (
	"testing"

	"egame-grpc/global"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func init() {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	logger, _ := cfg.Build()
	global.GVA_LOG = logger
}

// TestRtp RTP测试
func TestRtp(t *testing.T) {
	t.Log("xslm2 基于动态配置生成符号网格，无需预设数据")
	t.Log("可使用 spin.baseSpin() 进行 RTP 压测")

	// 示例：创建一个 RTP 测试
	// svc := newBetOrderService(true) // true 表示 RTP 测试模式
	// svc.spin.baseSpin(false) // false 表示基础模式
	// t.Logf("Step倍数: %d", svc.spin.stepMultiplier)
}
