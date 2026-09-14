package ports

import (
	"context"

	"github.com/Fulim13/microservices-go/order/internal/application/core/domain"
)

type APIPort interface {
	PlaceOrder(ctx context.Context, order domain.Order) (domain.Order, error)
}
