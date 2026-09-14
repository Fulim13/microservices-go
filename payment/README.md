# Run Postgres with docker

```sh
docker run -p 5432:5432 \
	-e POSTGRES_PASSWORD=verysecretpass \
	-e POSTGRES_DB=payment \
	postgres
```

# Run Payment Service

```sh
DATA_SOURCE_URL="postgres://postgres:verysecretpass@127.0.0.1:5432/payment?sslmode=disable" \
	APPLICATION_PORT=3001 \
	ENV=development \
	go run cmd/main.go
```

# Call to Payment Service Create

```sh
grpcurl -d '{"user_id": 123, "order_id":12, "total_price": 32}' -plaintext localhost:3001 Payment/Create
```
