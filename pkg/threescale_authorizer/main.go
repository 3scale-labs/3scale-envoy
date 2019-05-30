package threescale_authorizer

import (
	backendC "github.com/3scale/3scale-go-client/client"
	"github.com/3scale/3scale-istio-adapter/config"
	"github.com/3scale/3scale-istio-adapter/pkg/threescale"
	"github.com/3scale/3scale-istio-adapter/pkg/threescale/metrics"
	sysC "github.com/3scale/3scale-porta-go-client/client"
	logger "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	log = logger.New()
)

//
// This is a PoC, most of the code is taken from https://github.com/3scale/3scale-istio-adapter
// and adapted to this use case.
//

type authRepRequest struct {
	svcID   string
	authKey string
	params  backendC.AuthRepParams
	auth    backendC.TokenAuth
}

type AuthorizeRequest struct {
	Host        string `json:"host"` // not used yet...
	ServiceId   string `json:"service_id"`
	SystemUrl   string `json:"system_url"`
	AccessToken string `json:"access_token"`
	Path        string `json:"path"`
	Method      string `json:"method"`
	AppID       string `json:"appID"`
	AppKey      string `json:"appKey"`
	UserKey     string `json:"userKey"`
}

type authRepFn func(auth backendC.TokenAuth, key string, svcID string, params backendC.AuthRepParams, ext map[string]string) (backendC.ApiResponse, error)

type Authorizer struct {
	systemCache     *threescale.ProxyConfigCache
	metricsReporter *metrics.Reporter
}

func (a *Authorizer) systemClientBuilder(systemURL string) (*sysC.ThreeScaleClient, error) {
	sysURL, err := url.ParseRequestURI(systemURL)
	if err != nil {
		return nil, err
	}

	scheme, host, port := parseURL(sysURL)
	ap, err := sysC.NewAdminPortal(scheme, host, port)
	if err != nil {
		return nil, err
	}

	return sysC.NewThreeScale(ap, &http.Client{}), nil
}
func (a *Authorizer) backendClientBuilder(backendURL string) (*backendC.ThreeScaleClient, error) {
	parsedUrl, err := url.ParseRequestURI(backendURL)
	if err != nil {
		return nil, err
	}

	scheme, host, port := parseURL(parsedUrl)
	be, err := backendC.NewBackend(scheme, host, port)
	if err != nil {
		return nil, err
	}

	return backendC.NewThreeScale(be, &http.Client{}), nil
}
func (a *Authorizer) AuthRep(request AuthorizeRequest) bool {

	var (
		authRep authRepFn
	)

	params := config.Params{
		ServiceId:   request.ServiceId,
		SystemUrl:   request.SystemUrl,
		AccessToken: request.AccessToken,
	}

	threeScaleClient, err := a.systemClientBuilder(params.SystemUrl)
	if err != nil {
		log.Fatal(err)
	}

	pce, err := a.systemCache.Get(&params, threeScaleClient)
	if err != nil {
		log.Fatal(err)
	}

	m := generateMetrics(request.Path, request.Method, pce.ProxyConfig)
	if len(m) == 0 {
		return false
	}

	backendClient, err := a.backendClientBuilder(pce.ProxyConfig.Content.Proxy.Backend.Endpoint)

	var authRepRequest authRepRequest
	if request.UserKey != "" {
		authRepRequest.authKey = request.UserKey
		authRepRequest.params = backendC.NewAuthRepParamsUserKey("", "", m, nil)
		authRep = backendClient.AuthRepUserKey
	} else {
		authRepRequest.authKey = request.AppID
		authRepRequest.params = backendC.NewAuthRepParamsAppID(request.AppKey, "", "", m, nil)
		authRep = backendClient.AuthRepAppID
	}

	authRepRequest.auth.Type = pce.ProxyConfig.Content.BackendAuthenticationType
	authRepRequest.auth.Value = pce.ProxyConfig.Content.BackendAuthenticationValue

	resp, _ := authRep(authRepRequest.auth, authRepRequest.authKey, params.ServiceId, authRepRequest.params, nil)

	return resp.Success
}

func NewAuthorizer(cache *threescale.ProxyConfigCache) *Authorizer {
	return &Authorizer{
		systemCache:     cache,
		metricsReporter: nil,
	}
}

func generateMetrics(path string, method string, conf sysC.ProxyConfig) backendC.Metrics {
	m := make(backendC.Metrics)

	for _, pr := range conf.Content.Proxy.ProxyRules {
		if match, err := regexp.MatchString(pr.Pattern, path); err == nil {
			if match && strings.ToUpper(pr.HTTPMethod) == strings.ToUpper(method) {
				baseDelta := 0
				if val, ok := m[pr.MetricSystemName]; ok {
					baseDelta = val
				}
				err = m.Add(pr.MetricSystemName, baseDelta+int(pr.Delta))
				if err != nil {
					log.Println(err)
				}
			}
		}
	}
	return m
}

func parseURL(url *url.URL) (string, string, int) {
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
