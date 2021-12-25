package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"orblivion/lbry-id/auth"
	"orblivion/lbry-id/store"
	"orblivion/lbry-id/wallet"
)

type WalletStateRequest struct {
	Token     auth.AuthTokenString `json:"token"`
	BodyJSON  string               `json:"bodyJSON"`
	PubKey    auth.PublicKey       `json:"publicKey"`
	Signature auth.Signature       `json:"signature"`

	// downloadKey is derived from the same password used to encrypt the wallet.
	// We want to keep it all in sync so we update it at the same time.
	DownloadKey auth.DownloadKey `json:"downloadKey"`
}

func (r *WalletStateRequest) validate() bool {
	return (r.Token != auth.AuthTokenString("") &&
		r.BodyJSON != "" &&
		r.PubKey != auth.PublicKey("") &&
		r.Signature != auth.Signature(""))
}

type WalletStateResponse struct {
	BodyJSON  string         `json:"bodyJSON"`
	Signature auth.Signature `json:"signature"`
	Error     string         `json:"error"` // in case of 409 Conflict responses. TODO - make field not show up if it's empty, to avoid confusion
}

func (s *Server) handleWalletState(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet {
		s.getWalletState(w, req)
	} else if req.Method == http.MethodPost {
		s.postWalletState(w, req)
	} else {
		errorJSON(w, http.StatusMethodNotAllowed, "")
	}
}

// TODO - There's probably a struct-based solution here like with POST/PUT.
// We could put that struct up top as well.
func getWalletStateParams(req *http.Request) (pubKey auth.PublicKey, deviceId string, token auth.AuthTokenString, err error) {
	tokenSlice, hasTokenSlice := req.URL.Query()["token"]
	deviceIDSlice, hasDeviceId := req.URL.Query()["deviceId"]
	pubKeySlice, hasPubKey := req.URL.Query()["publicKey"]

	if !hasDeviceId {
		err = fmt.Errorf("Missing deviceId parameter")
	}
	if !hasTokenSlice {
		err = fmt.Errorf("Missing token parameter")
	}
	if !hasPubKey {
		err = fmt.Errorf("Missing publicKey parameter")
	}

	if err == nil {
		deviceId = deviceIDSlice[0]
		token = auth.AuthTokenString(tokenSlice[0])
		pubKey = auth.PublicKey(pubKeySlice[0])
	}

	return
}

func (s *Server) getWalletState(w http.ResponseWriter, req *http.Request) {

	if !getGetData(w, req) {
		return
	}

	pubKey, deviceId, token, err := getWalletStateParams(req)

	if err != nil {
		errorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	if !s.checkAuth(w, pubKey, deviceId, token, auth.ScopeGetWalletState) {
		return
	}

	latestWalletStateJSON, latestSignature, err := s.store.GetWalletState(pubKey)

	var response []byte

	if err == store.ErrNoWalletState {
		errorJSON(w, http.StatusNotFound, "No wallet state")
		return
	} else if err != nil {
		internalServiceErrorJSON(w, err, "Error retrieving walletState")
		return
	}

	walletResponse := WalletStateResponse{
		BodyJSON:  latestWalletStateJSON,
		Signature: latestSignature,
	}
	response, err = json.Marshal(walletResponse)

	if err != nil {
		internalServiceErrorJSON(w, err, "Error generating latestWalletState response")
		return
	}

	fmt.Fprintf(w, string(response))
}

func (s *Server) postWalletState(w http.ResponseWriter, req *http.Request) {
	var walletStateRequest WalletStateRequest
	if !getPostData(w, req, &walletStateRequest) {
		return
	}

	if !s.auth.IsValidSignature(walletStateRequest.PubKey, walletStateRequest.BodyJSON, walletStateRequest.Signature) {
		errorJSON(w, http.StatusBadRequest, "Bad signature")
		return
	}

	var walletState wallet.WalletState
	if err := json.Unmarshal([]byte(walletStateRequest.BodyJSON), &walletState); err != nil {
		errorJSON(w, http.StatusBadRequest, "Malformed walletState JSON")
		return
	}

	if s.walletUtil.ValidateWalletState(&walletState) {
		// TODO
	}

	if !s.checkAuth(
		w,
		walletStateRequest.PubKey,
		walletState.DeviceID,
		walletStateRequest.Token,
		auth.ScopeFull,
	) {
		return
	}

	// TODO - we could do an extra check- pull from db, make ure the new
	// walletState doesn't regress lastSynced for any given device.
	// Devices should do this, at least? But, we do other validation checks
	// here, but it's still that doesn't require new database calls.

	latestWalletStateJSON, latestSignature, updated, err := s.store.SetWalletState(
		walletStateRequest.PubKey,
		walletStateRequest.BodyJSON,
		walletState.Sequence(),
		walletStateRequest.Signature,
		walletStateRequest.DownloadKey,
	)

	var response []byte

	if err == store.ErrNoWalletState {
		// We failed to update, and when we tried pulling the latest wallet state,
		// there was nothing there. This should only happen if the client sets
		// sequence != 1 for the first walletState, which would be a bug.
		// TODO - figure out better error messages and/or document this
		errorJSON(w, http.StatusConflict, "Bad sequence number (No existing wallet state)")
		return
	} else if err != nil {
		// Something other than sequence error
		internalServiceErrorJSON(w, err, "Error saving walletState")
		return
	}

	walletResponse := WalletStateResponse{
		BodyJSON:  latestWalletStateJSON,
		Signature: latestSignature,
	}
	if !updated {
		// TODO - should we even call this an error?
		walletResponse.Error = "Bad sequence number"
	}
	response, err = json.Marshal(walletResponse)

	if err != nil {
		internalServiceErrorJSON(w, err, "Error generating walletState response")
		return
	}

	// Response Code:
	//   200: Update successful
	//   409: Update unsuccessful, probably due to new walletState's
	//      sequence not being 1 + current walletState's sequence
	//
	// Response Body:
	//   Current walletState (if it exists). If update successful, we just return
	//   the same one passed in. If update not successful, return the latest one
	//   from the db for the client to merge.
	if updated {
		fmt.Fprintf(w, string(response))
	} else {
		// We don't give the error field.
		http.Error(w, string(response), http.StatusConflict)
	}
}
