FROM golang:1.27 AS builder
WORKDIR /usr/src/app
COPY . .
RUN CGO_ENABLED=0 go build -o order ./cmd

FROM scratch
COPY --from=builder /usr/src/app/order /order
CMD ["/order"]

# docker build -t order .
