module 3scale-envoy

go 1.12

require (
	github.com/3scale/3scale-go-client v0.0.2-0.20190408085735-e366adc214e9
	github.com/3scale/3scale-istio-adapter v0.6.1-0.20190528142031-34f6878a8b28
	github.com/3scale/3scale-porta-go-client v0.0.3
	github.com/cenkalti/backoff v2.1.1+incompatible // indirect
	github.com/codahale/hdrhistogram v0.0.0-20161010025455-3a0bb77429bd // indirect
	github.com/d4l3k/messagediff v1.2.1 // indirect
	github.com/envoyproxy/go-control-plane v0.8.0
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/gogo/googleapis v1.2.0
	github.com/gogo/protobuf v1.2.1
	github.com/gogo/status v1.1.0 // indirect
	github.com/golang/sync v0.0.0-20190412183630-56d357773e84 // indirect
	github.com/gorilla/mux v1.7.2 // indirect
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645 // indirect
	github.com/hashicorp/go-multierror v1.0.0 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/natefinch/lumberjack v2.0.0+incompatible // indirect
	github.com/onsi/gomega v1.5.0 // indirect
	github.com/opentracing/opentracing-go v1.1.0 // indirect
	github.com/prometheus/client_golang v0.9.3 // indirect
	github.com/prometheus/prom2json v1.2.1 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.4 // indirect
	github.com/uber/jaeger-client-go v2.16.0+incompatible // indirect
	github.com/uber/jaeger-lib v2.0.0+incompatible // indirect
	github.com/yl2chen/cidranger v0.0.0-20180214081945-928b519e5268 // indirect
	go.opencensus.io v0.22.0 // indirect
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.10.0 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/appengine v1.4.0 // indirect
	google.golang.org/grpc v1.21.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/d4l3k/messagediff.v1 v1.2.1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	istio.io/api v0.0.0-20190522135727-e29f1a9ce041 // indirect
	istio.io/istio v0.0.0-20190515005051-eec7a74473de // indirect
	k8s.io/api v0.0.0-20190602205700-9b8cae951d65 // indirect
	k8s.io/apimachinery v0.0.0-20190602183612-63a6072eb563 // indirect
	k8s.io/client-go v11.0.0+incompatible // indirect
	k8s.io/utils v0.0.0-20190529001817-6999998975a7 // indirect
)

replace github.com/golang/sync v0.0.0-20190412183630-56d357773e84 => golang.org/x/sync v0.0.0-20190412183630-56d357773e84
