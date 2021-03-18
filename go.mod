module github.com/cobalt77/kubecc

go 1.16

require (
	cloud.google.com/go v0.78.0 // indirect
	github.com/Azure/go-autorest/autorest v0.11.18 // indirect
	github.com/HdrHistogram/hdrhistogram-go v1.0.1 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/banzaicloud/k8s-objectmatcher v1.5.1
	github.com/cobalt77/grpc-opentracing v0.0.0-20210220041601-edf9159a6710
	github.com/deckarep/golang-set v1.7.1
	github.com/gizak/termui/v3 v3.1.0
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr v0.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/gojp/goreportcard v0.0.0-20210127091035-7a8a1ebca86e // indirect
	github.com/golang/protobuf v1.4.3
	github.com/google/go-cmp v0.5.5
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.2.0
	github.com/googleapis/gnostic v0.5.4 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/karlseguin/ccache/v2 v2.0.8
	github.com/mattn/go-runewidth v0.0.10 // indirect
	github.com/minio/md5-simd v1.1.2
	github.com/minio/minio-go/v7 v7.0.10
	github.com/mitchellh/copystructure v1.1.1 // indirect
	github.com/onsi/ginkgo v1.15.2
	github.com/onsi/gomega v1.11.0
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.9.0
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/smallnest/weighted v0.0.0-20201102054551-85ac5c79528c
	github.com/spf13/cobra v1.1.3
	github.com/stretchr/testify v1.7.0
	github.com/tinylib/msgp v1.1.5
	github.com/uber/jaeger-client-go v2.25.0+incompatible
	github.com/uber/jaeger-lib v2.4.0+incompatible // indirect
	github.com/valyala/bytebufferpool v1.0.0
	go.uber.org/atomic v1.7.0
	go.uber.org/goleak v1.1.10 // indirect
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83 // indirect
	golang.org/x/exp v0.0.0-20200331195152-e8c3332aa8e5
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110 // indirect
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba // indirect
	google.golang.org/grpc v1.36.0
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/api v0.20.4
	k8s.io/apiextensions-apiserver v0.20.4 // indirect
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v0.20.4
	k8s.io/klog/v2 v2.6.0 // indirect
	k8s.io/kube-openapi v0.0.0-20210305164622-f622666832c1 // indirect
	sigs.k8s.io/controller-runtime v0.8.3
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc => github.com/cobalt77/grpc-opentracing/go/otgrpc v0.0.0-20210220041601-edf9159a6710
