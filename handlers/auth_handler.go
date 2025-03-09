package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/lambda-lama/webcam-api/config"
)

type Webcam struct {
	Name string
}

type Claims struct {
	Name string `json:"name"`
	jwt.RegisteredClaims
}

type contextKey string

const contextNameKey contextKey = "name"

var secretKey = []byte(config.SecretKey)

func SignUpPost(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var webcam Webcam
	err := dec.Decode(&webcam)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			sendError(w, http.StatusBadRequest, msg)

		case errors.Is(err, io.ErrUnexpectedEOF):
			msg := "Request body contains badly-formed JSON"
			sendError(w, http.StatusBadRequest, msg)

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			sendError(w, http.StatusBadRequest, msg)

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			sendError(w, http.StatusBadRequest, msg)

		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			sendError(w, http.StatusBadRequest, msg)

		case err.Error() == "http: request body too large":
			msg := "Request body must not be larger than 1MB"
			sendError(w, http.StatusRequestEntityTooLarge, msg)

		default:
			log.Print(err.Error())
			sendError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}
		return
	}
	token := generateToken(webcam.Name)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"api_key": token})
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("Authorization")

		if apiKey == "" {
			sendError(w, http.StatusUnauthorized, "API key is required")
			return
		}

		token, err := jwt.ParseWithClaims(apiKey, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return secretKey, nil
		})

		if err != nil {
			sendError(w, http.StatusUnauthorized, "Invalid API key")
			return
		}

		if !token.Valid {
			sendError(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			sendError(w, http.StatusUnauthorized, "Invalid token claims")
			return
		}

		ctx := context.WithValue(r.Context(), contextNameKey, claims.Name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func generateToken(name string) string {
	exp := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Name: name,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, _ := token.SignedString(secretKey)
	return signedToken
}
