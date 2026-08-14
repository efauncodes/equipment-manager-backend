FROM golang:1.19-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/equipment-manager-backend ./cmd/server

FROM alpine:3.19

RUN addgroup -S app && adduser -S -G app app

COPY --from=build /out/equipment-manager-backend /usr/local/bin/equipment-manager-backend

USER app
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/equipment-manager-backend"]
