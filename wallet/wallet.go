package wallet

// Currently a small package but given other packages it makes imports easier.
// Also this might grow substantially over time

// For test stubs
type WalletUtilInterface interface {
	ValidateWalletState(walletState *WalletState) bool
}

type WalletUtil struct{}

type WalletState struct {
	DeviceID   string         `json:"deviceId"`
	LastSynced map[string]int `json:"lastSynced"`
}

// TODO - These "validate" functions could/should be methods. Though I think
// we'd lose mockability for testing, since the method isn't the
// WalletUtilInterface.
// Mainly the job of the clients but we may as well short-circuit problems
// here before saving them.
func (wu *WalletUtil) ValidateWalletState(walletState *WalletState) bool {

	// TODO - nonempty fields, up to date, etc
	return true
}

// Assumptions: `ws` has been validated
// Avoid having to check for error
func (ws *WalletState) Sequence() int {
	return ws.LastSynced[ws.DeviceID]
}
