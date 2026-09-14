package ports

import "github.com/Fulim13/microservices-go/order/internal/application/core/domain"

type PaymentPort interface {
	Charge(order *domain.Order) error
}
