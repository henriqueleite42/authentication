package http

import (
	"net/http"
	"os"

	"github.com/econominhas/authentication/internal/delivery"
	"github.com/econominhas/authentication/internal/models"
)

type HttpDelivery struct {
	server    *http.Server
	router    *http.ServeMux
	validator delivery.Validator

	accountService models.AccountService
}

type NewHttpDeliveryInput struct {
	AccountService models.AccountService
}

func (dlv *HttpDelivery) Listen() {
	dlv.AuthController()

	dlv.server.ListenAndServe()
}

func NewHttpDelivery(i *NewHttpDeliveryInput) *HttpDelivery {
	router := http.NewServeMux()

	server := &http.Server{
		Addr:    ":" + os.Getenv("PORT"),
		Handler: router,
	}

	return &HttpDelivery{
		server:    server,
		router:    router,
		validator: delivery.NewValidator(),

		accountService: i.AccountService,
	}
}
