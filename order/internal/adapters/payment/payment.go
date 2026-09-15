package payment

import (
	"context"
	"log"

	"github.com/Fulim13/microservices-go/order/internal/application/core/domain"
	"github.com/Fulim13/microservices-proto/golang/payment"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// go get -u github.com/Fulim13/microservices-proto/golang/payment
type Adapter struct {
	payment payment.PaymentClient
}

func CircuitBreakerClientInterceptor(cb *gobreaker.CircuitBreaker) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		_, cbErr := cb.Execute(func() (interface{}, error) {
			err := invoker(ctx, method, req, reply, cc, opts...)
			if err != nil {
				return nil, err
			}

			return nil, nil
		})
		return cbErr
	}
}

func NewAdapter(paymentServiceUrl string) (*Adapter, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "cbPayment", // Unique name of circuit breaker
		MaxRequests: 0,           // Allowed number of requests for half open circult
		Timeout:     4,           // Timeout for an open to half-open transition
		ReadyToTrip: func(counts gobreaker.Counts) bool { // Decide if the circuit will be open
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRatio >= 0.6
		},
		OnStateChange: func(name string, from, to gobreaker.State) { // Execute on each state changes
			log.Printf("Circuit Breaker: %s, changed from %v, to %v", name, from, to)
		},
	})
	opts = append(opts, grpc.WithUnaryInterceptor(CircuitBreakerClientInterceptor(cb)))
	conn, err := grpc.NewClient(paymentServiceUrl, opts...)
	if err != nil {
		return nil, err
	}
	client := payment.NewPaymentClient(conn)
	return &Adapter{payment: client}, nil
}

// go get -u github.com/sony/gobreaker
func (a *Adapter) Charge(order *domain.Order) error {
	_, err := a.payment.Create(context.Background(), &payment.CreatePaymentRequest{
		UserId:     order.CustomerID,
		OrderId:    order.ID,
		TotalPrice: order.TotalPrice(),
	})
	return err
}
