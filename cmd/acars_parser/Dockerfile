FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# The ICAO Doc 8643 type list is not in the repository (ICAO's data); it is
# fetched from ICAO and embedded at build time, so the build needs network
# access to doc8643.icao.int.
RUN go generate ./internal/aircrafttype && \
    CGO_ENABLED=0 go build -buildvcs=false -o /acars_parser ./cmd/acars_parser

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /acars_parser /usr/local/bin/acars_parser
ENTRYPOINT ["acars_parser"]
