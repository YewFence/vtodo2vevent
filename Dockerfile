FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/vtodo2vevent ./cmd/vtodo2vevent

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata && \
	addgroup -S app && \
	adduser -S -G app app

WORKDIR /app

COPY --from=build /out/vtodo2vevent /usr/local/bin/vtodo2vevent
COPY config.example.yaml /app/config.example.yaml

USER app

ENTRYPOINT ["vtodo2vevent"]
CMD ["-config", "/config/config.yaml"]
