package main

import (
	"context"
	"errors"
	"net"

	"github.com/cobalt77/kubecc/api/v1alpha1"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	traefikv1alpha1 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefik/v1alpha1"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type schedulerServer struct {
	types.SchedulerServer

	agents map[*types.AgentInfo]struct{}
}

func (s *schedulerServer) Schedule(
	ctx context.Context,
	req *types.ScheduleRequest,
) (*types.ScheduleResponse, error) {
	return nil, errors.New("Unimplemented")
}

func (s *schedulerServer) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	return nil, errors.New("Unimplemented")
}

func (s *schedulerServer) Connect(
	srv types.Scheduler_ConnectServer,
) error {
	log.Info("Agent connecting...")
	agentInfo, err := srv.Recv()
	if err != nil {
		return err
	}
	lg := log.WithFields(log.Fields{
		"name": agentInfo.Hostname,
		"arch": agentInfo.Arch,
		"cpus": agentInfo.NumCpus,
	})
	lg.Info("Agent connected")
	s.agents[agentInfo] = struct{}{}
	<-srv.Context().Done()
	delete(s.agents, agentInfo)
	lg.Info("Agent disconnected")
	return nil
}

var (
	scheme = runtime.NewScheme()
	config *rest.Config
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(traefikv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	log.SetLevel(logrus.DebugLevel)
	log.Info("Server starting")

	cfg, err := rest.InClusterConfig()

	if err != nil {
		panic(err.Error())
	}

	config = cfg

	listener, err := net.Listen("tcp", ":9090")
	if err != nil {
		panic(err.Error())
	}
	log.Infof("Server listening on %s", listener.Addr().String())

	grpcServer := grpc.NewServer()

	srv := &schedulerServer{
		agents: make(map[*types.AgentInfo]struct{}),
	}
	types.RegisterSchedulerServer(grpcServer, srv)

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Error(err)
	}
}
