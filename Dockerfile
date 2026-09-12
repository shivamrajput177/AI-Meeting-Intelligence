# Shared multi-stage build for every Go service in this repo — one
# Dockerfile, parameterized by which cmd/<service> to build, rather than
# five near-identical copies. Build with:
#   docker build --build-arg SERVICE=auth-service -t auth-service .
FROM golang:1.25-bookworm AS build
ARG SERVICE
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/server ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
