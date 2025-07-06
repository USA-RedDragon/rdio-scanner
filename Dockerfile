FROM alpine:3.22.0@sha256:8a1f59ffb675680d47db6337b49d22281a139e9d709335b492be023728e11715

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
