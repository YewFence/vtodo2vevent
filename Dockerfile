FROM golang:1.24-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/baikal-vtodo-sync ./cmd/baikal-vtodo-sync

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata && \
	addgroup -S app && \
	adduser -S -G app app

WORKDIR /app

COPY --from=build /out/baikal-vtodo-sync /usr/local/bin/baikal-vtodo-sync
COPY config.example.yaml /app/config.example.yaml

USER app

ENTRYPOINT ["baikal-vtodo-sync"]
CMD ["-config", "/config/config.yaml"]
