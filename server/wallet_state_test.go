package server

import (
	"testing"
)

func TestServerGetWalletSuccess(t *testing.T) {
	t.Fatalf("Test me: GetWallet succeeds")
}

func TestServerGetWalletErrors(t *testing.T) {
	t.Fatalf("Test me: GetWallet fails for various reasons (malformed, auth, db fail)")
}

func TestServerGetWalletStateParams(t *testing.T) {
	t.Fatalf("Test me: getWalletStateParams")
}

func TestServerPostWalletSuccess(t *testing.T) {
	t.Fatalf("Test me: PostWallet succeeds and returns the new wallet, PostWallet succeeds but is preempted")
}

func TestServerPostWalletTooLate(t *testing.T) {
	t.Fatalf("Test me: PostWallet fails for sequence being too low, returns the latest wallet")
}

func TestServerPostWalletErrors(t *testing.T) {
	// (malformed json, db fail, auth token not found, walletstate signature fail, walletstate invalid (via stub, make sure the validation function is even called), sequence too high, device id doesn't match token device id)
	// Client sends sequence != 1 for first entry
	// Client sends sequence == x + 10 for xth entry or whatever
	t.Fatalf("Test me: PostWallet fails for various reasons")
}

func TestServerValidateWalletStateRequest(t *testing.T) {
	// also add a basic test case for this in TestServerAuthHandlerSuccess to make sure it's called at all
	// Maybe 401 specifically for missing signature?
	t.Fatalf("Test me: Implement and test WalletStateRequest.validate()")
}

func TestServerHandleWalletState(t *testing.T) {
	t.Fatalf("Test me: Call the get or post function as appropriate. Alternately: call handleWalletState for the existing tests.")
}
