module github.com/kubecc-io/kubecc

go 1.16

require (
	cloud.google.com/go v0.80.0 // indirect
	github.com/AlecAivazis/survey/v2 v2.2.9
	github.com/Azure/go-autorest/autorest v0.11.18 // indirect
	github.com/HdrHistogram/hdrhistogram-go v1.1.0 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/banzaicloud/k8s-objectmatcher v1.5.1
	github.com/cloudflare/golibs v0.0.0-20201113145655-eb7a42c5e0be
	github.com/cobalt77/grpc-opentracing v0.0.0-20210220041601-edf9159a6710
	github.com/deckarep/golang-set v1.7.1
	github.com/gizak/termui/v3 v3.1.0
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr v0.4.0 // indirect
	github.com/godbus/dbus v4.1.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/mock v1.5.0
	github.com/golang/protobuf v1.5.1
	github.com/google/go-cmp v0.5.5
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.2.0
	github.com/googleapis/gnostic v0.5.4 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.12
	github.com/karlseguin/ccache/v2 v2.0.8
	github.com/mattn/go-runewidth v0.0.10 // indirect
	github.com/minio/md5-simd v1.1.2
	github.com/minio/minio-go/v7 v7.0.10
	github.com/mitchellh/copystructure v1.1.1 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/ginkgo v1.15.2
	github.com/onsi/gomega v1.11.0
	github.com/opencontainers/runc v1.0.0-rc93.0.20210403014141-0d4947039220
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.10.0
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/riywo/loginshell v0.0.0-20200815045211-7d26008be1ab
	github.com/snapcore/snapd v0.0.0-20210409180151-3325f4e459f2
	github.com/snapcore/squashfuse v0.0.0-20171220165323-319f6d41a041 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/stretchr/testify v1.7.0
	github.com/uber/jaeger-client-go v2.25.0+incompatible
	github.com/uber/jaeger-lib v2.4.0+incompatible // indirect
	github.com/valyala/bytebufferpool v1.0.0
	go.uber.org/atomic v1.7.0
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2 // indirect
	golang.org/x/exp v0.0.0-20210220032938-85be41e4509f // indirect
	golang.org/x/mod v0.4.2 // indirect
	golang.org/x/net v0.0.0-20210326220855-61e056675ecf // indirect
	golang.org/x/sys v0.0.0-20210415045647-66c3f260301c // indirect
	golang.org/x/term v0.0.0-20210317153231-de623e64d2a6
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba // indirect
	golang.org/x/tools v0.1.1-0.20210408155258-593413344da1 // indirect
	gonum.org/v1/gonum v0.9.0
	gonum.org/v1/plot v0.9.0
	google.golang.org/grpc v1.36.1
	google.golang.org/protobuf v1.26.0
	gopkg.in/tomb.v2 v2.0.0-20161208151619-d5d1b5820637 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	honnef.co/go/tools v0.1.3 // indirect
	k8s.io/api v0.20.5
	k8s.io/apiextensions-apiserver v0.20.5 // indirect
	k8s.io/apimachinery v0.20.5
	k8s.io/client-go v0.20.5
	k8s.io/klog/v2 v2.8.0 // indirect
	k8s.io/kube-openapi v0.0.0-20210323165736-1a6458611d18 // indirect
	k8s.io/kubectl v0.20.5
	k8s.io/system-validators v1.4.0 // indirect
	sigs.k8s.io/controller-runtime v0.8.3
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc => github.com/cobalt77/grpc-opentracing/go/otgrpc v0.0.0-20210220041601-edf9159a6710
