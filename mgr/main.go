package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"strings"

	"github.com/cobalt77/kube-distcc/mgr/api"
	kdcv1alpha1 "github.com/cobalt77/kube-distcc/operator/api/v1alpha1"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

func namespace() string {
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	return "default"
}

var client *kubernetes.Clientset

func (s *server) Status(ctx context.Context, _ *api.StatusRequest) (*api.StatusResponse, error) {
	log.Info("=> Handling status request")

	ns := namespace()

	distccs := &kdcv1alpha1.DistccList{}
	err := client.
		RESTClient().
		Get().
		Resource("distccs"). // todo: this doesnt work
		Do(ctx).
		Into(distccs)

	if err != nil {
		return nil, err
	}

	distcc := distccs.Items[0]

	svcList, err := client.CoreV1().Services(ns).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("kdistcc.io/distcc_cr=%s", distcc.Name),
	})

	if err != nil {
		return nil, err
	}

	response := &api.StatusResponse{
		Agents: []*api.Agent{},
	}

	for _, svc := range svcList.Items {
		if svc.Spec.Type != v1.ServiceTypeExternalName {
			return nil, errors.New(
				"The distcc server appears to be improperly configured (wrong service type)")
		}
		response.Agents = append(response.Agents, &api.Agent{
			Address: svc.Spec.ExternalName,
			Port:    443,
		})
	}

	return response, nil
}

func main() {
	log.SetLevel(logrus.DebugLevel)
	log.Info("Server starting")
	config, err := rest.InClusterConfig()

	if err != nil {
		panic(err.Error())
	}

	client = kubernetes.NewForConfigOrDie(config)

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
