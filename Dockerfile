FROM alpine:3.19.0

WORKDIR /app

ENV DOCKER=1

COPY rdio-scanner ./

RUN mkdir -p /app/data && \
    apk --no-cache --no-progress add ffmpeg mailcap tzdata

VOLUME [ "/app/data" ]

EXPOSE 3000

ENTRYPOINT [ "./rdio-scanner", "-base_dir", "/app/data" ]
