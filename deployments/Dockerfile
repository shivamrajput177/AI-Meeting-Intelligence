# Shared multi-stage build for every Go service in this repo — one
# Dockerfile, parameterized by which services/<service> to build, rather
# than five near-identical copies. The build context is the repo root
# (not this deployments/ directory) so go.work can see every module —
# see docker-compose.yaml's `context: .. / dockerfile: deployments/Dockerfile`.
# Build with:
#   docker build -f deployments/Dockerfile --build-arg SERVICE=auth-service -t auth-service .
FROM golang:1.25-bookworm AS build
ARG SERVICE
WORKDIR /src

# Go workspaces resolve modules by directory, so there's no equivalent of
# copying just go.mod/go.sum to warm the module cache before the rest of
# the source — copy everything up front.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/server ./services/${SERVICE}/cmd

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/server /server
EXPOSE 8080
# Each service's config.json is bind-mounted to
# deployments/configs/<service>.json (relative to this WORKDIR) — see
# docker-compose.yaml.
ENTRYPOINT ["/server"]
