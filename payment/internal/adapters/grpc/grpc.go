package grpc

import (
	"context"
	"fmt"

	"github.com/Fulim13/microservices-go/payment/internal/application/core/domain"
	"github.com/Fulim13/microservices-proto/golang/payment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (a Adapter) Create(ctx context.Context, request *payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	// time.Sleep(4 * time.Second)
	// fmt.Println("In Payment service")
	// return nil, status.New(codes.Unavailable, fmt.Sprint("payment service unavailable.")).Err()
	newPayment := domain.NewPayment(request.UserId, request.OrderId, request.TotalPrice)
	result, err := a.api.Charge(ctx, newPayment)
	if err != nil {
		return nil, status.New(codes.Internal, fmt.Sprintf("failed to charge. %v ", err)).Err()
	}
	return &payment.CreatePaymentResponse{PaymentId: result.ID}, nil
}
