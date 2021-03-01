// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package testutil

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// FooClient is the client API for Foo service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type FooClient interface {
	Foo(ctx context.Context, in *Baz, opts ...grpc.CallOption) (*Baz, error)
}

type fooClient struct {
	cc grpc.ClientConnInterface
}

func NewFooClient(cc grpc.ClientConnInterface) FooClient {
	return &fooClient{cc}
}

func (c *fooClient) Foo(ctx context.Context, in *Baz, opts ...grpc.CallOption) (*Baz, error) {
	out := new(Baz)
	err := c.cc.Invoke(ctx, "/Foo/Foo", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// FooServer is the server API for Foo service.
// All implementations must embed UnimplementedFooServer
// for forward compatibility
type FooServer interface {
	Foo(context.Context, *Baz) (*Baz, error)
	mustEmbedUnimplementedFooServer()
}

// UnimplementedFooServer must be embedded to have forward compatible implementations.
type UnimplementedFooServer struct {
}

func (UnimplementedFooServer) Foo(context.Context, *Baz) (*Baz, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Foo not implemented")
}
func (UnimplementedFooServer) mustEmbedUnimplementedFooServer() {}

// UnsafeFooServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to FooServer will
// result in compilation errors.
type UnsafeFooServer interface {
	mustEmbedUnimplementedFooServer()
}

func RegisterFooServer(s grpc.ServiceRegistrar, srv FooServer) {
	s.RegisterService(&Foo_ServiceDesc, srv)
}

func _Foo_Foo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Baz)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FooServer).Foo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/Foo/Foo",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FooServer).Foo(ctx, req.(*Baz))
	}
	return interceptor(ctx, in, info, handler)
}

// Foo_ServiceDesc is the grpc.ServiceDesc for Foo service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Foo_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "Foo",
	HandlerType: (*FooServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Foo",
			Handler:    _Foo_Foo_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/testpb.proto",
}

// BarClient is the client API for Bar service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type BarClient interface {
	Bar(ctx context.Context, opts ...grpc.CallOption) (Bar_BarClient, error)
}

type barClient struct {
	cc grpc.ClientConnInterface
}

func NewBarClient(cc grpc.ClientConnInterface) BarClient {
	return &barClient{cc}
}

func (c *barClient) Bar(ctx context.Context, opts ...grpc.CallOption) (Bar_BarClient, error) {
	stream, err := c.cc.NewStream(ctx, &Bar_ServiceDesc.Streams[0], "/Bar/Bar", opts...)
	if err != nil {
		return nil, err
	}
	x := &barBarClient{stream}
	return x, nil
}

type Bar_BarClient interface {
	Send(*Baz) error
	Recv() (*Baz, error)
	grpc.ClientStream
}

type barBarClient struct {
	grpc.ClientStream
}

func (x *barBarClient) Send(m *Baz) error {
	return x.ClientStream.SendMsg(m)
}

func (x *barBarClient) Recv() (*Baz, error) {
	m := new(Baz)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// BarServer is the server API for Bar service.
// All implementations must embed UnimplementedBarServer
// for forward compatibility
type BarServer interface {
	Bar(Bar_BarServer) error
	mustEmbedUnimplementedBarServer()
}

// UnimplementedBarServer must be embedded to have forward compatible implementations.
type UnimplementedBarServer struct {
}

func (UnimplementedBarServer) Bar(Bar_BarServer) error {
	return status.Errorf(codes.Unimplemented, "method Bar not implemented")
}
func (UnimplementedBarServer) mustEmbedUnimplementedBarServer() {}

// UnsafeBarServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to BarServer will
// result in compilation errors.
type UnsafeBarServer interface {
	mustEmbedUnimplementedBarServer()
}

func RegisterBarServer(s grpc.ServiceRegistrar, srv BarServer) {
	s.RegisterService(&Bar_ServiceDesc, srv)
}

func _Bar_Bar_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(BarServer).Bar(&barBarServer{stream})
}

type Bar_BarServer interface {
	Send(*Baz) error
	Recv() (*Baz, error)
	grpc.ServerStream
}

type barBarServer struct {
	grpc.ServerStream
}

func (x *barBarServer) Send(m *Baz) error {
	return x.ServerStream.SendMsg(m)
}

func (x *barBarServer) Recv() (*Baz, error) {
	m := new(Baz)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Bar_ServiceDesc is the grpc.ServiceDesc for Bar service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Bar_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "Bar",
	HandlerType: (*BarServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Bar",
			Handler:       _Bar_Bar_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "proto/testpb.proto",
}