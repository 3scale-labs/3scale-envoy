package threescale_control_plane

import (
	"fmt"
	conf "github.com/3scale/3scale-istio-adapter/config"
	"github.com/3scale/3scale-istio-adapter/pkg/threescale"
	sysC "github.com/3scale/3scale-porta-go-client/client"
	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	extAuthService "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/ext_authz/v2"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/envoyproxy/go-control-plane/pkg/util"
	"github.com/gogo/protobuf/types"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
)

type ThreescaleConfig struct {
	AccessToken    string
	SystemURL      string
	ServiceID      string
	Environment    string
	CurrentVersion int
}

func (c *ThreescaleConfig) newSystemClient() (*sysC.ThreeScaleClient, error) {
	sysURL, err := url.ParseRequestURI(c.SystemURL)
	if err != nil {
		return nil, err
	}

	scheme, host, port := c.parseURL(sysURL)
	ap, err := sysC.NewAdminPortal(scheme, host, port)
	if err != nil {
		return nil, err
	}

	return sysC.NewThreeScale(ap, c.AccessToken, &http.Client{}), nil
}
func (c *ThreescaleConfig) GetConfig(config threescale.ProxyConfigCache, version int32, AuthPort, PublicPort uint, Host string) (cache.Snapshot, int32) {

	// Generate the External AuthZ Cluster for envoy
	var clusterCache []cache.Resource
	var newVersion int32
	clusterCache = c.generateAuthZCluster(clusterCache, AuthPort, Host)

	systemClient, err := c.newSystemClient()
	if err != nil {
		return cache.Snapshot{}, version
	}

	proxyConf, err := config.Get(&conf.Params{
		ServiceId:   c.ServiceID,
		SystemUrl:   c.SystemURL,
		AccessToken: c.AccessToken,
	}, systemClient)

	if err != nil {
		return cache.Snapshot{}, version
	}

	if c.CurrentVersion == proxyConf.ProxyConfig.Version {
		return cache.Snapshot{}, version
	}

	apiBackend := proxyConf.ProxyConfig.Content.Proxy.APIBackend
	proxyEndpoint := proxyConf.ProxyConfig.Content.Proxy.Endpoint
	// Generate the Service Cluster for envoy
	clusterCache = c.generateServiceCluster(clusterCache, apiBackend)

	//
	// Generate the Route for the service.
	//
	contextExtensions := map[string]string{"service_id": c.ServiceID, "system_url": c.SystemURL, "access_token": c.AccessToken}

	checkSettings := extAuthService.ExtAuthzPerRoute_CheckSettings{CheckSettings: &extAuthService.CheckSettings{ContextExtensions: contextExtensions}}

	extAuthzPerRoute := extAuthService.ExtAuthzPerRoute{
		Override: &checkSettings,
	}

	extAuthConf, _ := util.MessageToStruct(&extAuthzPerRoute)

	proxyEndpointURL, err := url.Parse(proxyEndpoint)

	if err != nil {
		return cache.Snapshot{}, version
	}

	apiBackendURL, err := url.Parse(apiBackend)

	if err != nil {
		return cache.Snapshot{}, version
	}

	clusterName := strings.Replace(apiBackendURL.Hostname(), ".", "_", -1)

	r := c.newRoute(clusterName, apiBackendURL)

	//
	// Generate the VirtualHost for the service
	//

	v := c.newVirtualHost(proxyConf, proxyEndpointURL, r, extAuthConf)

	//
	// Generate the HTTPConnectionManager for the service
	//

	envoyGrpcConfig := c.newExternalAuthService()

	envoyConf, err := util.MessageToStruct(&envoyGrpcConfig)
	if err != nil {
		panic(err)
	}

	manager := c.newHTTPManager(v, envoyConf)

	pbst, err := util.MessageToStruct(manager)
	if err != nil {
		panic(err)
	}

	//
	// Generate the Listeners for the service

	listenersCache := c.newListenersCache(pbst, PublicPort)
	routesCache := []cache.Resource{&r}

	// Create the cache snapshot and add all the caches.
	// Set the local currentVersion pointer to the new config version, and increase the version for the snapshot.
	newVersion = version + 1
	c.CurrentVersion = proxyConf.ProxyConfig.Version

	snapshot := cache.NewSnapshot(fmt.Sprintf("%d", newVersion), nil, clusterCache, routesCache, listenersCache)

	return snapshot, newVersion
}

