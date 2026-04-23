package handlers

import (
	"net/http"
)

type HomeResponse struct {
	Message string `json:"message"`
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		_ = WriteFailure(w, http.StatusMethodNotAllowed, ReasonMethodNotAllowed)
		return
	}

	if err := WriteResult(w, http.StatusOK, HomeResponse{Message: "Data From the Backend!"}); err != nil {
		_ = WriteFailure(w, http.StatusInternalServerError, ReasonInternalError)
	}
}
