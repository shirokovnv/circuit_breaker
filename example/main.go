package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/shirokovnv/circuit_breaker"
)

var cb *circuit_breaker.CircuitBreaker

func init() {
	cfg := circuit_breaker.Config{
		Name: "Example Circuit Breaker",
		ReadyToTrip: func(counts circuit_breaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.7
		},
		OnStateChange: func(name string, from, to circuit_breaker.State) {
			fmt.Printf("%s: state changed from %s to %s", name, from, to)
		},
	}

	cb = circuit_breaker.NewCircuitBreaker(cfg)
}

func RandBool() bool {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(2) == 1
}

func main() {
	msg, err := cb.Execute(func() (interface{}, error) {
		if RandBool() {
			return "success", nil
		}

		return nil, errors.New("service error")
	})

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(msg)
}
