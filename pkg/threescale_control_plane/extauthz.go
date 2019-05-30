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
	server     grpc.Server
	authorizer threescale_authorizer.Authorizer
}

func (ea envoyAuth) Check(ctx context.Context, ar *authZ.CheckRequest) (*authZ.CheckResponse, error) {

	requestHTTP, err := url.ParseRequestURI(ar.Attributes.Request.Http.Path)
	if err != nil {
		return &authZ.CheckResponse{
			Status: &rpc.Status{
				Code:    7,
				Message: "not_allowed",
				Details: nil,
			},
			HttpResponse: &authZ.CheckResponse_DeniedResponse{},
		}, nil
	}

	if ar.Attributes.ContextExtensions["service_id"] == "" ||
		ar.Attributes.ContextExtensions["system_url"] == "" ||
		ar.Attributes.ContextExtensions["access_token"] == "" {
		return &authZ.CheckResponse{
			Status: &rpc.Status{
				Code:    7,
				Message: "not_allowed",
				Details: nil,
			},
			HttpResponse: &authZ.CheckResponse_DeniedResponse{},
		}, nil
	}

	// TODO: Get the credentials from headers etc...
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

	if ea.authorizer.AuthRep(request) {
		return &authZ.CheckResponse{
			Status: &rpc.Status{
				Code:    0,
				Message: "ok",
				Details: nil,
			},
			HttpResponse: &authZ.CheckResponse_OkResponse{
				OkResponse: nil,
			},
		}, nil
	} else {
		return &authZ.CheckResponse{
			Status: &rpc.Status{
				Code:    7,
				Message: "not_allowed",
				Details: nil,
			},
			HttpResponse: &authZ.CheckResponse_DeniedResponse{},
		}, nil
	}
}
