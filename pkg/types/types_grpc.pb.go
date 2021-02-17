// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package types

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

// ConsumerdClient is the client API for Consumerd service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ConsumerdClient interface {
	Run(ctx context.Context, in *RunRequest, opts ...grpc.CallOption) (*RunResponse, error)
}

type consumerdClient struct {
	cc grpc.ClientConnInterface
}

func NewConsumerdClient(cc grpc.ClientConnInterface) ConsumerdClient {
	return &consumerdClient{cc}
}

func (c *consumerdClient) Run(ctx context.Context, in *RunRequest, opts ...grpc.CallOption) (*RunResponse, error) {
	out := new(RunResponse)
	err := c.cc.Invoke(ctx, "/Consumerd/Run", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ConsumerdServer is the server API for Consumerd service.
// All implementations must embed UnimplementedConsumerdServer
// for forward compatibility
type ConsumerdServer interface {
	Run(context.Context, *RunRequest) (*RunResponse, error)
	mustEmbedUnimplementedConsumerdServer()
}

// UnimplementedConsumerdServer must be embedded to have forward compatible implementations.
type UnimplementedConsumerdServer struct {
}

func (UnimplementedConsumerdServer) Run(context.Context, *RunRequest) (*RunResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Run not implemented")
}
func (UnimplementedConsumerdServer) mustEmbedUnimplementedConsumerdServer() {}

// UnsafeConsumerdServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ConsumerdServer will
// result in compilation errors.
type UnsafeConsumerdServer interface {
	mustEmbedUnimplementedConsumerdServer()
}

func RegisterConsumerdServer(s grpc.ServiceRegistrar, srv ConsumerdServer) {
	s.RegisterService(&Consumerd_ServiceDesc, srv)
}

func _Consumerd_Run_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RunRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConsumerdServer).Run(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/Consumerd/Run",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConsumerdServer).Run(ctx, req.(*RunRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Consumerd_ServiceDesc is the grpc.ServiceDesc for Consumerd service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Consumerd_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "Consumerd",
	HandlerType: (*ConsumerdServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Run",
			Handler:    _Consumerd_Run_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/types.proto",
}

// AgentClient is the client API for Agent service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type AgentClient interface {
	Compile(ctx context.Context, in *CompileRequest, opts ...grpc.CallOption) (*CompileResponse, error)
	GetCpuConfig(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*CpuConfig, error)
	SetCpuConfig(ctx context.Context, in *CpuConfig, opts ...grpc.CallOption) (*Empty, error)
}

type agentClient struct {
	cc grpc.ClientConnInterface
}

func NewAgentClient(cc grpc.ClientConnInterface) AgentClient {
	return &agentClient{cc}
}

func (c *agentClient) Compile(ctx context.Context, in *CompileRequest, opts ...grpc.CallOption) (*CompileResponse, error) {
	out := new(CompileResponse)
	err := c.cc.Invoke(ctx, "/Agent/Compile", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *agentClient) GetCpuConfig(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*CpuConfig, error) {
	out := new(CpuConfig)
	err := c.cc.Invoke(ctx, "/Agent/GetCpuConfig", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *agentClient) SetCpuConfig(ctx context.Context, in *CpuConfig, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/Agent/SetCpuConfig", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AgentServer is the server API for Agent service.
// All implementations must embed UnimplementedAgentServer
// for forward compatibility
type AgentServer interface {
	Compile(context.Context, *CompileRequest) (*CompileResponse, error)
	GetCpuConfig(context.Context, *Empty) (*CpuConfig, error)
	SetCpuConfig(context.Context, *CpuConfig) (*Empty, error)
	mustEmbedUnimplementedAgentServer()
}

// UnimplementedAgentServer must be embedded to have forward compatible implementations.
type UnimplementedAgentServer struct {
}

func (UnimplementedAgentServer) Compile(context.Context, *CompileRequest) (*CompileResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Compile not implemented")
}
func (UnimplementedAgentServer) GetCpuConfig(context.Context, *Empty) (*CpuConfig, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetCpuConfig not implemented")
}
func (UnimplementedAgentServer) SetCpuConfig(context.Context, *CpuConfig) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetCpuConfig not implemented")
}
func (UnimplementedAgentServer) mustEmbedUnimplementedAgentServer() {}

// UnsafeAgentServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to AgentServer will
// result in compilation errors.
type UnsafeAgentServer interface {
	mustEmbedUnimplementedAgentServer()
}

func RegisterAgentServer(s grpc.ServiceRegistrar, srv AgentServer) {
	s.RegisterService(&Agent_ServiceDesc, srv)
}

func _Agent_Compile_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CompileRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentServer).Compile(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/Agent/Compile",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentServer).Compile(ctx, req.(*CompileRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Agent_GetCpuConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentServer).GetCpuConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/Agent/GetCpuConfig",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentServer).GetCpuConfig(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Agent_SetCpuConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CpuConfig)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentServer).SetCpuConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/Agent/SetCpuConfig",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentServer).SetCpuConfig(ctx, req.(*CpuConfig))
	}
	return interceptor(ctx, in, info, handler)
}

