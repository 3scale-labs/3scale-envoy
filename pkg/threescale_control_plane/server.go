package threescale_control_plane

import (
	"3scale-envoy/pkg/threescale_authorizer"
	"context"
	"fmt"
	"github.com/3scale/3scale-istio-adapter/pkg/threescale"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	authZ "github.com/envoyproxy/go-control-plane/envoy/service/auth/v2"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	xds "github.com/envoyproxy/go-control-plane/pkg/server"
	logger "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"net"
	"net/http"
	"time"
)

var (
	log    = logger.New()
	config cache.SnapshotCache
)

const (
	grpcMaxConcurrentStreams = 1000000
	nodeID                   = "3scale-envoy-gateway"
)

type ControlPlane struct {
	CacheTTL, CacheRefreshInterval           time.Duration
	CacheUpdateRetries, CacheEntriesMax      int
	AuthPort, XDSport, AdminPort, PublicPort uint
	AdminEnabled                             bool
	Config                                   ThreescaleConfig
	Host                                     string
}

func (ec *ControlPlane) Start() {

	ctx := context.Background()
	signal := make(chan struct{})
	cb := &callbacks{
		signal:   signal,
		fetches:  0,
		requests: 0,
	}
	config = cache.NewSnapshotCache(true, Hasher{}, nil)

	proxyCache := threescale.NewProxyConfigCache(ec.CacheTTL, ec.CacheRefreshInterval, ec.CacheUpdateRetries, ec.CacheEntriesMax)
	err := proxyCache.StartRefreshWorker()
	if err != nil {
		panic(err)
	}
	authorizer := threescale_authorizer.NewAuthorizer(proxyCache)

	srv := xds.NewServer(config, cb)

	go RunManagementServer(ctx, srv, ec.XDSport)

	if ec.AdminEnabled {
		go RunManagementGateway(ctx, srv, ec.AdminPort)
	}

	go RunExternalAuthzService(ctx, authorizer, ec.AuthPort)

	var waitFor time.Duration
	waitFor = ec.CacheTTL - ec.CacheRefreshInterval + 10*time.Second

	<-signal
	var version, newVersion int32
	version = 0
	newVersion = 0

	for {
		log.Println("Refreshing config 3scale, version:" + fmt.Sprint(version))
		snap := cache.Snapshot{}

		snap, newVersion = ec.Config.GetConfig(*proxyCache, version, ec.AuthPort, ec.PublicPort, ec.Host)
		if newVersion != version {
			log.Printf("Updating new version: %d", newVersion)
			err := config.SetSnapshot(nodeID, snap)
			if err != nil {
				log.Println(err)
			}
			version = newVersion
		} else {
			log.Printf("No changes detected in the 3scale configuration.")
		}

		log.Printf("Refreshing from 3scale in: %s", waitFor)
		time.Sleep(waitFor)
	}
}

// RunExternalAuthzService starts an external-authorization service for envoy
func RunExternalAuthzService(ctx context.Context, server *threescale_authorizer.Authorizer, port uint) {

	var grpcOptions []grpc.ServerOption
	grpcOptions = append(grpcOptions, grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams))
	grpcServer := grpc.NewServer(grpcOptions...)
	ea := envoyAuth{
		server:     *grpcServer,
		authorizer: *server,
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Println("Failed to listen")
	}

	authZ.RegisterAuthorizationServer(grpcServer, ea)

	log.Printf("Starting Authorization Service on Port %d\n", port)
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.Error(err)
		}
	}()
	<-ctx.Done()

	grpcServer.GracefulStop()

}

// RunManagementServer starts an xDS server at the given Port.
func RunManagementServer(ctx context.Context, server xds.Server, port uint) {
	var grpcOptions []grpc.ServerOption
	grpcOptions = append(grpcOptions, grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams))
	grpcServer := grpc.NewServer(grpcOptions...)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Error("Failed to listen")
	}

	// register services
	discovery.RegisterAggregatedDiscoveryServiceServer(grpcServer, server)
	v2.RegisterEndpointDiscoveryServiceServer(grpcServer, server)
	v2.RegisterClusterDiscoveryServiceServer(grpcServer, server)
	v2.RegisterRouteDiscoveryServiceServer(grpcServer, server)
	v2.RegisterListenerDiscoveryServiceServer(grpcServer, server)

	log.Printf("Starting Management Server on Port %d\n", port)
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			log.Error(err)
		}
	}()
	<-ctx.Done()

	grpcServer.GracefulStop()
}

// RunManagementGateway starts an HTTP gateway to an xDS server.
func RunManagementGateway(ctx context.Context, srv xds.Server, port uint) {
	log.Printf("Starting HTTP/1.1 gateway on Port %d\n", port)
	server := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: &xds.HTTPGateway{Server: srv}}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			panic(err)
		}
	}()

	<-ctx.Done()
	if err := server.Shutdown(ctx); err != nil {
		panic(err)
	}
}
