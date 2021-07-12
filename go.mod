module github.com/kubecc-io/kubecc

go 1.16

require (
	cloud.google.com/go v0.86.0 // indirect
	github.com/AlecAivazis/survey/v2 v2.2.14
	github.com/Azure/go-autorest/autorest v0.11.19 // indirect
	github.com/HdrHistogram/hdrhistogram-go v1.1.0 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/ajstarks/svgo v0.0.0-20210406150507-75cfd577ce75 // indirect
	github.com/banzaicloud/k8s-objectmatcher v1.5.1
	github.com/cloudflare/golibs v0.0.0-20201113145655-eb7a42c5e0be
	github.com/containerd/console v1.0.2
	github.com/deckarep/golang-set v1.7.1
	github.com/gizak/termui/v3 v3.1.0
	github.com/go-logr/logr v0.4.0
	github.com/godbus/dbus v4.1.0+incompatible // indirect
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.6
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.2.0
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.12
	github.com/karlseguin/ccache/v2 v2.0.8
	github.com/klauspost/cpuid/v2 v2.0.8 // indirect
	github.com/kralicky/grpc-opentracing v0.0.0-20210220041601-edf9159a6710
	github.com/kralicky/ragu v0.1.0
	github.com/magefile/mage v1.11.0
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/minio/md5-simd v1.1.2
	github.com/minio/minio-go/v7 v7.0.12
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/morikuni/aec v1.0.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.14.0
	github.com/opencontainers/runc v1.0.0
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/riywo/loginshell v0.0.0-20200815045211-7d26008be1ab
	github.com/rs/xid v1.3.0 // indirect
	github.com/snapcore/snapd v0.0.0-20210709115903-34ff3cde0a19
	github.com/snapcore/squashfuse v0.0.0-20171220165323-319f6d41a041 // indirect
	github.com/spf13/cobra v1.2.1
	github.com/stretchr/testify v1.7.0
	github.com/uber/jaeger-client-go v2.29.1+incompatible
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	github.com/valyala/bytebufferpool v1.0.0
	go.uber.org/atomic v1.8.0
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.18.1
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97 // indirect
	golang.org/x/exp v0.0.0-20210709195130-ecdcf02a369a // indirect
	golang.org/x/image v0.0.0-20210628002857-a66eb6448b8d // indirect
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e // indirect
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b
	gonum.org/v1/gonum v0.9.3
	gonum.org/v1/plot v0.9.0
	google.golang.org/grpc v1.39.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/tomb.v2 v2.0.0-20161208151619-d5d1b5820637 // indirect
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/kube-openapi v0.0.0-20210527164424-3c818078ee3d // indirect
	k8s.io/kubectl v0.21.2
	k8s.io/system-validators v1.5.0
	sigs.k8s.io/controller-runtime v0.9.2
	sigs.k8s.io/kustomize/kustomize/v4 v4.2.0
	sigs.k8s.io/structured-merge-diff/v4 v4.1.2 // indirect
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc => github.com/kralicky/grpc-opentracing/go/otgrpc v0.0.0-20210220041601-edf9159a6710
