package auth

import (
	"testing"
)

// Test stubs for now

func TestAuthValidateTokenRequest(t *testing.T) {
	// also add a basic test case for this in TestServerAuthHandlerErrors to make sure it's called at all
	t.Fatalf("Test me: Implement and test ValidateTokenRequest")
}

func TestAuthSignaturePass(t *testing.T) {
	t.Fatalf("Test me: Valid siganture passes")
}

func TestAuthSignatureFail(t *testing.T) {
	t.Fatalf("Test me: Valid siganture fails")
}

func TestAuthNewTokenSuccess(t *testing.T) {
	t.Fatalf("Test me: New token passes. Different scopes etc.")
}

func TestAuthNewTokenFail(t *testing.T) {
	t.Fatalf("Test me: New token fails (error generating random string? others?)")
}

func TestAuthScopeValid(t *testing.T) {
	t.Fatalf("Test me: Scope Valid tests")
	/*
		  authToken.Scope = "get-wallet-state"; authToken.ScopeValid("*")
			authToken.Scope = "get-wallet-state"; authToken.ScopeValid("get-wallet-state")

			// even things that haven't been defined yet, for simplicity
			authToken.Scope = "bananas"; authToken.ScopeValid("*")
	*/
}

func TestAuthScopeInvalid(t *testing.T) {
	t.Fatalf("Test me: Scope Invalid tests")
	/*
		authToken.Scope = "get-wallet-state"; authToken.ScopeValid("bananas")
		authToken.Scope = "bananas"; authToken.ScopeValid("get-wallet-state")
	*/
}
