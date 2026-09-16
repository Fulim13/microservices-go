FROM golang:1.27 AS builder
WORKDIR /usr/src/app
COPY . .
RUN CGO_ENABLED=0 go build -o payment ./cmd

FROM scratch
COPY --from=builder /usr/src/app/payment /payment
CMD ["/payment"]

# docker build -t payment .
