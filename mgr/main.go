package main

import (
	"context"
	"errors"
	"net"

	"github.com/cobalt77/kube-distcc/mgr/api"
	kdcv1alpha1 "github.com/cobalt77/kube-distcc/operator/api/v1alpha1"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	traefikv1alpha1 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefik/v1alpha1"
	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type server struct {
	api.ApiServer
}

var (
	gvr = schema.GroupVersionResource{
		Group:    "kdistcc.io",
		Version:  "v1",
		Resource: "distccs",
	}
)

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

func (s *server) Status(ctx context.Context, req *api.StatusRequest) (*api.StatusResponse, error) {
	log.WithField("cmd", req.Command).Info("=> Handling status request")

	cl, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatal(err)
	}

	distccs := &kdcv1alpha1.DistccList{}
	err = cl.List(ctx, distccs, &client.ListOptions{})
	if err != nil {
		return nil, err
	}

	if len(distccs.Items) == 0 {
		log.Warning("No distccs found in the cluster")
		return &api.StatusResponse{Agents: []*api.Agent{}}, nil
	}

	response := &api.StatusResponse{
		Agents: []*api.Agent{},
	}

	for _, distcc := range distccs.Items {
		svcList := &v1.ServiceList{}
		err = cl.List(ctx, svcList, &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{
				"kdistcc.io/distcc_cr": distcc.Name,
			}),
		})

		if err != nil {
			log.Error(err)
			continue
		}

		for _, svc := range svcList.Items {
			if svc.Spec.Type != v1.ServiceTypeExternalName {
				return nil, errors.New(
					"The distcc server appears to be improperly configured (wrong service type)")
			}
			response.Agents = append(response.Agents, &api.Agent{
				Address: svc.Spec.ExternalName,
				Port:    3632,
			})
		}
	}

	return response, nil
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

	srv := &server{}
	api.RegisterApiServer(grpcServer, srv)

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Error(err)
	}
}
