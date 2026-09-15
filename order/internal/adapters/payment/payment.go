package payment

import (
	"context"
	"log"
	"time"

	"github.com/Fulim13/microservices-go/order/internal/application/core/domain"
	"github.com/Fulim13/microservices-proto/golang/payment"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
)

// go get -u github.com/Fulim13/microservices-proto/golang/payment
type Adapter struct {
	payment payment.PaymentClient
}

func NewAdapter(paymentServiceUrl string) (*Adapter, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	opts = append(opts, grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(
		grpc_retry.WithCodes(codes.Unavailable, codes.ResourceExhausted),
		grpc_retry.WithMax(5),
		grpc_retry.WithBackoff(grpc_retry.BackoffLinear(time.Second)))))
	conn, err := grpc.NewClient(paymentServiceUrl, opts...)
	if err != nil {
		return nil, err
	}
	client := payment.NewPaymentClient(conn)
	return &Adapter{payment: client}, nil
}

// Circuit Breaker
// - Connection between services are called circuits
// - If the error rate of an interservice communication reach a threshold value, it will
//   open the circuit (which means connection between two services are closed)
// - Request to the dependent service will fail immediately
// - The circuit will be reset after certain reset timeout, then it will go to half open,
//   if the failure rate is below threshold, it will close the circuit, else open the circuit
// go get -u github.com/sony/gobreaker
func (a *Adapter) Charge(order *domain.Order) error {
	// ctx, _ := context.WithTimeout(context.TODO(), time.Second*3)
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

	paymentResponse, errCreate := cb.Execute(func() (interface{}, error) {
		return a.payment.Create(context.Background(), &payment.CreatePaymentRequest{
			UserId:     order.CustomerID,
			OrderId:    order.ID,
			TotalPrice: order.TotalPrice(),
		})
	})
	if errCreate != nil {
		log.Printf("Failed to create payment. Err: %v", errCreate)
	} else {
		log.Printf("Payment %d is created successfully.", paymentResponse.(*payment.CreatePaymentResponse).PaymentId)
	}

	return errCreate
}
