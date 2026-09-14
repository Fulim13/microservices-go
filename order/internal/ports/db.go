package ports

import (
	"context"

	"github.com/Fulim13/microservices-go/order/internal/application/core/domain"
)

type DBPort interface {
	Get(ctx context.Context, id int64) (domain.Order, error)
	Save(ctx context.Context, order *domain.Order) error
}
