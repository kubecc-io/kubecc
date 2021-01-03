package main

import (
	"context"
	"errors"
	"io/ioutil"
	"net"
	"strings"

	"github.com/cobalt77/kube-distcc-mgr/api"
	"github.com/prometheus/common/log"
	"google.golang.org/grpc"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type server struct {
	api.ApiServer
}

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
	ns := namespace()
	ds, err := client.AppsV1().DaemonSets(ns).Get("distcc-agent", v1.GetOptions{})
	if err != nil {
		log.Error(err)
		return nil, errors.New("No distcc agents available")
	}

	pods, err := client.CoreV1().Pods(ns).List(v1.ListOptions{
		LabelSelector: "app=distcc-agent",
	})
	if err != nil {
		log.Error(err)
		return nil, errors.New("No distcc agents available")
	}
	// for _, pod := range pods.Items {
	// 	pod.
	// }
}

func main() {
	config, err := rest.InClusterConfig()
	// kubernetes.
	if err != nil {
		panic(err.Error())
	}

	client := kubernetes.NewForConfigOrDie(config)

	listener, err := net.Listen("tcp", ":9090")
	if err != nil {
		panic(err.Error())
	}

	grpcServer := grpc.NewServer()

	srv := &server{}
	api.RegisterApiServer(grpcServer, srv)

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Error(err)
	}
}
