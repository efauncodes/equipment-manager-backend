FROM golang:1.19-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/equipment-manager-backend ./cmd/server

FROM alpine:3.19
RUN addgroup -S app && adduser -S -G app app && mkdir -p /app/data && chown -R app:app /app/data
COPY --from=build /out/equipment-manager-backend /usr/local/bin/equipment-manager-backend
USER app
VOLUME ["/app/data"]
ENV EQUIPMENT_DB_PATH=/app/data/equipment.db
ENTRYPOINT ["/usr/local/bin/equipment-manager-backend"]
