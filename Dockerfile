# syntax=docker/dockerfile:1

FROM alpine:3.21 AS runtime

LABEL org.opencontainers.image.source="https://github.com/tranthethang/docker-shared-services-ui"

WORKDIR /app

RUN --mount=type=cache,target=/etc/apk/cache,sharing=locked \
    apk add --no-cache docker-cli docker-cli-compose

COPY --chmod=0755 bin/dashboard-linux /usr/local/bin/dashboard

EXPOSE 8787

ENTRYPOINT ["/usr/local/bin/dashboard"]
CMD ["--addr", "0.0.0.0:8787", "--no-open"]
