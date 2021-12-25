package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"orblivion/lbry-id/auth"
	"orblivion/lbry-id/store"
)

/*
TODO - consider reworking the naming convention

Rename `AuthFullRequest` struct to:

type TokenRequest struct {
	BodyJSON  string // or maybe PayloadJSON
	PubKey    auth.PublicKey
	Signature string
}

And then rename the existing `TokenRequest` to `TokenRequestBody` (this is what `BodyJSON` unmarhals to).

The reason is that we'll need this format for walletState eventually. The walletState as such, saved on devices,
passed around, etc, should contain the signature and the public key, but of course the signed portion itself
cannot contain the signature.

So, the part not containing the signature we'll similarly call something lke `WalletStateBody`. `WaletStateBody`
will still contain other metadata about the encrypted wallet such as DeviceID and Sequence. We only keep the
signature and PubKey outside because of the verification process. We could keep the PubKey inside the Body but
it's more convenient this way, and the signature process will verify it along with the body.
*/

type AuthFullRequest struct {
	// TokenRequestJSON: json string within json, so that the string representation is
	//   unambiguous for the purposes of signing. This means we need to deserialize the
	//   request body twice.
	TokenRequestJSON string         `json:"tokenRequestJSON"`
	PubKey           auth.PublicKey `json:"publicKey"`
	Signature        auth.Signature `json:"signature"`
}

func (r *AuthFullRequest) validate() bool {
	return (r.TokenRequestJSON != "" &&
		r.PubKey != auth.PublicKey("") &&
		r.Signature != auth.Signature(""))
}

type AuthForGetWalletStateRequest struct {
	// TokenRequestJSON: json string within json, so that the string representation is
	//   unambiguous for the purposes of signing. This means we need to deserialize the
	//   request body twice.
	Email       string           `json:"email"`
	DownloadKey auth.DownloadKey `json:"downloadKey"`
	DeviceID    string           `json:"deviceId"`
}

func (r *AuthForGetWalletStateRequest) validate() bool {
	return (r.Email != "" &&
		r.DownloadKey != auth.DownloadKey("") &&
		r.DeviceID != "")
}

// NOTE - (Perhaps for docs)
//
// This is not very well authenticated. Requiring the downloadKey and email
// isn't very high security. It adds an entry into the same auth_tokens db
// table as full auth tokens. There won't be a danger of a malicious actor
// overriding existing auth tokens so long as the legitimate devices choose
// unique DeviceIDs. (DeviceID being part of the primary key in the auth token
// table.)
//
// A malicious actor could try to flood the auth token table to take down the
// server, but then again they could do this with a legitimate account as well.
// We could perhaps require registration for full auth tokens and limit to 10
// get-wallet-state auth tokens per account.

func (s *Server) getAuthTokenForGetWalletState(w http.ResponseWriter, req *http.Request) {
	var authRequest AuthForGetWalletStateRequest
	if !getPostData(w, req, &authRequest) {
		return
	}

	pubKey, err := s.store.GetPublicKey(authRequest.Email, authRequest.DownloadKey)
	if err == store.ErrNoPubKey {
		errorJSON(w, http.StatusUnauthorized, "No match for email and password")
		return
	}
	if err != nil {
		internalServiceErrorJSON(w, err, "Error getting public key")
		return
	}

	authToken, err := s.auth.NewToken(pubKey, authRequest.DeviceID, auth.ScopeGetWalletState)

	if err != nil {
		internalServiceErrorJSON(w, err, "Error generating auth token")
		log.Print(err)
		return
	}

	// NOTE - see comment on auth.AuthToken definition regarding what we may
	// want to present to the client that has only presented a valid
	// downloadKey and email
	response, err := json.Marshal(&authToken)

	if err != nil {
		internalServiceErrorJSON(w, err, "Error generating auth token")
		return
	}

	if err := s.store.SaveToken(authToken); err != nil {
		internalServiceErrorJSON(w, err, "Error saving auth token")
		log.Print(err)
		return
	}

	fmt.Fprintf(w, string(response))
}

func (s *Server) getAuthTokenFull(w http.ResponseWriter, req *http.Request) {
	/*
	  (This comment may only be needed for WIP)

	   Server should be in charge of such things as:
	   * Request body size check (in particular to not tie up signature check)
	   * JSON validation/deserialization

	   auth.Auth should be in charge of such things as:
	   * Checking signatures
	   * Generating tokens

	   The order of events:
	   * Server checks the request body size
	   * Server deserializes and then validates the AuthFullRequest
	   * auth.Auth checks the signature of authRequest.TokenRequestJSON
	     * This the awkward bit, since auth.Auth is being passed a (serialized) JSON string.
	       However, it's not deserializing it. It's ONLY checking the signature of it
	       as a string per se. (The same function will be used for signed walletState)
	   * Server deserializes and then validates the auth.TokenRequest
	   * auth.Auth takes auth.TokenRequest and PubKey and generates a token
	   * DataStore stores the token. The pair (PubKey, TokenRequest.DeviceID) is the primary key.
	       We should have one token for each device.
	*/

	var authRequest AuthFullRequest
	if !getPostData(w, req, &authRequest) {
		return
	}

	if !s.auth.IsValidSignature(authRequest.PubKey, authRequest.TokenRequestJSON, authRequest.Signature) {
		errorJSON(w, http.StatusForbidden, "Bad signature")
		return
	}

	var tokenRequest auth.TokenRequest
	if err := json.Unmarshal([]byte(authRequest.TokenRequestJSON), &tokenRequest); err != nil {
		errorJSON(w, http.StatusBadRequest, "Malformed tokenRequest JSON")
		return
	}

	if !s.auth.ValidateTokenRequest(&tokenRequest) {
		errorJSON(w, http.StatusBadRequest, "tokenRequest failed validation")
		return
	}

	authToken, err := s.auth.NewToken(authRequest.PubKey, tokenRequest.DeviceID, auth.ScopeFull)

	if err != nil {
		internalServiceErrorJSON(w, err, "Error generating auth token")
		return
	}

	response, err := json.Marshal(&authToken)

	if err != nil {
		internalServiceErrorJSON(w, err, "Error generating auth token")
		return
	}

	if err := s.store.SaveToken(authToken); err != nil {
		internalServiceErrorJSON(w, err, "Error saving auth token")
		return
	}

	fmt.Fprintf(w, string(response))
}
