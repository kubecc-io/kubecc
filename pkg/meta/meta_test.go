package meta_test

import (
	"context"
	"net"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/internal/testutil"
	"github.com/cobalt77/kubecc/pkg/identity"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/meta/mdkeys"
	"github.com/cobalt77/kubecc/pkg/tracing"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

type fooServer struct {
	testutil.FooServer
}

func (s *fooServer) Foo(
	ctx context.Context,
	in *testutil.Baz,
) (*testutil.Baz, error) {
	defer GinkgoRecover()
	mctx := ctx.(meta.Context)
	Expect(mctx.Component()).To(Equal(types.TestComponent))
	Expect(func() { uuid.MustParse(mctx.UUID()) }).NotTo(Panic())
	Expect(mctx.Log()).NotTo(BeNil())
	Expect(mctx.Tracer()).NotTo(BeNil())
	return &testutil.Baz{}, nil
}

type barServer struct {
	testutil.BarServer
}

func (s *barServer) Bar(
	srv testutil.Bar_BarServer,
) error {
	defer GinkgoRecover()
	mctx := srv.Context().(meta.Context)
	Expect(mctx.Component()).To(Equal(types.TestComponent))
	Expect(func() { uuid.MustParse(mctx.UUID()) }).NotTo(Panic())
	Expect(mctx.Log()).NotTo(BeNil())
	Expect(mctx.Tracer()).NotTo(BeNil())
	return nil
}

var _ = Describe("Meta", func() {
	When("Creating a context with all providers", func() {
		var ctx meta.Context
		It("Should succeed", func() {
			Expect(func() {
				ctx = meta.NewContext(
					meta.WithProvider(identity.Component,
						meta.WithValue(types.TestComponent)),
					meta.WithProvider(identity.UUID),
					meta.WithProvider(logkc.MetadataProvider),
					meta.WithProvider(tracing.MetadataProvider),
				)
			}).ShouldNot(Panic())
			Expect(ctx).NotTo(BeNil())
		})
		It("Should contain values for each provider", func() {
			Expect(ctx.Component()).To(Equal(types.TestComponent))
			Expect(func() { uuid.MustParse(ctx.UUID()) }).NotTo(Panic())
			Expect(ctx.Log()).NotTo(BeNil())
			Expect(ctx.Tracer()).NotTo(BeNil())
		})
	})
	When("Creating a context with some providers", func() {
		var ctx meta.Context
		It("Should succeed", func() {
			Expect(func() {
				ctx = meta.NewContext(
					meta.WithProvider(identity.Component,
						meta.WithValue(types.TestComponent)),
					meta.WithProvider(logkc.MetadataProvider),
				)
			}).ShouldNot(Panic())
			Expect(ctx).NotTo(BeNil())
		})
		It("Should contain values for the given providers", func() {
			Expect(ctx.Component()).To(Equal(types.TestComponent))
			Expect(ctx.Log()).NotTo(BeNil())
			By("Causing a panic when querying nonexistent values")
			Expect(func() { ctx.UUID() }).To(Panic())
			Expect(func() { ctx.Tracer() }).To(Panic())
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
				meta.WithProvider(logkc.MetadataProvider),
				meta.WithProvider(tracing.MetadataProvider),
			)
			ctx2 := context.WithValue(ctx, testKey, "testValue")
			Expect(ctx2.Value(testKey)).To(Equal("testValue"))
			Expect(ctx2.Value(mdkeys.ComponentKey)).To(Equal(ctx.Component()))
			Expect(ctx2.Value(mdkeys.UUIDKey)).To(Equal(ctx.UUID()))
			Expect(ctx2.Value(mdkeys.LogKey)).To(Equal(ctx.Log()))
			Expect(ctx2.Value(mdkeys.TracingKey)).To(Equal(ctx.Tracer()))
		})
		It("Should allow overriding values", func() {
			ctx := meta.NewContext(
				meta.WithProvider(identity.Component,
					meta.WithValue(types.TestComponent)),
				meta.WithProvider(identity.UUID),
				meta.WithProvider(logkc.MetadataProvider),
				meta.WithProvider(tracing.MetadataProvider),
			)
			newLog := ctx.Log().Named("test")
			ctx2 := context.WithValue(ctx, mdkeys.LogKey, newLog)
			Expect(ctx2.Value(mdkeys.ComponentKey)).To(Equal(ctx.Component()))
			Expect(ctx2.Value(mdkeys.UUIDKey)).To(Equal(ctx.UUID()))
			Expect(ctx2.Value(mdkeys.LogKey)).To(Equal(newLog))
			Expect(ctx2.Value(mdkeys.TracingKey)).To(Equal(ctx.Tracer()))
		})
	})
	When("Using meta.Context with gRPC", func() {
		It("Should export and import values across unary gRPC boundaries", func() {
			By("Creating a context with some values")
			ctx := meta.NewContext(
				meta.WithProvider(identity.Component,
					meta.WithValue(types.TestComponent)),
				meta.WithProvider(identity.UUID),
				meta.WithProvider(logkc.MetadataProvider),
				meta.WithProvider(tracing.MetadataProvider),
			)
			By("Creating a gRPC server with the meta interceptor")
			fooSrv := &fooServer{}
			listener := bufconn.Listen(1024 * 1024)
			srv := grpc.NewServer(
				grpc.UnaryInterceptor(
					meta.ServerContextInterceptor(ctx,
						[]meta.Provider{identity.Component, identity.UUID})),
			)
			testutil.RegisterFooServer(srv, fooSrv)
			go srv.Serve(listener)
			defer srv.GracefulStop()
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
			client := testutil.NewFooClient(cc)
			reply, err := client.Foo(ctx, &testutil.Baz{})
			Expect(err).NotTo(HaveOccurred())
			Expect(reply).NotTo(BeNil())
		})
		It("Should export and import values across stream gRPC boundaries", func() {
			By("Creating a context with some values")
			ctx := meta.NewContext(
				meta.WithProvider(identity.Component,
					meta.WithValue(types.TestComponent)),
				meta.WithProvider(identity.UUID),
				meta.WithProvider(logkc.MetadataProvider),
				meta.WithProvider(tracing.MetadataProvider),
			)
			By("Creating a gRPC server with the stream meta interceptor")
			barSrv := &barServer{}
			listener := bufconn.Listen(1024 * 1024)
			srv := grpc.NewServer(
				grpc.StreamInterceptor(
					meta.StreamServerContextInterceptor(ctx,
						[]meta.Provider{identity.Component, identity.UUID})),
			)
			testutil.RegisterBarServer(srv, barSrv)
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
			client := testutil.NewBarClient(cc)
			stream, err := client.Bar(ctx)
			Expect(err).NotTo(HaveOccurred())
			err = stream.Send(&testutil.Baz{})
			Expect(err).NotTo(HaveOccurred())
			err = stream.CloseSend()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
