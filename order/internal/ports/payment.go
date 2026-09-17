package ports

import (
	"context"

	"github.com/Fulim13/microservices-go/order/internal/application/core/domain"
)

type PaymentPort interface {
	Charge(ctx context.Context, order *domain.Order) error
}
