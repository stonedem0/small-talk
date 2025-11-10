# App server container
FROM golang:1.23 as builder
WORKDIR /src
COPY . .
# Build the app server binary
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd apps/app/server && go build -o /out/small-talk-app

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /out/small-talk-app /usr/local/bin/small-talk-app
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/small-talk-app"]


