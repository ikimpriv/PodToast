# Build from the PodToast repository root. A static Linux binary needs no shell,
# libc or package manager in either the agent image or the target container.
FROM --platform=$BUILDPLATFORM golang:1.25 AS build
ARG TARGETOS=linux
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
COPY LICENSE /licenses/MIT.txt
RUN go mod download golang.org/x/net golang.org/x/text \
 && cp /usr/local/go/LICENSE /licenses/Go.txt \
 && cp /go/pkg/mod/golang.org/x/net@*/LICENSE /licenses/x-net.txt \
 && cp /go/pkg/mod/golang.org/x/text@*/LICENSE /licenses/x-text.txt
COPY internal/podtoast ./internal/podtoast
COPY cmd/podtoast ./cmd/podtoast
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o /podtoast ./cmd/podtoast
FROM scratch
LABEL org.opencontainers.image.source="https://github.com/ikimpriv/PodToast" \
      org.opencontainers.image.title="PodToast" \
      org.opencontainers.image.description="Shell-free Kubernetes ephemeral-container diagnostics for FarshMac" \
      org.opencontainers.image.licenses="MIT"
COPY --from=build /podtoast /podtoast
COPY --from=build /licenses /licenses
ENTRYPOINT ["/podtoast"]
CMD ["idle"]
