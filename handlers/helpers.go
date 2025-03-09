package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Response struct {
	Result string `json:"result"`
	Error  string `json:"error"`
}

func sendError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := Response{
		Result: "",
		Error:  message,
	}

	json.NewEncoder(w).Encode(errorResponse)
}

func sendResponse(w http.ResponseWriter, statusCode int, responseData interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := Response{
		Result: fmt.Sprintf("%v", responseData),
		Error:  "",
	}

	json.NewEncoder(w).Encode(response)
}
