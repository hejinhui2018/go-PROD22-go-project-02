FROM golang:1.23.12 AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go test ./... && go vet ./... && CGO_ENABLED=0 go build -trimpath -o /out/fleetforge ./cmd/fleetforge

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/fleetforge /usr/local/bin/fleetforge
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/fleetforge"]
