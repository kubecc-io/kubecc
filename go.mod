module github.com/kubecc-io/kubecc

go 1.16

require (
	cloud.google.com/go v0.82.0 // indirect
	github.com/AlecAivazis/survey/v2 v2.2.12
	github.com/Azure/go-autorest/autorest v0.11.18 // indirect
	github.com/HdrHistogram/hdrhistogram-go v1.1.0 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/ajstarks/svgo v0.0.0-20210406150507-75cfd577ce75 // indirect
	github.com/andreyvit/diff v0.0.0-20170406064948-c7f18ee00883 // indirect
	github.com/banzaicloud/k8s-objectmatcher v1.5.1
	github.com/cloudflare/golibs v0.0.0-20201113145655-eb7a42c5e0be
	github.com/containerd/console v1.0.2
	github.com/coreos/go-systemd/v22 v22.3.2 // indirect
	github.com/deckarep/golang-set v1.7.1
	github.com/gizak/termui/v3 v3.1.0
	github.com/go-logr/logr v0.4.0
	github.com/godbus/dbus v4.1.0+incompatible // indirect
	github.com/golang/mock v1.5.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.5
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.2.0
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.12
	github.com/json-iterator/go v1.1.11 // indirect
	github.com/karlseguin/ccache/v2 v2.0.8
	github.com/klauspost/cpuid/v2 v2.0.6 // indirect
	github.com/kralicky/grpc-opentracing v0.0.0-20210220041601-edf9159a6710
	github.com/mattn/go-runewidth v0.0.12 // indirect
	github.com/minio/md5-simd v1.1.2
	github.com/minio/minio-go/v7 v7.0.10
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/morikuni/aec v1.0.0
	github.com/onsi/ginkgo v1.16.2
	github.com/onsi/gomega v1.12.0
	github.com/opencontainers/runc v1.0.0-rc95
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.10.0
	github.com/prometheus/common v0.25.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/riywo/loginshell v0.0.0-20200815045211-7d26008be1ab
	github.com/rs/xid v1.3.0 // indirect
	github.com/snapcore/snapd v0.0.0-20210521224045-e0b63e07bc9b
	github.com/snapcore/squashfuse v0.0.0-20171220165323-319f6d41a041 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/stretchr/testify v1.7.0
	github.com/uber/jaeger-client-go v2.29.0+incompatible
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	github.com/valyala/bytebufferpool v1.0.0
	go.uber.org/atomic v1.7.0
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a // indirect
	golang.org/x/exp v0.0.0-20210514180818-737f94c0881e // indirect
	golang.org/x/image v0.0.0-20210504121937-7319ad40d33e // indirect
	golang.org/x/net v0.0.0-20210521195947-fe42d452be8f // indirect
	golang.org/x/sys v0.0.0-20210521203332-0cec03c779c1 // indirect
	golang.org/x/term v0.0.0-20210503060354-a79de5458b56
	gonum.org/v1/gonum v0.9.3
	gonum.org/v1/plot v0.9.0
	google.golang.org/genproto v0.0.0-20210521181308-5ccab8a35a9a // indirect
	google.golang.org/grpc v1.38.0
	google.golang.org/protobuf v1.26.0
	gopkg.in/ini.v1 v1.62.0 // indirect
	gopkg.in/tomb.v2 v2.0.0-20161208151619-d5d1b5820637 // indirect
	honnef.co/go/tools v0.1.4 // indirect
	k8s.io/api v0.21.1
	k8s.io/apiextensions-apiserver v0.21.1 // indirect
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/kube-openapi v0.0.0-20210421082810-95288971da7e // indirect
	k8s.io/kubectl v0.21.1
	k8s.io/system-validators v1.4.0
	sigs.k8s.io/controller-runtime v0.9.0-beta.5
	sigs.k8s.io/structured-merge-diff/v4 v4.1.1 // indirect
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc => github.com/kralicky/grpc-opentracing/go/otgrpc v0.0.0-20210220041601-edf9159a6710
