FROM golang:1.25 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go install go.opentelemetry.io/collector/cmd/builder@v0.146.1
RUN builder --config /src/build/otelcol-builder.yaml

FROM gcr.io/distroless/base-debian12

COPY --from=builder /src/_build/otelcol /otelcol
COPY build/otelcol-config.yaml /etc/otelcol/config.yaml

EXPOSE 4317 4318 9411 13133 18080

ENTRYPOINT ["/otelcol"]
CMD ["--config=/etc/otelcol/config.yaml"]