// TODO: Create better and more generic constructors for Clusters, Listeners, Routes...
func (c *ThreescaleConfig) generateServiceCluster(clusterCache []cache.Resource, apiBackend string) []cache.Resource {

	apiBackendURL, err := url.Parse(apiBackend)
	if err != nil {
		return nil
	}

	clusterName := strings.Replace(apiBackendURL.Hostname(), ".", "_", -1)

	var port uint32
	if apiBackendURL.Port() == "" {
		if apiBackendURL.Scheme == "http" {
			port = uint32(80)
		} else if apiBackendURL.Scheme == "https" {
			port = uint32(443)
		}
	} else {
		i, _ := strconv.Atoi(apiBackendURL.Port())
		port = uint32(i)
	}

	apiBackendAddress := &core.Address{
		Address: &core.Address_SocketAddress{
			SocketAddress: &core.SocketAddress{
				Protocol: core.TCP,
				Address:  apiBackendURL.Hostname(),
				PortSpecifier: &core.SocketAddress_PortValue{
					PortValue: port,
				},
				Ipv4Compat: true,
			},
		},
	}

	apiBackendCluster := &v2.Cluster{
		Name: clusterName,
		ClusterDiscoveryType: &v2.Cluster_Type{
			Type: v2.Cluster_LOGICAL_DNS,
		},
		ConnectTimeout: 5 * time.Second,
		LbPolicy:       v2.Cluster_ROUND_ROBIN,
		LoadAssignment: &v2.ClusterLoadAssignment{
			ClusterName: clusterName,
			Endpoints: []endpoint.LocalityLbEndpoints{{
				LbEndpoints: []endpoint.LbEndpoint{{
					HostIdentifier: &endpoint.LbEndpoint_Endpoint{
						Endpoint: &endpoint.Endpoint{
							Address: apiBackendAddress,
						},
					},
				},},
			},},
		},
	}

	if apiBackendURL.Scheme == "https" {
		apiBackendCluster.TlsContext = &auth.UpstreamTlsContext{
			CommonTlsContext:   nil,
			Sni:                apiBackendURL.Hostname(),
			AllowRenegotiation: false,
			MaxSessionKeys:     nil,
		}
	}
	clusterCache = append(clusterCache, apiBackendCluster)
	return clusterCache
}
func (c *ThreescaleConfig) generateAuthZCluster(clusterCache []cache.Resource, AuthPort uint, Host string) []cache.Resource {
	// externalAuthZ Cluster
	externalAuthZ := Host
	externalAuthZPort := uint32(AuthPort)
	extAuthzAddress := &core.Address{
		Address: &core.Address_SocketAddress{
			SocketAddress: &core.SocketAddress{
				Protocol: core.TCP,
				Address:  externalAuthZ,
				PortSpecifier: &core.SocketAddress_PortValue{
					PortValue: externalAuthZPort,
				},
				Ipv4Compat: true,
			},
		},
	}
	clusterName := "extauthz"

	extAuthZCluster := &v2.Cluster{
		Name: clusterName,
		ClusterDiscoveryType: &v2.Cluster_Type{
			Type: v2.Cluster_LOGICAL_DNS,
		},
		ConnectTimeout: 5 * time.Second,
		LbPolicy:       v2.Cluster_ROUND_ROBIN,
		LoadAssignment: &v2.ClusterLoadAssignment{
			ClusterName: clusterName,
			Endpoints: []endpoint.LocalityLbEndpoints{{
				LbEndpoints: []endpoint.LbEndpoint{{
					HostIdentifier: &endpoint.LbEndpoint_Endpoint{
						Endpoint: &endpoint.Endpoint{
							Address: extAuthzAddress,
						},
					},
				},},
			},},
		},
	}

	// Enable HTTP2 support

	extAuthZCluster.Http2ProtocolOptions = &core.Http2ProtocolOptions{
		HpackTableSize:              nil,
		MaxConcurrentStreams:        nil,
		InitialStreamWindowSize:     nil,
		InitialConnectionWindowSize: nil,
		AllowConnect:                false,
		AllowMetadata:               false,
	}

	clusterCache = append(clusterCache, extAuthZCluster)
	return clusterCache
}

