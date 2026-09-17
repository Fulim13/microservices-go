package payment

import (
	"context"
	"log"

	"github.com/Fulim13/microservices-go/order/internal/application/core/domain"
	"github.com/Fulim13/microservices-proto/golang/payment"
	"github.com/sony/gobreaker"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
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
	opts = append(opts, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
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

// Circuit Breaker
//   - Connection between services are called circuits
//   - If the error rate of an interservice communication reach a threshold value, it will
//     open the circuit (which means connection between two services are closed)
//   - Request to the dependent service will fail immediately
//   - The circuit will be reset after certain reset timeout, then it will go to half open,
//     if the failure rate is below threshold, it will close the circuit, else open the circuit
//
// go get -u github.com/sony/gobreaker
func (a *Adapter) Charge(ctx context.Context, order *domain.Order) error {
	_, err := a.payment.Create(ctx, &payment.CreatePaymentRequest{
		UserId:     order.CustomerID,
		OrderId:    order.ID,
		TotalPrice: order.TotalPrice(),
	})
	return err
}
