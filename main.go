package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/lambda-lama/webcam-api/handlers"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/gorillamux"
	"github.com/gorilla/mux"

	"go.uber.org/zap"
)

const (
	localServerPort = ":8080"
)

func setupRouter() *mux.Router {
	r := mux.NewRouter()

	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("Not found", r.RequestURI)
		http.Error(w, fmt.Sprintf("Not found: %s", r.RequestURI), http.StatusNotFound)
	})

	r.HandleFunc("/login", handlers.SignUpPost).Methods("POST")
	r.Handle("/", handlers.AuthMiddleware(http.HandlerFunc(handlers.WebcamCreate))).Methods("POST")

	return r
}

func setupHttpRouter() *http.Server {
	return &http.Server{
		Addr:    localServerPort,
		Handler: setupRouter(),
	}
}

func main() {
	logger := zap.Must(zap.NewProduction())
	defer logger.Sync()

	if runtime_api, _ := os.LookupEnv("AWS_LAMBDA_RUNTIME_API"); runtime_api != "" {
		log.Println("Starting up in Lambda Runtime")
		adapter := gorillamux.NewV2(setupRouter())
		lambda.Start(adapter.ProxyWithContext)
	} else {
		log.Println("Starting up on local")
		srv := setupHttpRouter()
		_ = srv.ListenAndServe()
	}
}
