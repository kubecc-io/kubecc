package scheduler

import (
	"context"
	"sync"

	"github.com/cobalt77/kubecc/pkg/cluster"
	"github.com/cobalt77/kubecc/pkg/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Monitor struct {
	AgentResolver

	agents sync.Map // map[types.AgentID]*Agent
}

func NewMonitor() *Monitor {
	return &Monitor{
		agents: sync.Map{},
	}
}

func (w *Monitor) AgentIsConnected(id types.AgentID) bool {
	_, ok := w.agents.Load(id)
	return ok
}

func (w *Monitor) AgentConnected(a *Agent) error {
	id, err := a.Info.AgentID()
	if err != nil {
		return err
	}
	w.agents.Store(id, a)
	go func() {
		<-a.Context.Done()
		w.agents.Delete(id)
	}()
	return nil
}

func (w *Monitor) GetAgentInfo(ctx context.Context) (*types.AgentInfo, error) {
	info, err := cluster.AgentInfoFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, err := info.AgentID()
	if err != nil {
		return nil, err
	}
	if !w.AgentIsConnected(id) {
		return nil, status.Error(codes.FailedPrecondition,
			"Not connected, ensure a connection stream is active with Connect()")
	}
	return info, nil
}

// func (mon *Monitor) Wait(task *CompileTask) (*types.CompileResponse, error) {
// 	logkc.Info("=> Watching task")
// 	task.Start()
// 	for {
// 		select {
// 		case s := <-task.Status():
// 			switch s.CompileStatus {
// 			case types.CompileStatus_Accept:
// 				info := s.GetInfo()
// 				id, err := info.AgentID()
// 				if err != nil {
// 					return nil, err
// 				}
// 				logkc.With("agent", info.Pod).Info("=> Agent accepted task")
// 				if agent, ok := mon.agents.Load(id); ok {
// 					agent.(*Agent).ActiveTasks.Inc()
// 				}
// 				defer func() {
// 					if agent, ok := mon.agents.Load(id); ok {
// 						agent.(*Agent).ActiveTasks.Dec()
// 					}
// 				}()
// 			case types.CompileStatus_Reject:
// 				logkc.Info("=> Agent rejected task")
// 				return nil, status.Error(codes.Aborted, "Agent rejected task")
// 			case types.CompileStatus_Success:
// 				logkc.Info("=> Compile completed successfully")
// 				return &types.CompileResponse{
// 					CompileResult: types.CompileResponse_Success,
// 					Data: &types.CompileResponse_CompiledSource{
// 						CompiledSource: s.GetCompiledSource(),
// 					},
// 				}, nil
// 			case types.CompileStatus_Fail:
// 				logkc.Info("=> Compile failed")
// 				return &types.CompileResponse{
// 					CompileResult: types.CompileResponse_Fail,
// 					Data: &types.CompileResponse_Error{
// 						Error: s.GetError(),
// 					},
// 				}, nil
// 			}
// 		case err := <-task.Error():
// 			logkc.With("err", err).Info("=> Error")
// 			return nil, err
// 		case <-task.Canceled():
// 			logkc.Info("=> Task canceled by the consumer")
// 			return nil, status.Error(codes.Canceled, "Consumer canceled the request")
// 		}
// 	}
// }

// func (r *Monitor) Dial() (*grpc.ClientConn, error) {
// 	agents := []types.AgentID{}
// 	r.agents.Range(func(key, value interface{}) bool {
// 		agents = append(agents, key.(types.AgentID))
// 		return true
// 	})
// 	if len(agents) == 0 {
// 		return nil, errors.New("No agents available")
// 	}
// 	agent, ok := r.agents.Load(agents[rand.Intn(len(agents))])
// 	if !ok {
// 		return nil, errors.New("Agent disappeared")
// 	}
// 	peer, ok := peer.FromContext(agent.(*Agent).Context)
// 	if !ok {
// 		return nil, errors.New("Peer unavailable from agent context")
// 	}
// 	return grpc.Dial(fmt.Sprintf("%s:9090", peer.Addr.(*net.TCPAddr).IP.String()), grpc.WithInsecure())
// }
