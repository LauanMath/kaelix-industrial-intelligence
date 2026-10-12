FROM golang:1.26-alpine AS build
WORKDIR /src
COPY packages/contracts packages/contracts
COPY apps/server/go.mod apps/server/go.sum apps/server/
WORKDIR /src/apps/server
RUN go mod download
COPY apps/server .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /api ./cmd/api

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 kaelix
USER kaelix
COPY --from=build /api /usr/local/bin/api
ENV API_ADDR=0.0.0.0:8080
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
