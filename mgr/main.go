package main

import (
	"context"
	"errors"
	"net"

	kdcv1alpha1 "github.com/cobalt77/kube-cc/operator/api/v1alpha1"
	"github.com/cobalt77/kube-cc/types"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	traefikv1alpha1 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefik/v1alpha1"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type mgrServer struct {
	types.MgrServer

	agents map[*types.AgentInfo]struct{}
}

func (s *mgrServer) Schedule(
	ctx context.Context,
	req *types.ScheduleRequest,
) (*types.ScheduleResponse, error) {
	return nil, errors.New("Unimplemented")
}

func (s *mgrServer) Compile(
	ctx context.Context,
	req *types.CompileRequest,
) (*types.CompileResponse, error) {
	return nil, errors.New("Unimplemented")
}

func (s *mgrServer) Connect(
	agentInfo *types.AgentInfo,
	srv types.Mgr_ConnectServer,
) error {
	s.agents[agentInfo] = struct{}{}
	<-srv.Context().Done()
	delete(s.agents, agentInfo)
	return nil
}

var (
	scheme = runtime.NewScheme()
	config *rest.Config
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kdcv1alpha1.AddToScheme(scheme))
	utilruntime.Must(traefikv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// func (s *mgrServer) Status(
// 	ctx context.Context,
// 	req *types.StatusRequest,
// ) (*types.StatusResponse, error) {
// 	log.WithField("cmd", req.Command).Info("=> Handling status request")

// 	cl, err := client.New(config, client.Options{Scheme: scheme})
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	kubeccs := &kdcv1alpha1.kubeccList{}
// 	err = cl.List(ctx, kubeccs, &client.ListOptions{})
// 	if err != nil {
// 		return nil, err
// 	}

// 	if len(kubeccs.Items) == 0 {
// 		log.Warning("No kubeccs found in the cluster")
// 		return &api.StatusResponse{Agents: []*api.Agent{}}, nil
// 	}

// 	response := &api.StatusResponse{
// 		Agents: []*api.Agent{},
// 	}

// 	for _, kubecc := range kubeccs.Items {
// 		svcList := &v1.ServiceList{}
// 		err = cl.List(ctx, svcList, &client.ListOptions{
// 			LabelSelector: labels.SelectorFromSet(labels.Set{
// 				"kkubecc.io/kubecc_cr": kubecc.Name,
// 			}),
// 		})

// 		if err != nil {
// 			log.Error(err)
// 			continue
// 		}

// 		for _, svc := range svcList.Items {
// 			if svc.Spec.Type != v1.ServiceTypeExternalName {
// 				return nil, errors.New(
// 					"The kubecc server appears to be improperly configured (wrong service type)")
// 			}
// 			response.Agents = append(response.Agents, &api.Agent{
// 				Address: svc.Spec.ExternalName,
// 				Port:    3632,
// 			})
// 		}
// 	}

// 	return response, nil
// }

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

	srv := &mgrServer{}
	types.RegisterMgrServer(grpcServer, srv)

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Error(err)
	}
}
