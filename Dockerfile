FROM alpine:3.20.0

WORKDIR /app

ENV DOCKER=1

# renovate: datasource=repology depName=alpine_3_19/ca-certificates
ARG CA_CERTIFICATES_VERSION=20230506-r0
# renovate: datasource=repology depName=alpine_3_19/ffmpeg
ARG FFMPEG_VERSION=6.1.1-r0
# renovate: datasource=repology depName=alpine_3_19/tzdata
ARG TZDATA_VERSION=2024a-r0

RUN apk add --no-cache \
    ca-certificates="${CA_CERTIFICATES_VERSION}" \
    ffmpeg="${FFMPEG_VERSION}" \
    tzdata="${TZDATA_VERSION}"

COPY rdio-scanner ./

RUN mkdir -p /app/data

VOLUME [ "/app/data" ]

EXPOSE 3000

ENTRYPOINT [ "./rdio-scanner", "-base_dir", "/app/data" ]
