# syntax=docker/dockerfile:1.28
FROM --platform=$BUILDPLATFORM golang:1.27.4 AS build
ENV CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION=dev
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY themes/ ./themes/
COPY web/ ./web/
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  GOOS=$TARGETOS GOARCH=$TARGETARCH \
  go build -trimpath -ldflags="-s -w -X main.Version=${VERSION}" -o /warden-webui ./cmd/warden-webui
RUN install -d -o 65532 -g 0 -m 2770 /outfs/data
FROM gcr.io/distroless/static:nonroot
COPY --from=build /warden-webui /warden-webui
COPY --from=build /outfs/ /
ENV WARDEN_WEBUI__DATABASE=/data/warden-webui.db
USER 65532:0
ENTRYPOINT ["/warden-webui"]
