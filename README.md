# Run Microservice

```sh
# Start PostgreSQL
docker run --name postgres \
  -p 5432:5432 \
  -e POSTGRES_PASSWORD=verysecretpass \
  -e POSTGRES_DB=order \
  -d postgres

# Create payment database
docker exec -it postgres \
  psql -U postgres \
  -c "CREATE DATABASE payment;"

DATA_SOURCE_URL="postgres://postgres:verysecretpass@127.0.0.1:5432/payment?sslmode=disable" \
	APPLICATION_PORT=3001 \
	ENV=development \
	go run cmd/main.go

DATA_SOURCE_URL="postgres://postgres:verysecretpass@127.0.0.1:5432/order?sslmode=disable" \
	APPLICATION_PORT=3000 \
	ENV=development \
	PAYMENT_SERVICE_URL=localhost:3001 \
	go run cmd/main.go

grpcurl \
	-d '{"user_id": 123, "order_items": [{"product_code": "prod", "quantity": 4, "unit_price": 12}]}' \
	-plaintext \
	localhost:3000 \
	Order/Create
```
