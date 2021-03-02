package servers

import (
	"context"
	"math"
	"time"

	"github.com/cobalt77/kubecc/internal/zapkc"
	"github.com/cobalt77/kubecc/pkg/meta"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/wait"
)

type ConnectionHandler interface {
	TryConnect() (grpc.ClientStream, error)
	HandleStream(grpc.ClientStream) error
	Target() string
}

type OnConnectedEventHandler interface {
	OnConnected()
}

type OnLostConnectionEventHandler interface {
	OnLostConnection()
}

type OnConnectFailedEventHandler interface {
	OnConnectFailed()
}

type StreamManager struct {
	ctx        context.Context
	handler    ConnectionHandler
	backoffMgr wait.BackoffManager
}

func NewStreamManager(
	ctx context.Context,
	handler ConnectionHandler,
) *StreamManager {
	return &StreamManager{
		ctx:     ctx,
		handler: handler,
		backoffMgr: wait.NewExponentialBackoffManager(
			500*time.Millisecond, // Initial
			8*time.Second,        // Max
			15*time.Second,       // Reset
			math.Sqrt2,           // Backoff factor
			0.25,                 // Jitter factor
			clock.RealClock{},
		),
	}
}

func (cm *StreamManager) Run() {
	lg := meta.Log(cm.ctx)
	for {
		if stream, err := cm.handler.TryConnect(); err != nil {
			if e, ok := cm.handler.(OnConnectFailedEventHandler); ok {
				e.OnConnectFailed()
			}
			lg.With(
				zap.String("err", status.Convert(err).Message()),
				zap.String("target", cm.handler.Target()),
			).Warn(zapkc.Red.Add("Failed to connect"))
			<-cm.backoffMgr.Backoff().C()
		} else {
			if e, ok := cm.handler.(OnConnectedEventHandler); ok {
				e.OnConnected()
			}
			err := cm.handler.HandleStream(stream)
			if err := stream.CloseSend(); err != nil {
				lg.With(zap.Error(err)).Error("Failed to close stream")
			}
			if err != nil {
				lg.With(
					zap.Error(err),
					zap.String("target", cm.handler.Target()),
				).Error(zapkc.Red.Add("Connection lost, Attempting to reconnect"))
			}
			if e, ok := cm.handler.(OnLostConnectionEventHandler); ok {
				e.OnLostConnection()
			}
		}
	}
}
