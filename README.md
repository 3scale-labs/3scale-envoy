# 3scale-envoy

*This is a PoC, it is not supported.* 

## What is this? 

3scale-envoy is a simple Control Plane for Envoy that allows using an Envoy proxy as an API Gateway with the 3scale API Management solution.  

The Control Plane takes care of: 

* Config translation: Translates the 3scale configuration to an Envoy configuration.
* Authorization: Exposes an Envoy External Auth endpoint that abstracts 3scale Service Management APIs used for authentication and authorization. 


## How it works?

3scale-envoy fetches the Proxy configuration from the 3scale Account Management API and translates that configuration
into Envoy xDS objects.
 
Envoy requests a new config from 3scale-envoy using the xDS API, and "hot reloads" itself with the latest configuration. 

When Envoy processes an API request, it gets authorized by the External Authorization Service that is exposed
by 3scale-envoy (It follows the [External authorization HTTP Filter](https://www.envoyproxy.io/docs/envoy/latest/configuration/http_filters/ext_authz_filter) implementation).

Depending on the Request PATH, Method, and query parameters, and the rules and rate-limits configured in 3scale, the request is allowed or not. 

3scale gathers analytics data about requests made that can be viewed in its Admin Portal UI.

```
                                 ┌────────────────┐               ┌───────────────────┐
                                 │                │               │                   │
                                 │                │               │                   │
   API Requests   ──────────────▶│     Envoy      │──────────────▶│    API Backend    │
                                 │                │               │                   │
                                 │                │               │                   │
                                 └────────────────┘               └───────────────────┘
                                    │           │                                      
                                 xDS API   ExtAuthZ API                                
                                  (gRPC)      (gRPC)                                   
                                    │           │                                      
                                    ▼           ▼                                      
                                ┌──────────────────┐                                   
                                │                  │                                   
                         ┌─────▶│   3scale-envoy   │──────┐                             
                         │      │                  │      │                             
                         │      └──────────────────┘      │                             
                Proxy    │                                │ Authorization               
            configuration│                                │ and reporting
                         │                                │    request               
                         │                                │                             
                         │                                ▼                             
           ┌───────────────────────────┐   ┌───────────────────────────┐               
           │                           │   │                           │               
           │                           │   │                           │               
           │       3scale Account      │   │       3scale Service      │               
           │       Management API      │   │       Management API      │               
           │                           │   │                           │               
           └───────────────────────────┘   └───────────────────────────┘               
```


## Getting started

3scale-envoy is design to work together with a 3scale account, either SaaS or on-premises. 
The service you want to expose will need to be configured with the "Apicast Self-managed" integration method.
The "Production Public Base URL", and "Private Base URL" values should be set and the configuration
should be promoted to the production environment in 3scale.

Additionally 3scale-envoy requires the following information:

* **3scale Admin URL**: The admin portal of your tenant, for ex "https://mytenant-admin.3scale.net:443/"
* **ServiceID**: The Service ID of the API in 3scale you wish to expose via Envoy.
* **AccessToken**: An AccessToken with enough permissions to read the 3scale proxy config.

### Build: 

In your local machine, Golang > 1.12 is required : 

```bash
go build 3scale-envoy
```

Using the Dockerfile:

```bash 
docker build .
```

### Run

3scale-envoy exposes two ports by default: tcp/18000 (xDS) and tcp/9090 (ExtAuthZ). Those need to be appropriately
forwarded if containerized. Those ports need to be accessible from the Envoy server.
If the 3scale-envoy is running in a different instance than the Envoy gateway, the `HOSTNAME` var needs to be adjusted accordingly.

Using the binary: 

```bash
HOSTNAME="127.0.0.1" \
ACCESS_TOKEN="XXXXXXXXXXXXXXXXXXXXXXXXXX" \ 
3SCALE_ADMIN_URL="https://yourtenant-admin.3scale.net:443/" \
SERVICE_ID="9999999999" ./3scale-envoy
```

Using the Docker image:

```bash
docker run -p 9090:9090 -p18000:18000 \
    -e HOSTNAME="127.0.0.1" \
    -e ACCESS_TOKEN="XXXXXXXXXXXXXXXXXXXXXXXXXX" \
    -e 3SCALE_ADMIN_URL="https://yourtenant-admin.3scale.net:443/" \
    -e SERVICE_ID="9999999999" \
    -ti 3scale-envoy
```

You can get help by running `3scale-envoy --help`:

```bash
usage: 3scale-envoy --hostname=HOSTNAME --access_token=ACCESS_TOKEN --3scale_admin_url=3SCALE_ADMIN_URL --service_id=SERVICE_ID [<flags>]

Flags:
  --help                        Show context-sensitive help (also try --help-long and --help-man).
  --hostname=HOSTNAME           The hostname or address used by Envoy to reach this control plane.
  --access_token=ACCESS_TOKEN   Your 3scale admin portal access token.
  --3scale_admin_url=3SCALE_ADMIN_URL
                                The URL of your 3scale Admin portal: "https://tenant-admin.3scale.net:443/".
  --service_id=SERVICE_ID       The Service ID from 3scale to be used.
  --public_port=10000           Gateway Public port, for external traffic.
  --xds_port=18000              xDS server, this is where Envoy should connect to get the configuration.
  --admin_enabled               Enable the admin endpoint in Envoy. (true or false)
  --admin_http_port=19001       Envoy HTTP admin endpoint port.
  --auth_port=9090              External AuthZ service port.
  --cache_ttl=1m                Porta Cache time to wait before purging expired items from the cache.
  --cache_refresh_interval=30s  Porta cache time difference to refresh the cache element before expiry time.
  --cache_entries_max=1000      Porta cache max number of items that can be stored in the cache at any time.
  --cache_update_retries=2      Porta Cache number of additional attempts made to update cached entries for unreachable hosts.
```

## Envoy bootstrap configuration

This project provides a basic config for bootstrapping an Envoy gateway. 

### local envoy: 

First start 3scale-envoy, then run Envoy with the example configuration:

```bash
envoy -c envoyproxy/envoy:latest 
```

### Containerized Envoy: 

We need to export the port for external requests, `tcp/10000`

```bash
docker run -p 10000:10000 \ 
    -v $(pwd)/example/envoy-bootstrap.yaml:/tmp/envoy-bootstrap.yaml \
    -ti envoyproxy/envoy:latest -c /tmp/envoy-bootstrap.yaml
```

### If you are running macOS with docker desktop:

You will need to modify the address in the envoy-bootstrap.yaml file,
find this part:

```yaml
static_resources:
  clusters:
    - connect_timeout: 1s
      hosts:
        - socket_address:
            address: "127.0.0.1"
            port_value: 18000
      http2_protocol_options: {}
      name: xds_cluster
      type: STRICT_DNS
```

And modify the address value to `host.docker.internal`:

```yaml
static_resources:
  clusters:
    - connect_timeout: 1s
      hosts:
        - socket_address:
            address: "host.docker.internal"
            port_value: 18000
      http2_protocol_options: {}
      name: xds_cluster
      type: STRICT_DNS
```

And you will need to start `3scale-envoy` with the `HOSTNAME` value set to `host.docker.internal`

## Making a request.

By default, the control plane will configure Envoy to expose the port `tcp/10000` for external requests. 
I configured my API production endpoint to be `production.local`. 

We need to set the `Host` header accordingly: 

```bash
curl -v -H "Host: production.local" http://127.0.0.1:10000/test\?user_key\=YOUR_USER_KEY 
```
 
If the `user_key` or `app_key` value is correct, you should get a `200` response and the response from your API Backend.

## Limitations & Known issues

As this is a PoC, there's missing support for:

* Limited authentication options, no support for OAuth2 or OIDC.
* Changing the Authz query parameters name from 3scale. 
* Policy support, working on the WASM support. 
* Missing documentation, tests... 

## Request improvements, new features

Feel free to open Issues in this repo or even better, open a PR ;) 
