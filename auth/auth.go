package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// TODO - Learn how to use https://github.com/golang/oauth2 instead
// TODO - Look into jwt, etc.
// For now I just want a process that's shaped like what I'm looking for (pubkey signatures, downloadKey, etc)

type AuthTokenString string
type PublicKey string
type DownloadKey string
type Signature string

type AuthScope string

const ScopeFull = AuthScope("*")
const ScopeGetWalletState = AuthScope("get-wallet-state")

type AuthInterface interface {
	NewToken(pubKey PublicKey, DeviceID string, Scope AuthScope) (*AuthToken, error)
	IsValidSignature(pubKey PublicKey, payload string, signature Signature) bool
	ValidateTokenRequest(tokenRequest *TokenRequest) bool
}

type Auth struct{}

func (a *Auth) IsValidSignature(pubKey PublicKey, payload string, signature Signature) bool {
	// TODO - a real check
	return signature == "Good Signature"
}

// Note that everything here is given to anybody who presents a valid
// downloadKey and associated email. Currently these fields are lower
// security levels than those fields. Keep this in mind as we change this
// struct.
type AuthToken struct {
	Token      AuthTokenString `json:"token"`
	DeviceID   string          `json:"deviceId"`
	Scope      AuthScope       `json:"scope"`
	PubKey     PublicKey       `json:"publicKey"`
	Expiration *time.Time      `json:"expiration"`
}

type TokenRequest struct {
	DeviceID    string `json:"deviceId"`
	RequestTime int64  `json:"requestTime"`
	// TODO - add target domain as well. anything to limit the scope of the
	// request to mitigate replays.
}

func (a *Auth) ValidateTokenRequest(tokenRequest *TokenRequest) bool {
	if tokenRequest.DeviceID == "" {
		return false
	}

	// Since we're going by signatures with a key that we don't want to change,
	// let's avoid replays.
	timeDiff := time.Now().Unix() - tokenRequest.RequestTime
	if timeDiff < -2 {
		// Maybe time drift will cause the request time to be in the future. This
		// would also include request time. Only allow a few seconds of this.
		return false
	}
	if timeDiff > 60 {
		// Maybe the request is slow. Allow for a minute of lag time.
		return false
	}

	return true
}

const AuthTokenLength = 32

func (a *Auth) NewToken(pubKey PublicKey, DeviceID string, Scope AuthScope) (*AuthToken, error) {
	b := make([]byte, AuthTokenLength)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("Error generating token: %+v", err)
	}

	return &AuthToken{
		Token:    AuthTokenString(hex.EncodeToString(b)),
		DeviceID: DeviceID,
		Scope:    Scope,
		PubKey:   pubKey,
		// TODO add Expiration here instead of putting it in store.go. and thus redo store.go. d'oh.
	}, nil
}

// NOTE - not stubbing methods of structs like this. more convoluted than it's worth right now
func (at *AuthToken) ScopeValid(required AuthScope) bool {
	// So far the only two scopes issued
	if at.Scope == ScopeFull {
		return true
	}
	if at.Scope == ScopeGetWalletState && required == ScopeGetWalletState {
		return true
	}
	return false
}

func (d DownloadKey) Obfuscate() string {
	// TODO KDF instead
	hash := sha256.Sum256([]byte(d))
	return hex.EncodeToString(hash[:])
}
