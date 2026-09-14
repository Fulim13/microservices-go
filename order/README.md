# Run Postgres with docker

```sh
docker run -p 5432:5432 \
	-e POSTGRES_PASSWORD=verysecretpass \
	-e POSTGRES_DB=order \
	postgres
```

# Run Order Service

```sh
DATA_SOURCE_URL="postgres://postgres:verysecretpass@127.0.0.1:5432/order?sslmode=disable" \
	APPLICATION_PORT=3000 \
	ENV=development \
	go run cmd/main.go
```

# Call to Order Service Create

```sh
grpcurl \
	-d '{"user_id": 123, "order_items": [{"product_code": "prod", "quantity": 4, "unit_price": 12}]}' \
	-plaintext \
	localhost:3000 \
	Order/Create
```
