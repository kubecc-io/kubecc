module github.com/cobalt77/kube-distcc-operator

go 1.15

require (
	github.com/cenk/backoff v2.2.1+incompatible // indirect
	github.com/containous/flaeg v1.4.1 // indirect
	github.com/containous/mux v0.0.0-20200408164629-f779179d490a // indirect
	github.com/containous/traefik v1.7.26 // indirect
	github.com/go-logr/logr v0.3.0
	github.com/ogier/pflag v0.0.1 // indirect
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/traefik/traefik v1.7.26
	github.com/traefik/traefik/v2 v2.3.6
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	sigs.k8s.io/controller-runtime v0.7.0
)

replace (
	github.com/abbot/go-http-auth => github.com/containous/go-http-auth v0.4.1-0.20200324110947-a37a7636d23e
	github.com/docker/docker => github.com/docker/engine v1.4.2-0.20200204220554-5f6d6f3f2203
	github.com/go-check/check => github.com/containous/check v0.0.0-20170915194414-ca0bf163426a
	github.com/gorilla/mux => github.com/containous/mux v0.0.0-20181024131434-c33f32e26898
	github.com/mailgun/minheap => github.com/containous/minheap v0.0.0-20190809180810-6e71eb837595
	github.com/mailgun/multibuf => github.com/containous/multibuf v0.0.0-20190809014333-8b6c9a7e6bba
)
