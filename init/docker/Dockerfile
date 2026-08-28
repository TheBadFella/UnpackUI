FROM golang:1.27-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY main.go settings.sh ./
COPY pkg pkg
COPY examples examples
RUN go generate ./...

ARG TARGETOS
ARG TARGETARCH
ARG BUILD_DATE
ARG BRANCH
ARG VERSION
ARG ITERATION
ARG COMMIT

ENV GOFLAGS="-trimpath -mod=readonly -modcacherw"
ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o /tmp/unpackerr -tags osusergo,netgo \
    -ldflags "-w -s -X \"golift.io/version.Branch=${BRANCH} (${COMMIT})\" \
    -X \"golift.io/version.BuildDate=${BUILD_DATE}\" \
    -X \"golift.io/version.BuildUser=docker\" \
    -X \"golift.io/version.Revision=${ITERATION}\" \
    -X \"golift.io/version.Version=${VERSION}\""

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

ARG TARGETOS
ARG TARGETARCH
ARG BUILD_DATE
ARG COMMIT
ARG VERSION
ARG ITERATION
ARG LICENSE=MIT
ARG SOURCE_URL=http://github.com/golift/application-builder
ARG DESC=application-builder
ARG VENDOR=golift
ARG AUTHOR=golift
# Build-time metadata as defined at https://github.com/opencontainers/image-spec/blob/master/annotations.md
LABEL org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.title="Unpackerr" \
      org.opencontainers.image.documentation="https://unpackerr.zip" \
      org.opencontainers.image.description="${DESC}" \
      org.opencontainers.image.url="${SOURCE_URL}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.source="${SOURCE_URL}" \
      org.opencontainers.image.vendor="${VENDOR}" \
      org.opencontainers.image.authors="${AUTHOR}" \
      org.opencontainers.image.architecture="${TARGETOS} ${TARGETARCH}" \
      org.opencontainers.image.licenses="${LICENSE}" \
      org.opencontainers.image.version="${VERSION}-${ITERATION}"

COPY --from=builder /tmp/unpackerr /unpackerr
# Make sure we have an ssl cert chain and timezone data.
RUN apk add --no-cache ca-certificates openssl tzdata

ENV TZ=UTC

ENTRYPOINT ["/unpackerr"]
