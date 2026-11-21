# ---- build ----
FROM golang:1.27-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ---- run ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/api /app/api
COPY configs /app/configs
RUN chown -R app:app /app
USER app
EXPOSE 8080
ENTRYPOINT ["/app/api", "-config", "/app/configs/config.yaml"]
