ARG BASE_IMAGE=registry.access.redhat.com/ubi9-micro:latest@sha256:f332c99eb8f798a8486821c91937f10ad64ee83d7e739303be2df051040918f6

FROM registry.access.redhat.com/ubi9/go-toolset:1.26@sha256:5e68f09a652ac6627a83c57655e42e24575efb278b54336039c9308607fc6b21 AS build
WORKDIR /opt/app-root/src
COPY --chown=1001:0 go.mod go.sum ./
RUN go mod download
COPY --chown=1001:0 . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /tmp/func .

# ubi9-micro doesn't include CA certificates; copy from builder for TLS to the
# OCI API endpoints. The binary is static (CGO_ENABLED=0) and fdk-go embeds the
# OCI Functions invocation protocol directly, so no Go toolchain is needed at
# runtime.
FROM ${BASE_IMAGE}
WORKDIR /function
COPY --from=build /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem
COPY --from=build /tmp/func /function/func
USER 65532:65532
ENTRYPOINT ["./func"]
