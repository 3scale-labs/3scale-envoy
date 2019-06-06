package main

//
// This is a PoC, don't use it in production.
// If you have comments, issues, requests for improvements: joaquim@redhat.com.
//

import (
	"3scale-envoy/pkg/threescale_control_plane"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	log                  = logrus.New()
	hostname             = kingpin.Flag("hostname", "The hostname or address used by Envoy to reach this control plane.").Required().Envar("HOSTNAME").String()
	accessToken          = kingpin.Flag("access_token", "Your 3scale admin portal access token.").Required().Envar("ACCESS_TOKEN").String()
	threescaleAdminUrl   = kingpin.Flag("3scale_admin_url", "The URL of your 3scale Admin portal: \"https://tenant-admin.3scale.net:443/\".").Required().Envar("3SCALE_ADMIN_URL").String()
	serviceID            = kingpin.Flag("service_id", "The Service ID from 3scale to be used, if not set, envoy will expose all the services in the account.").Envar("SERVICE_ID").String()
	publicPort           = kingpin.Flag("public_port", "Gateway Public port, for external traffic.").Default("10000").Uint()
	xdsPort              = kingpin.Flag("xds_port", "xDS server, this is where Envoy should connect to get the configuration.").Default("18000").Uint()
	adminEnabled         = kingpin.Flag("admin_enabled", "Enable the admin endpoint in Envoy. (true or false)").Default("false").Bool()
	adminHTTPPort        = kingpin.Flag("admin_http_port", "Envoy HTTP admin endpoint port.").Default("19001").Uint()
	authPort             = kingpin.Flag("auth_port", "External AuthZ service port.").Default("9090").Uint()
	cacheTTL             = kingpin.Flag("cache_ttl", "Porta Cache time to wait before purging expired items from the cache.").Default("1m").Duration()
	cacheRefreshInterval = kingpin.Flag("cache_refresh_interval", "Porta cache time difference to refresh the cache element before expiry time.").Default("30s").Duration()
	cacheEntriesMax      = kingpin.Flag("cache_entries_max", "Porta cache max number of items that can be stored in the cache at any time.").Default("1000").Int()
	cacheUpdateRetries   = kingpin.Flag("cache_update_retries", "Porta Cache number of additional attempts made to update cached entries for unreachable hosts.").Default("2").Int()
)

func main() {
	kingpin.Parse()

	log.Info("Starting 3scale Envoy Control Plane")

	ec := threescale_control_plane.Server{
		CacheTTL:             *cacheTTL,
		CacheRefreshInterval: *cacheRefreshInterval,
		CacheUpdateRetries:   *cacheUpdateRetries,
		CacheEntriesMax:      *cacheEntriesMax,
		XDSport:              *xdsPort,
		AdminPort:            *adminHTTPPort,
		AuthPort:             *authPort,
		AdminEnabled:         *adminEnabled,
		PublicPort:           *publicPort,
		Host:                 *hostname,
		Config: threescale_control_plane.ThreescaleConfig{
			AccessToken: *accessToken,
			SystemURL:   *threescaleAdminUrl,
			ServiceID:   *serviceID,
			Environment: "production",
		},
	}
	ec.Start()

}
