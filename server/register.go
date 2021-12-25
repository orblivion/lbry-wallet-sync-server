package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"orblivion/lbry-id/auth"
	"orblivion/lbry-id/store"
)

type RegisterRequest struct {
	Token    auth.AuthTokenString `json:"token"`
	PubKey   auth.PublicKey       `json:"publicKey"`
	DeviceID string               `json:"deviceId"`
	Email    string               `json:"email"`
}

func (r *RegisterRequest) validate() bool {
	return (r.Token != auth.AuthTokenString("") &&
		r.PubKey != auth.PublicKey("") &&
		r.DeviceID != "" &&
		r.Email != "")
}

func (s *Server) register(w http.ResponseWriter, req *http.Request) {
	var registerRequest RegisterRequest
	if !getPostData(w, req, &registerRequest) {
		return
	}

	if !s.checkAuth(
		w,
		registerRequest.PubKey,
		registerRequest.DeviceID,
		registerRequest.Token,
		auth.ScopeFull,
	) {
		return
	}

	err := s.store.InsertEmail(registerRequest.PubKey, registerRequest.Email)

	if err != nil {
		if err == store.ErrDuplicateEmail || err == store.ErrDuplicateAccount {
			errorJSON(w, http.StatusConflict, "Error registering")
		} else {
			internalServiceErrorJSON(w, err, "Error registering")
		}
		log.Print(err)
		return
	}

	var registerResponse struct{} // no data to respond with, but keep it JSON
	var response []byte
	response, err = json.Marshal(registerResponse)

	if err != nil {
		internalServiceErrorJSON(w, err, "Error generating register response")
		return
	}

	// TODO StatusCreated also for first walletState and/or for get auth token?
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, string(response))
}
