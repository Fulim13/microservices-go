package grpc

import (
	"context"
	"fmt"

	"github.com/Fulim13/microservices-go/payment/internal/application/core/domain"
	"github.com/Fulim13/microservices-proto/golang/payment"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// To try
//
//	grpcurl -d '{"user_id": -1, "order_items": [{"product_code": "prod", "quantity": 4, "unit_price": -1}]}' -plaintext localhost:3000 Order/Create
func (a Adapter) Create(ctx context.Context, request *payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	// time.Sleep(4 * time.Second)
	fmt.Println("In Payment service")
	// return nil, status.New(codes.Unavailable, fmt.Sprint("payment service unavailable.")).Err()

	var validationErrors []*errdetails.BadRequest_FieldViolation

	if request.UserId < 1 {
		validationErrors = append(validationErrors, &errdetails.BadRequest_FieldViolation{
			Field:       "user_id",
			Description: "user_id cannot be less than 1",
		})
	}

	if request.OrderId < 1 {
		validationErrors = append(validationErrors, &errdetails.BadRequest_FieldViolation{
			Field:       "order_id",
			Description: "order_id cannot be less than 1",
		})
	}

	if request.TotalPrice < 0 {
		validationErrors = append(validationErrors, &errdetails.BadRequest_FieldViolation{
			Field:       "total_price",
			Description: "total_price cannot be less than 0",
		})
	}

	if len(validationErrors) > 0 {
		stat := status.New(400, "invalid payment request")
		badRequest := &errdetails.BadRequest{}
		badRequest.FieldViolations = validationErrors
		s, _ := stat.WithDetails(badRequest)
		return nil, s.Err()
	}

	newPayment := domain.NewPayment(request.UserId, request.OrderId, request.TotalPrice)
	result, err := a.api.Charge(ctx, newPayment)
	if err != nil {
		return nil, status.New(codes.Internal, fmt.Sprintf("failed to charge. %v ", err)).Err()
	}
	return &payment.CreatePaymentResponse{PaymentId: result.ID}, nil
}
