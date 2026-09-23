FROM golang:1.27 AS build

WORKDIR /src

# Dependencies first, so editing source does not re-download the module cache.
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY api ./api

# CGO off makes the binary static, which is what lets the final stage be an
# image with no libc at all. -trimpath keeps build paths out of the binary.
# ./cmd/... builds every binary, so the sweeper ships in the same image as the
# API and cannot drift a version behind it.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/...

FROM gcr.io/distroless/static-debian12:nonroot

# Venue.Location() resolves an IANA name at runtime, and distroless carries no
# timezone database. Without this every venue falls back to UTC — silently,
# because LoadLocation's error is swallowed — and courts open at the wrong hour.
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo

COPY --from=build /out/ /

# Cloud Run sends $PORT; 8080 is both its default and the server's.
EXPOSE 8080

# Numeric, not the "nonroot" name: Kubernetes cannot verify a named user is
# non-root and refuses to start the container under runAsNonRoot.
USER 65532:65532

ENTRYPOINT ["/deuce"]
