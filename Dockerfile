FROM alpine:3.21.3@sha256:a8560b36e8b8210634f77d9f7f9efd7ffa463e380b75e2e74aff4511df3ef88c

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
