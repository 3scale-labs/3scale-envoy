#### Builder
FROM golang:1.12 as builder


WORKDIR /tmp/3scale-envoy

COPY . .

RUN go build 3scale-envoy

#### Runtime
FROM registry.access.redhat.com/ubi8/ubi-minimal

ENV HOSTNAME ACCESS_TOKEN 3SCALE_ADMIN_URL SERVICE_ID

WORKDIR /root/

COPY --from=builder /tmp/3scale-envoy/3scale-envoy .

EXPOSE 8080

ENTRYPOINT ["./3scale-envoy"] 