func (c *ThreescaleConfig) newListenersCache(pbst *types.Struct, PublicPort uint) []cache.Resource {
	listenersCache := []cache.Resource{
		&v2.Listener{
			Name: "listener_0",
			Address: core.Address{
				Address: &core.Address_SocketAddress{
					SocketAddress: &core.SocketAddress{
						Protocol: core.TCP,
						Address:  "0.0.0.0",
						PortSpecifier: &core.SocketAddress_PortValue{
							PortValue: uint32(PublicPort),
						},
					},
				},
			},
			FilterChains: []listener.FilterChain{{
				Filters: []listener.Filter{{
					Name:       util.HTTPConnectionManager,
					ConfigType: &listener.Filter_Config{Config: pbst},
				}},
			}},
		},
	}
	return listenersCache
}
func (c *ThreescaleConfig) newRoute(clusterName string, apiBackendURL *url.URL) route.Route {
	r := route.Route{
		Match: route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: apiBackendURL.Path,
			},
		},
		Action: &route.Route_Route{
			Route: &route.RouteAction{
				ClusterSpecifier: &route.RouteAction_Cluster{
					Cluster: clusterName,
				},
				HostRewriteSpecifier: &route.RouteAction_HostRewrite{
					HostRewrite: apiBackendURL.Hostname(),
				},
			},
		},
	}
	return r
}
func (c *ThreescaleConfig) newExternalAuthService() extAuthService.ExtAuthz {
	envoyGrpcConfig := extAuthService.ExtAuthz{
		Services: &extAuthService.ExtAuthz_GrpcService{
			GrpcService: &core.GrpcService{
				TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
					EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
						ClusterName: "extauthz",
					},
				},
				Timeout: &types.Duration{
					Seconds: 5,
					Nanos:   0,
				},
			},
		},
		FailureModeAllow: false,
	}
	return envoyGrpcConfig
}

func (c *ThreescaleConfig) newHTTPManager(v route.VirtualHost, envoyConf *types.Struct) *hcm.HttpConnectionManager {
	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.AUTO,
		StatPrefix: "ingress_http",
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &v2.RouteConfiguration{
				Name:         "local_route",
				VirtualHosts: []route.VirtualHost{v},
			},
		},
		HttpFilters: []*hcm.HttpFilter{
			{
				Name: util.ExternalAuthorization,
				ConfigType: &hcm.HttpFilter_Config{
					Config: envoyConf,
				},
			},
			{
				Name: util.Router,
			},
		},
	}
	return manager
}
func (c *ThreescaleConfig) newVirtualHost(proxyConf sysC.ProxyConfigElement, proxyEndpointURL *url.URL, r route.Route, extAuthConf *types.Struct) route.VirtualHost {
	v := route.VirtualHost{
		Name:    strconv.Itoa(int(proxyConf.ProxyConfig.Content.Proxy.ServiceID)),
		Domains: []string{proxyEndpointURL.Hostname()},
		Routes:  []route.Route{r},
		PerFilterConfig: map[string]*types.Struct{
			util.ExternalAuthorization: extAuthConf,
		},
	}
	return v
}
func (c *ThreescaleConfig) parseURL(url *url.URL) (string, string, int) {
	scheme := url.Scheme
	if scheme == "" {
		scheme = "https"
	}

	host, port, _ := net.SplitHostPort(url.Host)
	if port == "" {
		if scheme == "http" {
			port = "80"
		} else if scheme == "https" {
			port = "443"
		}
	}

	if host == "" {
		host = url.Host
	}

	p, _ := strconv.Atoi(port)
	return scheme, host, p
}
