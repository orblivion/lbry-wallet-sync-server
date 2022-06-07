package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"orblivion/lbry-id/auth"
	"orblivion/lbry-id/store"
)

// TODO email verification cycle

type RegisterRequest struct {
	Email    auth.Email    `json:"email"`
	Password auth.Password `json:"password"`
}

func (r *RegisterRequest) validate() bool {
	return r.Email != "" && r.Password != ""
}

func (s *Server) register(w http.ResponseWriter, req *http.Request) {
	var registerRequest RegisterRequest
	if !getPostData(w, req, &registerRequest) {
		return
	}

	err := s.store.CreateAccount(registerRequest.Email, registerRequest.Password)

	if err != nil {
		if err == store.ErrDuplicateEmail || err == store.ErrDuplicateAccount {
			errorJson(w, http.StatusConflict, "Error registering")
		} else {
			internalServiceErrorJson(w, err, "Error registering")
		}
		log.Print(err)
		return
	}

	var registerResponse struct{} // no data to respond with, but keep it JSON
	var response []byte
	response, err = json.Marshal(registerResponse)

	if err != nil {
		internalServiceErrorJson(w, err, "Error generating register response")
		return
	}

	// TODO StatusCreated also for first walletState and/or for get auth token?
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, string(response))
}
