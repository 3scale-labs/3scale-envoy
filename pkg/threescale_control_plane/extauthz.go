package threescale_control_plane

import (
	"3scale-envoy/pkg/threescale_authorizer"
	"context"
	authZ "github.com/envoyproxy/go-control-plane/envoy/service/auth/v2"
	"github.com/gogo/googleapis/google/rpc"
	"google.golang.org/grpc"
	"net/url"
)

type envoyAuth struct {
	Server     grpc.Server
	Authorizer threescale_authorizer.Server
}

var (
	requestDenied = &authZ.CheckResponse{
		Status: &rpc.Status{
			Code:    7,
			Message: "not_allowed",
			Details: nil,
		},
		HttpResponse: &authZ.CheckResponse_DeniedResponse{},
	}
	requestAllowed = &authZ.CheckResponse{
		Status: &rpc.Status{
			Code:    0,
			Message: "ok",
			Details: nil,
		},
		HttpResponse: &authZ.CheckResponse_OkResponse{
			OkResponse: nil,
		},
	}
)

func (ea envoyAuth) Check(ctx context.Context, ar *authZ.CheckRequest) (*authZ.CheckResponse, error) {

	requestHTTP, err := url.ParseRequestURI(ar.Attributes.Request.Http.Path)
	if err != nil {
		return requestDenied, nil
	}

	if ar.Attributes.ContextExtensions["service_id"] == "" ||
		ar.Attributes.ContextExtensions["system_url"] == "" ||
		ar.Attributes.ContextExtensions["access_token"] == "" {
		return requestDenied, nil
	}

	// TODO: Get the credentials from headers, or custom query params...
	request := threescale_authorizer.AuthorizeRequest{
		Host:        ar.Attributes.Request.Http.Host,
		ServiceId:   ar.Attributes.ContextExtensions["service_id"],
		SystemUrl:   ar.Attributes.ContextExtensions["system_url"],
		AccessToken: ar.Attributes.ContextExtensions["access_token"],
		Path:        requestHTTP.Path,
		Method:      ar.Attributes.Request.Http.Method,
		AppID:       requestHTTP.Query().Get("app_id"),
		AppKey:      requestHTTP.Query().Get("app_key"),
		UserKey:     requestHTTP.Query().Get("user_key"),
	}

	if ea.Authorizer.AuthRep(request) {
		return requestAllowed, nil
	} else {
		return requestDenied, nil
	}
}
