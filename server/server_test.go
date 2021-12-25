package server

import (
	"fmt"
	"orblivion/lbry-id/auth"
	"testing"
)

// Implementing interfaces for stubbed out packages

type TestAuth struct {
	TestToken    auth.AuthTokenString
	FailSigCheck bool
	FailGenToken bool
}

func (a *TestAuth) NewToken(pubKey auth.PublicKey, DeviceID string, Scope auth.AuthScope) (*auth.AuthToken, error) {
	if a.FailGenToken {
		return nil, fmt.Errorf("Test error: fail to generate token")
	}
	return &auth.AuthToken{Token: a.TestToken, Scope: Scope}, nil
}

func (a *TestAuth) IsValidSignature(pubKey auth.PublicKey, payload string, signature auth.Signature) bool {
	return !a.FailSigCheck
}

func (a *TestAuth) ValidateTokenRequest(tokenRequest *auth.TokenRequest) bool {
	// TODO
	return true
}

type TestStore struct {
	FailSave bool

	SaveTokenCalled bool
}

func (s *TestStore) SaveToken(token *auth.AuthToken) error {
	if s.FailSave {
		return fmt.Errorf("TestStore.SaveToken fail")
	}
	s.SaveTokenCalled = true
	return nil
}

func (s *TestStore) GetToken(auth.PublicKey, string) (*auth.AuthToken, error) {
	return nil, nil
}

func (s *TestStore) GetPublicKey(string, auth.DownloadKey) (auth.PublicKey, error) {
	return "", nil
}

func (s *TestStore) InsertEmail(auth.PublicKey, string) error {
	return nil
}

func (s *TestStore) SetWalletState(
	pubKey auth.PublicKey,
	walletStateJson string,
	sequence int,
	signature auth.Signature,
	downloadKey auth.DownloadKey,
) (latestWalletStateJson string, latestSignature auth.Signature, updated bool, err error) {
	return
}

func (s *TestStore) GetWalletState(pubKey auth.PublicKey) (walletStateJSON string, signature auth.Signature, err error) {
	return
}

func TestServerHelperCheckAuthSuccess(t *testing.T) {
	t.Fatalf("Test me: checkAuth success")
}

func TestServerHelperCheckAuthErrors(t *testing.T) {
	t.Fatalf("Test me: checkAuth failure")
}

func TestServerHelperGetGetDataSuccess(t *testing.T) {
	t.Fatalf("Test me: getGetData success")
}
func TestServerHelperGetGetDataErrors(t *testing.T) {
	t.Fatalf("Test me: getGetData failure")
}

func TestServerHelperGetPostDataSuccess(t *testing.T) {
	t.Fatalf("Test me: getPostData success")
}
func TestServerHelperGetPostDataErrors(t *testing.T) {
	t.Fatalf("Test me: getPostData failure")
}

func TestServerHelperRequestOverheadSuccess(t *testing.T) {
	t.Fatalf("Test me: requestOverhead success")
}
func TestServerHelperRequestOverheadErrors(t *testing.T) {
	t.Fatalf("Test me: requestOverhead failures")
}