// Agent_ServiceDesc is the grpc.ServiceDesc for Agent service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Agent_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "Agent",
	HandlerType: (*AgentServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Compile",
			Handler:    _Agent_Compile_Handler,
		},
		{
			MethodName: "GetCpuConfig",
			Handler:    _Agent_GetCpuConfig_Handler,
		},
		{
			MethodName: "SetCpuConfig",
			Handler:    _Agent_SetCpuConfig_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/types.proto",
}

// SchedulerClient is the client API for Scheduler service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SchedulerClient interface {
	Compile(ctx context.Context, in *CompileRequest, opts ...grpc.CallOption) (*CompileResponse, error)
	ConnectAgent(ctx context.Context, opts ...grpc.CallOption) (Scheduler_ConnectAgentClient, error)
	ConnectConsumerd(ctx context.Context, opts ...grpc.CallOption) (Scheduler_ConnectConsumerdClient, error)
	SystemStatus(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*SystemStatusResponse, error)
}

type schedulerClient struct {
	cc grpc.ClientConnInterface
}

func NewSchedulerClient(cc grpc.ClientConnInterface) SchedulerClient {
	return &schedulerClient{cc}
}

func (c *schedulerClient) Compile(ctx context.Context, in *CompileRequest, opts ...grpc.CallOption) (*CompileResponse, error) {
	out := new(CompileResponse)
	err := c.cc.Invoke(ctx, "/Scheduler/Compile", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schedulerClient) ConnectAgent(ctx context.Context, opts ...grpc.CallOption) (Scheduler_ConnectAgentClient, error) {
	stream, err := c.cc.NewStream(ctx, &Scheduler_ServiceDesc.Streams[0], "/Scheduler/ConnectAgent", opts...)
	if err != nil {
		return nil, err
	}
	x := &schedulerConnectAgentClient{stream}
	return x, nil
}

type Scheduler_ConnectAgentClient interface {
	Send(*Metadata) error
	Recv() (*Empty, error)
	grpc.ClientStream
}

type schedulerConnectAgentClient struct {
	grpc.ClientStream
}

func (x *schedulerConnectAgentClient) Send(m *Metadata) error {
	return x.ClientStream.SendMsg(m)
}

func (x *schedulerConnectAgentClient) Recv() (*Empty, error) {
	m := new(Empty)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *schedulerClient) ConnectConsumerd(ctx context.Context, opts ...grpc.CallOption) (Scheduler_ConnectConsumerdClient, error) {
	stream, err := c.cc.NewStream(ctx, &Scheduler_ServiceDesc.Streams[1], "/Scheduler/ConnectConsumerd", opts...)
	if err != nil {
		return nil, err
	}
	x := &schedulerConnectConsumerdClient{stream}
	return x, nil
}

type Scheduler_ConnectConsumerdClient interface {
	Send(*Metadata) error
	Recv() (*Empty, error)
	grpc.ClientStream
}

type schedulerConnectConsumerdClient struct {
	grpc.ClientStream
}

func (x *schedulerConnectConsumerdClient) Send(m *Metadata) error {
	return x.ClientStream.SendMsg(m)
}

func (x *schedulerConnectConsumerdClient) Recv() (*Empty, error) {
	m := new(Empty)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *schedulerClient) SystemStatus(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*SystemStatusResponse, error) {
	out := new(SystemStatusResponse)
	err := c.cc.Invoke(ctx, "/Scheduler/SystemStatus", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SchedulerServer is the server API for Scheduler service.
// All implementations must embed UnimplementedSchedulerServer
// for forward compatibility
type SchedulerServer interface {
	Compile(context.Context, *CompileRequest) (*CompileResponse, error)
	ConnectAgent(Scheduler_ConnectAgentServer) error
	ConnectConsumerd(Scheduler_ConnectConsumerdServer) error
	SystemStatus(context.Context, *Empty) (*SystemStatusResponse, error)
	mustEmbedUnimplementedSchedulerServer()
}

// UnimplementedSchedulerServer must be embedded to have forward compatible implementations.
type UnimplementedSchedulerServer struct {
}

func (UnimplementedSchedulerServer) Compile(context.Context, *CompileRequest) (*CompileResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Compile not implemented")
}
func (UnimplementedSchedulerServer) ConnectAgent(Scheduler_ConnectAgentServer) error {
	return status.Errorf(codes.Unimplemented, "method ConnectAgent not implemented")
}
func (UnimplementedSchedulerServer) ConnectConsumerd(Scheduler_ConnectConsumerdServer) error {
	return status.Errorf(codes.Unimplemented, "method ConnectConsumerd not implemented")
}
func (UnimplementedSchedulerServer) SystemStatus(context.Context, *Empty) (*SystemStatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SystemStatus not implemented")
}
func (UnimplementedSchedulerServer) mustEmbedUnimplementedSchedulerServer() {}

// UnsafeSchedulerServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SchedulerServer will
// result in compilation errors.
type UnsafeSchedulerServer interface {
	mustEmbedUnimplementedSchedulerServer()
}

func RegisterSchedulerServer(s grpc.ServiceRegistrar, srv SchedulerServer) {
	s.RegisterService(&Scheduler_ServiceDesc, srv)
}

func _Scheduler_Compile_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CompileRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulerServer).Compile(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/Scheduler/Compile",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulerServer).Compile(ctx, req.(*CompileRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Scheduler_ConnectAgent_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(SchedulerServer).ConnectAgent(&schedulerConnectAgentServer{stream})
}

type Scheduler_ConnectAgentServer interface {
	Send(*Empty) error
	Recv() (*Metadata, error)
	grpc.ServerStream
}

type schedulerConnectAgentServer struct {
	grpc.ServerStream
}

func (x *schedulerConnectAgentServer) Send(m *Empty) error {
	return x.ServerStream.SendMsg(m)
}

func (x *schedulerConnectAgentServer) Recv() (*Metadata, error) {
	m := new(Metadata)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _Scheduler_ConnectConsumerd_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(SchedulerServer).ConnectConsumerd(&schedulerConnectConsumerdServer{stream})
}

type Scheduler_ConnectConsumerdServer interface {
	Send(*Empty) error
	Recv() (*Metadata, error)
	grpc.ServerStream
}

type schedulerConnectConsumerdServer struct {
	grpc.ServerStream
}

func (x *schedulerConnectConsumerdServer) Send(m *Empty) error {
	return x.ServerStream.SendMsg(m)
}

func (x *schedulerConnectConsumerdServer) Recv() (*Metadata, error) {
	m := new(Metadata)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _Scheduler_SystemStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchedulerServer).SystemStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/Scheduler/SystemStatus",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchedulerServer).SystemStatus(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

// Scheduler_ServiceDesc is the grpc.ServiceDesc for Scheduler service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Scheduler_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "Scheduler",
	HandlerType: (*SchedulerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Compile",
			Handler:    _Scheduler_Compile_Handler,
		},
		{
			MethodName: "SystemStatus",
			Handler:    _Scheduler_SystemStatus_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ConnectAgent",
			Handler:       _Scheduler_ConnectAgent_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "ConnectConsumerd",
			Handler:       _Scheduler_ConnectConsumerd_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "proto/types.proto",
}