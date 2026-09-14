package api

import (
	"context"

	"github.com/Fulim13/microservices-go/order/internal/application/core/domain"
	"github.com/Fulim13/microservices-go/order/internal/ports"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		st, _ := status.FromError(paymentErr)
		fieldErr := &errdetails.BadRequest_FieldViolation{
			Field:       "payment",
			Description: st.Message(),
		}
		badReq := &errdetails.BadRequest{}
		badReq.FieldViolations = append(badReq.FieldViolations, fieldErr)
		orderStatus := status.New(codes.InvalidArgument, "order creation failed")
		statusWithDetails, _ := orderStatus.WithDetails(badReq)
		return domain.Order{}, statusWithDetails.Err()
	}
	return order, nil
}
