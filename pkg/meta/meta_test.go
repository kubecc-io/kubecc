/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package meta_test

import (
	"context"
	"net"

	"github.com/google/uuid"
	"github.com/kubecc-io/kubecc/internal/logkc"
	"github.com/kubecc-io/kubecc/pkg/host"
	"github.com/kubecc-io/kubecc/pkg/identity"
	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/meta/mdkeys"
	"github.com/kubecc-io/kubecc/pkg/test"
	"github.com/kubecc-io/kubecc/pkg/tracing"
	"github.com/kubecc-io/kubecc/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

type fooServer struct {
	test.FooServer
}

func (s *fooServer) Foo(
	ctx context.Context,
	in *test.Baz,
) (*test.Baz, error) {
	defer GinkgoRecover()
	Expect(ctx).NotTo(BeNil())
	Expect(meta.Component(ctx)).To(Equal(types.TestComponent))
	Expect(func() { uuid.MustParse(meta.UUID(ctx)) }).NotTo(Panic())
	Expect(meta.Log(ctx)).NotTo(BeNil())
	Expect(meta.Tracer(ctx)).NotTo(BeNil())
	Expect(meta.SystemInfo(ctx)).NotTo(BeNil())
	return &test.Baz{}, nil
}

type barServer struct {
	test.BarServer
}

func (s *barServer) Bar(
	srv test.Bar_BarServer,
) error {
	defer GinkgoRecover()
	ctx := srv.Context()
	Expect(ctx).NotTo(BeNil())
	Expect(meta.Component(ctx)).To(Equal(types.TestComponent))
	Expect(func() { uuid.MustParse(meta.UUID(ctx)) }).NotTo(Panic())
	Expect(meta.Log(ctx)).NotTo(BeNil())
	Expect(meta.Tracer(ctx)).NotTo(BeNil())
	Expect(meta.SystemInfo(ctx)).NotTo(BeNil())
	return nil
}

var _ = Describe("Meta", func() {
	When("Creating a context with all providers", func() {
		var ctx context.Context
		It("Should succeed", func() {
			Expect(func() {
				ctx = meta.NewContext(
					meta.WithProvider(identity.Component,
						meta.WithValue(types.TestComponent)),
					meta.WithProvider(identity.UUID),
					meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.TestComponent,
						logkc.WithLogLevel(zapcore.ErrorLevel),
					))),
					meta.WithProvider(tracing.Tracer),
					meta.WithProvider(host.SystemInfo),
				)
			}).ShouldNot(Panic())
			Expect(ctx).NotTo(BeNil())
		})
		It("Should contain values for each provider", func() {
			Expect(meta.Component(ctx)).To(Equal(types.TestComponent))
			Expect(func() { uuid.MustParse(meta.UUID(ctx)) }).NotTo(Panic())
			Expect(meta.Log(ctx)).NotTo(BeNil())
			Expect(meta.Tracer(ctx)).NotTo(BeNil())
		})
	})
	When("Creating a context with some providers", func() {
		var ctx context.Context
		It("Should succeed", func() {
			Expect(func() {
				ctx = meta.NewContext(
					meta.WithProvider(identity.Component,
						meta.WithValue(types.TestComponent)),
					meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.TestComponent,
						logkc.WithLogLevel(zapcore.ErrorLevel),
					))),
				)
			}).ShouldNot(Panic())
			Expect(ctx).NotTo(BeNil())
		})
		It("Should contain values for the given providers", func() {
			Expect(meta.Component(ctx)).To(Equal(types.TestComponent))
			Expect(meta.Log(ctx)).NotTo(BeNil())
			By("Causing a panic when querying nonexistent values")
			Expect(func() { meta.UUID(ctx) }).To(Panic())
			Expect(func() { meta.Tracer(ctx) }).To(Panic())
			Expect(func() { meta.SystemInfo(ctx) }).To(Panic())
		})
	})
	When("Creating contexts using meta.Context as a parent", func() {
		It("Should allow adding new values", func() {
			type testKeyType struct{}
			testKey := testKeyType{}
			ctx := meta.NewContext(
				meta.WithProvider(identity.Component,
					meta.WithValue(types.TestComponent)),
				meta.WithProvider(identity.UUID),
				meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.TestComponent,
					logkc.WithLogLevel(zapcore.ErrorLevel),
				))),
				meta.WithProvider(tracing.Tracer),
				meta.WithProvider(host.SystemInfo),
			)
			ctx2 := context.WithValue(ctx, testKey, "testValue")
			Expect(ctx2.Value(testKey)).To(Equal("testValue"))
			Expect(ctx2.Value(mdkeys.ComponentKey)).To(Equal(meta.Component(ctx)))
			Expect(ctx2.Value(mdkeys.UUIDKey)).To(Equal(meta.UUID(ctx)))
			Expect(ctx2.Value(mdkeys.LogKey)).To(Equal(meta.Log(ctx)))
			Expect(ctx2.Value(mdkeys.TracingKey)).To(Equal(meta.Tracer(ctx)))
			Expect(ctx2.Value(mdkeys.SystemInfoKey)).To(Equal(meta.SystemInfo(ctx)))
		})
		It("Should allow overriding values", func() {
			ctx := meta.NewContext(
				meta.WithProvider(identity.Component,
					meta.WithValue(types.TestComponent)),
				meta.WithProvider(identity.UUID),
				meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.TestComponent,
					logkc.WithLogLevel(zapcore.ErrorLevel),
				))),
				meta.WithProvider(tracing.Tracer),
				meta.WithProvider(host.SystemInfo),
			)
			newLog := meta.Log(ctx).Named("test")
			ctx2 := context.WithValue(ctx, mdkeys.LogKey, newLog)
			Expect(ctx2.Value(mdkeys.ComponentKey)).To(Equal(meta.Component(ctx)))
			Expect(ctx2.Value(mdkeys.UUIDKey)).To(Equal(meta.UUID(ctx)))
			Expect(ctx2.Value(mdkeys.LogKey)).To(Equal(newLog))
			Expect(ctx2.Value(mdkeys.TracingKey)).To(Equal(meta.Tracer(ctx)))
			Expect(ctx2.Value(mdkeys.SystemInfoKey)).To(Equal(meta.SystemInfo(ctx)))
		})
		It("Should cancel properly", func() {
			ctx, ca := context.WithCancel(context.Background())
			ctx2 := meta.NewContextWithParent(ctx)
			ca()
			Eventually(ctx2.Done()).Should(BeClosed())
			Eventually(ctx2.Err()).Should(MatchError(context.Canceled))
		})
	})
	When("Using meta.Context with gRPC", func() {
		It("Should export and import values across unary gRPC boundaries", func() {
			By("Creating a context with some values")
			ctx := meta.NewContext(
				meta.WithProvider(identity.Component,
					meta.WithValue(types.TestComponent)),
				meta.WithProvider(identity.UUID),
				meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.TestComponent,
					logkc.WithLogLevel(zapcore.ErrorLevel),
				))),
				meta.WithProvider(tracing.Tracer),
				meta.WithProvider(host.SystemInfo),
			)
			By("Creating a gRPC server with the meta interceptor")
			fooSrv := &fooServer{}
			listener := bufconn.Listen(1024 * 1024)
			srv := grpc.NewServer(
				grpc.UnaryInterceptor(
					meta.ServerContextInterceptor(
						meta.ImportOptions{
							Required: []meta.Provider{
								identity.Component,
								identity.UUID,
							},
							Optional: []meta.Provider{
								host.SystemInfo,
							},
							Inherit: &meta.InheritOptions{
								InheritFrom: ctx,
								Providers: []meta.Provider{
									logkc.Logger,
									tracing.Tracer,
								},
							},
						})),
			)
			test.RegisterFooServer(srv, fooSrv)
			go srv.Serve(listener)
			defer srv.Stop()
			By("Creating a gRPC client with the meta interceptor")
			cc, err := grpc.Dial("bufconn",
				grpc.WithContextDialer(
					func(c context.Context, s string) (net.Conn, error) {
						return listener.Dial()
					}),
				grpc.WithUnaryInterceptor(meta.ClientContextInterceptor()),
				grpc.WithInsecure(),
			)
			Expect(err).NotTo(HaveOccurred())
			By("Calling a gRPC method from the client")
			client := test.NewFooClient(cc)
			reply, err := client.Foo(ctx, &test.Baz{})
			Expect(err).NotTo(HaveOccurred())
			Expect(reply).NotTo(BeNil())
		})
		It("Should export and import values across stream gRPC boundaries", func() {
			By("Creating a context with some values")
			ctx := meta.NewContext(
				meta.WithProvider(identity.Component,
					meta.WithValue(types.TestComponent)),
				meta.WithProvider(identity.UUID),
				meta.WithProvider(logkc.Logger, meta.WithValue(logkc.New(types.TestComponent,
					logkc.WithLogLevel(zapcore.ErrorLevel),
				))),
				meta.WithProvider(tracing.Tracer),
				meta.WithProvider(host.SystemInfo),
			)
			By("Creating a gRPC server with the stream meta interceptor")
			barSrv := &barServer{}
			listener := bufconn.Listen(1024 * 1024)
			srv := grpc.NewServer(
				grpc.StreamInterceptor(
					meta.StreamServerContextInterceptor(
						meta.ImportOptions{
							Required: []meta.Provider{
								identity.Component,
								identity.UUID,
							},
							Optional: []meta.Provider{
								host.SystemInfo,
							},
							Inherit: &meta.InheritOptions{
								InheritFrom: ctx,
								Providers: []meta.Provider{
									logkc.Logger,
									tracing.Tracer,
								},
							},
						})),
			)
			test.RegisterBarServer(srv, barSrv)
			go srv.Serve(listener)
			By("Creating a gRPC client with the stream meta interceptor")
			cc, err := grpc.Dial("bufconn",
				grpc.WithContextDialer(
					func(c context.Context, s string) (net.Conn, error) {
						return listener.Dial()
					}),
				grpc.WithStreamInterceptor(meta.StreamClientContextInterceptor()),
				grpc.WithInsecure(),
			)
			Expect(err).NotTo(HaveOccurred())
			By("Starting a gRPC stream")
			client := test.NewBarClient(cc)
			stream, err := client.Bar(ctx)
			Expect(err).NotTo(HaveOccurred())
			err = stream.Send(&test.Baz{})
			Expect(err).NotTo(HaveOccurred())
			err = stream.CloseSend()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
