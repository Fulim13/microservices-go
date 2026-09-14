package api

import (
	"context"

	"github.com/Fulim13/microservices-go/order/internal/application/core/domain"
	"github.com/Fulim13/microservices-go/order/internal/ports"
)

type Application struct {
	db      ports.DBPort
	payment ports.PaymentPort
}

func NewApplication(db ports.DBPort, payment ports.PaymentPort) *Application {
	return &Application{db: db, payment: payment}
}

func (a Application) PlaceOrder(ctx context.Context, order domain.Order) (domain.Order, error) {
	err := a.db.Save(ctx, &order)
	if err != nil {
		return domain.Order{}, nil
	}
	paymentErr := a.payment.Charge(&order)
	if paymentErr != nil {
		return domain.Order{}, nil
	}
	return order, nil
}
