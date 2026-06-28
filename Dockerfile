FROM golang:1.25.11 AS builder

WORKDIR /usr/src/app

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -v -o /usr/local/bin/app
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -v -o /usr/local/bin/migrate ./cmd/migrate/

FROM gcr.io/distroless/static-debian12

COPY --from=builder /usr/local/bin/app /usr/local/bin/app
COPY --from=builder /usr/local/bin/migrate /usr/local/bin/migrate

ADD https://cockroachlabs.cloud/clusters/0de3351e-57c1-4910-836d-5504d3dae7fc/cert /root.crt

EXPOSE 8080

CMD ["/usr/local/bin/app"]
