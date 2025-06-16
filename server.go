package kamacache

import (
	"context"
	"crypto/tls"
	"fmt"
	"gitee.com/messizuo/kama-cache-go/consts"
	"gitee.com/messizuo/kama-cache-go/pb"
	"gitee.com/messizuo/kama-cache-go/registry"
	"github.com/sirupsen/logrus"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"net"
	"sync"
	"time"
)

// Server 定义缓存服务器
type Server struct {
	pb.UnimplementedKamaCacheServer
	addr       string
	srcName    string
	groups     *sync.Map    // 缓存组
	grpcServer *grpc.Server // grpc服务器
	etcdCli    *clientv3.Client
	stopCh     chan error
	opts       *ServerOptions
}

// Get pb实现
func (s *Server) Get(ctx context.Context, in *pb.Request) (*pb.ResponseForGet, error) {
	group := GetGroup(in.GetGroup())
	if group == nil {
		return nil, fmt.Errorf("group %s not found", in.GetGroup())
	}
	value, err := group.Get(ctx, in.GetKey())
	if err != nil {
		return nil, err
	}
	return &pb.ResponseForGet{Value: value.ByteSLice()}, nil
}

// Set pb实现
func (s *Server) Set(ctx context.Context, in *pb.Request) (*pb.ResponseForGet, error) {
	group := GetGroup(in.GetGroup())
	if group == nil {
		return nil, fmt.Errorf("group %s not found", in.GetGroup())
	}
	// 从context从获取标识
	fromPeer := ctx.Value(consts.FromPeer)
	if fromPeer == nil {
		ctx = context.WithValue(ctx, consts.FromPeer, true)
	}
	err := group.Set(ctx, in.GetKey(), in.GetValue())
	if err != nil {
		return nil, err
	}
	return &pb.ResponseForGet{Value: in.GetValue()}, nil
}

// Delete pb实现
func (s *Server) Delete(ctx context.Context, in *pb.Request) (*pb.ResponseForDelete, error) {
	group := GetGroup(in.GetGroup())
	if group == nil {
		return nil, fmt.Errorf("group %s not found", in.GetGroup())
	}
	err := group.Delete(ctx, in.GetKey())
	return &pb.ResponseForDelete{Value: err == nil}, err

}

// ServerOptions 服务器选项
type ServerOptions struct {
	EtcdEndpoints []string      // etcd端点
	DialTimeout   time.Duration // 连接超时
	MaxMsgSize    int           // 最大消息大小
	TLS           bool          // 是否启用TLS
	CertFile      string        // 证书文件
	KeyFile       string        // 密钥文件
}

// DefaultServerOptions 默认服务器选项
func DefaultServerOptions() *ServerOptions {
	return &ServerOptions{
		EtcdEndpoints: []string{"127.0.0.1:2379"},
		DialTimeout:   5 * time.Second,
		MaxMsgSize:    4 << 20, // 4MB
	}
}

// ServerOption 服务器选项
type ServerOption func(*ServerOptions)

// WithEtcdEndpoints 设置etcd端点
func WithEtcdEndpoints(endpoints []string) ServerOption {
	return func(o *ServerOptions) {
		o.EtcdEndpoints = endpoints
	}
}

// WithDialTimeout 设置连接超时
func WithDialTimeout(timeout time.Duration) ServerOption {
	return func(o *ServerOptions) {
		o.DialTimeout = timeout
	}
}

// WithMaxMsgSize 设置最大消息大小
func WithMaxMsgSize(size int) ServerOption {
	return func(o *ServerOptions) {
		o.MaxMsgSize = size
	}
}

// WithTLS 设置TLS
func WithTLS(certFile, keyFile string) ServerOption {
	return func(o *ServerOptions) {
		o.TLS = true
		o.CertFile = certFile
		o.KeyFile = keyFile
	}
}

// NewServer 创建服务器
func NewServer(addr, srcName string, opts ...ServerOption) (*Server, error) {
	options := DefaultServerOptions()
	for _, o := range opts {
		o(options)
	}
	// 创建etcd客户端
	etcdCli, err := clientv3.New(clientv3.Config{
		Endpoints:   options.EtcdEndpoints,
		DialTimeout: options.DialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %v", err)
	}
	// 创建grpc服务客户端
	var grpcServerOpts []grpc.ServerOption
	grpcServerOpts = append(grpcServerOpts, grpc.MaxRecvMsgSize(options.MaxMsgSize))
	if options.TLS {
		// 支持TLS
		creds, err := loadTLSCredentials(options.CertFile, options.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS credentials: %v", err)
		}
		grpcServerOpts = append(grpcServerOpts, grpc.Creds(creds))
	}
	s := &Server{
		addr:       addr,
		srcName:    srcName,
		groups:     new(sync.Map),
		opts:       options,
		stopCh:     make(chan error),
		etcdCli:    etcdCli,
		grpcServer: grpc.NewServer(grpcServerOpts...),
	}
	// 注册服务
	pb.RegisterKamaCacheServer(s.grpcServer, s)

	// 注册健康服务检查
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(s.grpcServer, health.NewServer())
	healthServer.SetServingStatus(srcName, healthpb.HealthCheckResponse_SERVING)

	return s, nil
}

// loadTLSCredentials 加载TLS凭证
func loadTLSCredentials(certFile, keyFile string) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS credentials: %v", err)
	}
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
	}), nil
}

// Start 启动服务器
func (s *Server) Start() error {
	// 启动grpc服务器
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	// 注册到etcd
	stopCh := make(chan error)
	go func() {
		if err = registry.Register(s.srcName, s.addr, stopCh); err != nil {
			logrus.Errorf("failed to register service: %v", err)
			close(stopCh)
			return
		}
	}()

	logrus.Infof("Start server at %s", s.addr)
	return s.grpcServer.Serve(lis)
}

// Stop 停止服务器
func (s *Server) Stop() {
	close(s.stopCh)
	s.grpcServer.Stop()
	if s.etcdCli != nil {
		err := s.etcdCli.Close()
		if err != nil {
			logrus.Errorf("failed to close etcd client: %v", err)
		}
	}
}
