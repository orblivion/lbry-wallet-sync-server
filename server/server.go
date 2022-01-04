package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"orblivion/lbry-id/auth"
	"orblivion/lbry-id/store"
	"orblivion/lbry-id/wallet"
)

// TODO proper doc comments!

const PathAuthTokenFull = "/auth/full"
const PathAuthTokenGetWalletState = "/auth/get-wallet-state"
const PathRegister = "/signup"
const PathWalletState = "/wallet-state"

type Server struct {
	auth       auth.AuthInterface
	store      store.StoreInterface
	walletUtil wallet.WalletUtilInterface
}

func Init(
	auth auth.AuthInterface,
	store store.StoreInterface,
	walletUtil wallet.WalletUtilInterface,
) *Server {
	return &Server{
		auth:       auth,
		store:      store,
		walletUtil: walletUtil,
	}
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func errorJSON(w http.ResponseWriter, code int, extra string) {
	errorStr := http.StatusText(code)
	if extra != "" {
		errorStr = errorStr + ": " + extra
	}
	authErrorJSON, err := json.Marshal(ErrorResponse{Error: errorStr})
	if err != nil {
		// In case something really stupid happens
		http.Error(w, `{"error": "error when JSON-encoding error message"}`, code)
	}
	http.Error(w, string(authErrorJSON), code)
	return
}

func internalServiceErrorJSON(w http.ResponseWriter, err error, errContext string) {
	errorStr := http.StatusText(http.StatusInternalServerError)
	authErrorJSON, err := json.Marshal(ErrorResponse{Error: errorStr})
	if err != nil {
		// In case something really stupid happens
		http.Error(w, `{"error": "error when JSON-encoding error message"}`, http.StatusInternalServerError)
	}
	http.Error(w, string(authErrorJSON), http.StatusInternalServerError)
	log.Printf("%s: %+v\n", errContext, err)

	return
}

//////////////////
// Handler Helpers
//////////////////

// Cut down on code repetition. No need to return errors since it can all be
// handled here. Just return a bool to indicate success.
// TODO the names `getPostData` and `getGetData` don't fully describe what they do

func requestOverhead(w http.ResponseWriter, req *http.Request, method string) bool {
	if req.Method != method {
		errorJSON(w, http.StatusMethodNotAllowed, "")
		return false
	}

	/*
		TODO - http.StatusRequestEntityTooLarge for some arbitrary large size
		see:
		* MaxBytesReader or LimitReader
		* https://pkg.go.dev/net/http#Request.ParseForm
		* some library/framework that handles it (along with req.Method)

		also - GET params too large?
	*/

	return true
}

// All structs representing incoming json request body should implement this
type PostRequest interface {
	validate() bool
}

// Confirm it's a Post request, various overhead, decode the json, validate the struct
func getPostData(w http.ResponseWriter, req *http.Request, reqStruct PostRequest) bool {
	if !requestOverhead(w, req, http.MethodPost) {
		return false
	}

	if err := json.NewDecoder(req.Body).Decode(&reqStruct); err != nil {
		errorJSON(w, http.StatusBadRequest, "Malformed request body JSON")
		return false
	}

	if !reqStruct.validate() {
		// TODO validate() should return useful error messages instead of a bool.
		errorJSON(w, http.StatusBadRequest, "Request failed validation")
		return false
	}

	return true
}

// Confirm it's a Get request, various overhead
func getGetData(w http.ResponseWriter, req *http.Request) bool {
	return requestOverhead(w, req, http.MethodGet)
}

func (s *Server) checkAuth(
	w http.ResponseWriter,
	pubKey auth.PublicKey,
	deviceId string,
	token auth.AuthTokenString,
	scope auth.AuthScope,
) bool {
	authToken, err := s.store.GetToken(pubKey, deviceId)
	if err == store.ErrNoToken {
		errorJSON(w, http.StatusUnauthorized, "Token Not Found")
		return false
	}
	if err != nil {
		internalServiceErrorJSON(w, err, "Error getting Token")
		return false
	}

	if authToken.Token != token {
		errorJSON(w, http.StatusUnauthorized, "Token Invalid")
		return false
	}

	if !authToken.ScopeValid(scope) {
		errorJSON(w, http.StatusForbidden, "Scope")
		return false
	}

	return true
}

// TODO - both wallet and token requests should be PUT, not POST.
// PUT = "...creates a new resource or replaces a representation of the target resource with the request payload."

func (s *Server) Serve() {
	http.HandleFunc(PathAuthTokenGetWalletState, s.getAuthTokenForGetWalletState)
	http.HandleFunc(PathAuthTokenFull, s.getAuthTokenFull)
	http.HandleFunc(PathWalletState, s.handleWalletState)
	http.HandleFunc(PathRegister, s.register)

	fmt.Println("Serving at :8090")
	http.ListenAndServe(":8090", nil)
}
