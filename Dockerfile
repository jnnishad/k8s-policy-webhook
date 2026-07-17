# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/webhook ./cmd/webhook

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/webhook /webhook
USER nonroot:nonroot
ENTRYPOINT ["/webhook"]
