# syntax=docker/dockerfile:1
FROM golang:1.27.0-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Amazon DocumentDB requires TLS and isn't signed by a CA in the default
# system trust store, so the app's MongoDB URI points tlsCAFile at this
# bundle (see terraform/environments/dev/docdb.tf). Harmless when talking
# to a non-DocumentDB MongoDB (e.g. local docker-compose), which doesn't
# use TLS and never reads this file.
FROM alpine:3.22 AS ca-bundle
RUN apk add --no-cache curl && \
    curl -fsSL -o /rds-global-bundle.pem https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
COPY --from=ca-bundle /rds-global-bundle.pem /etc/ssl/certs/rds-global-bundle.pem

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/server"]
