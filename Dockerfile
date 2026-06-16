FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

WORKDIR /app

ENV DOCKER=1

# renovate: datasource=repology depName=alpine_3_21/ca-certificates
ARG CA_CERTIFICATES_VERSION=20241121-r1
# renovate: datasource=repology depName=alpine_3_21/ffmpeg
ARG FFMPEG_VERSION=6.1.2-r1
# renovate: datasource=repology depName=alpine_3_21/tzdata
ARG TZDATA_VERSION=2025b-r0

RUN apk add --no-cache \
    ca-certificates="${CA_CERTIFICATES_VERSION}" \
    ffmpeg="${FFMPEG_VERSION}" \
    tzdata="${TZDATA_VERSION}"

COPY rdio-scanner ./

RUN mkdir -p /app/data

VOLUME [ "/app/data" ]

EXPOSE 3000

ENTRYPOINT [ "./rdio-scanner", "-base_dir", "/app/data" ]
