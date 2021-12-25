package store

import (
	"orblivion/lbry-id/auth"
	"reflect"
	"testing"
	"time"
)

// Test insertToken, using GetToken as a helper
// Try insertToken twice with the same public key, error the second time
func TestStoreInsertToken(t *testing.T) {

	s, tmpFile := StoreTestInit(t)
	defer StoreTestCleanup(tmpFile)

	authToken1 := auth.AuthToken{
		Token:    "seekrit-1",
		DeviceID: "dID",
		Scope:    "*",
		PubKey:   "pubKey",
	}

	// The value expected when we pull it from the database.
	authToken1DB := authToken1
	authToken1DB.Expiration = timePtr(time.Now().Add(time.Hour * 24 * 14).UTC())

	authToken2 := authToken1
	authToken2.Token = "seekrit-2"

	// Get a token, come back empty
	gotToken, err := s.GetToken(authToken1.PubKey, authToken1.DeviceID)
	if gotToken != nil || err != ErrNoToken {
		t.Fatalf("Expected ErrNoToken. token: %+v err: %+v", gotToken, err)
	}

	// Put in a token
	if err := s.insertToken(&authToken1, *authToken1DB.Expiration); err != nil {
		t.Fatalf("Unexpected error in insertToken: %+v", err)
	}

	// Get and confirm the token we just put in
	gotToken, err = s.GetToken(authToken1.PubKey, authToken1.DeviceID)
	if err != nil {
		t.Fatalf("Unexpected error in GetToken: %+v", err)
	}
	if gotToken == nil || !reflect.DeepEqual(*gotToken, authToken1DB) {
		t.Fatalf("token: \n  expected %+v\n  got:     %+v", authToken1DB, *gotToken)
	}

	// Try to put a different token, fail becaues we already have one
	if err := s.insertToken(&authToken2, *authToken1DB.Expiration); err != ErrDuplicateToken {
		t.Fatalf(`insertToken err: wanted "%+v", got "%+v"`, ErrDuplicateToken, err)
	}

	// Get the same *first* token we successfully put in
	gotToken, err = s.GetToken(authToken1.PubKey, authToken1.DeviceID)
	if err != nil {
		t.Fatalf("Unexpected error in GetToken: %+v", err)
	}
	if gotToken == nil || !reflect.DeepEqual(*gotToken, authToken1DB) {
		t.Fatalf("token: expected %+v, got: %+v", authToken1DB, gotToken)
	}
}

// Test updateToken, using GetToken and insertToken as helpers
// Try updateToken with no existing token, err for lack of anything to update
// Try updateToken with a preexisting token, succeed
// Try updateToken again with a new token, succeed
func TestStoreUpdateToken(t *testing.T) {
	s, tmpFile := StoreTestInit(t)
	defer StoreTestCleanup(tmpFile)

	authToken1 := auth.AuthToken{
		Token:    "seekrit-1",
		DeviceID: "dID",
		Scope:    "*",
		PubKey:   "pubKey",
	}
	authToken2 := authToken1
	authToken2.Token = "seekrit-2"

	// The value expected when we pull it from the database.
	authToken2DB := authToken2
	authToken2DB.Expiration = timePtr(time.Now().Add(time.Hour * 24 * 14).UTC())

	// Try to get a token, come back empty because we're just starting out
	gotToken, err := s.GetToken(authToken1.PubKey, authToken1.DeviceID)
	if gotToken != nil || err != ErrNoToken {
		t.Fatalf("Expected ErrNoToken. token: %+v err: %+v", gotToken, err)
	}

	// Try to update the token - fail because we don't have an entry there in the first place
	if err := s.updateToken(&authToken1, *authToken2DB.Expiration); err != ErrNoToken {
		t.Fatalf(`updateToken err: wanted "%+v", got "%+v"`, ErrNoToken, err)
	}

	// Try to get a token, come back empty because the update attempt failed to do anything
	gotToken, err = s.GetToken(authToken1.PubKey, authToken1.DeviceID)
	if gotToken != nil || err != ErrNoToken {
		t.Fatalf("Expected ErrNoToken. token: %+v err: %+v", gotToken, err)
	}

	// Put in a token - just so we have something to test updateToken with
	if err := s.insertToken(&authToken1, *authToken2DB.Expiration); err != nil {
		t.Fatalf("Unexpected error in insertToken: %+v", err)
	}

	// Now successfully update token
	if err := s.updateToken(&authToken2, *authToken2DB.Expiration); err != nil {
		t.Fatalf("Unexpected error in updateToken: %+v", err)
	}

	// Get and confirm the token we just put in
	gotToken, err = s.GetToken(authToken2.PubKey, authToken2.DeviceID)
	if err != nil {
		t.Fatalf("Unexpected error in GetToken: %+v", err)
	}
	if gotToken == nil || !reflect.DeepEqual(*gotToken, authToken2DB) {
		t.Fatalf("token: \n  expected %+v\n  got:     %+v", authToken2DB, *gotToken)
	}
}

// Two different devices.
// Test first and second Save (one for insert, one for update)
// Get fails initially
// Put token1-d1 token1-d2
// Get token1-d1 token1-d2
// Put token2-d1 token2-d2
// Get token2-d1 token2-d2
func TestStoreSaveToken(t *testing.T) {
	s, tmpFile := StoreTestInit(t)
	defer StoreTestCleanup(tmpFile)

	// Version 1 of the token for both devices
	authToken_d1_1 := auth.AuthToken{
		Token:    "seekrit-d1-1",
		DeviceID: "dID-1",
		Scope:    "*",
		PubKey:   "pubKey",
	}

	authToken_d2_1 := authToken_d1_1
	authToken_d2_1.DeviceID = "dID-2"
	authToken_d2_1.Token = "seekrit-d2-1"

	// Version 2 of the token for both devices
	authToken_d1_2 := authToken_d1_1
	authToken_d1_2.Token = "seekrit-d1-2"

	authToken_d2_2 := authToken_d2_1
	authToken_d2_2.Token = "seekrit-d2-2"

	// Try to get the tokens, come back empty because we're just starting out
	gotToken, err := s.GetToken(authToken_d1_1.PubKey, authToken_d1_1.DeviceID)
	if gotToken != nil || err != ErrNoToken {
		t.Fatalf("Expected ErrNoToken. token: %+v err: %+v", gotToken, err)
	}
	gotToken, err = s.GetToken(authToken_d2_1.PubKey, authToken_d2_1.DeviceID)
	if gotToken != nil || err != ErrNoToken {
		t.Fatalf("Expected ErrNoToken. token: %+v err: %+v", gotToken, err)
	}

	// Save Version 1 tokens for both devices
	if err = s.SaveToken(&authToken_d1_1); err != nil {
		t.Fatalf("Unexpected error in SaveToken: %+v", err)
	}
	if err = s.SaveToken(&authToken_d2_1); err != nil {
		t.Fatalf("Unexpected error in SaveToken: %+v", err)
	}

	// Check one of the authTokens to make sure expiration was set
	if authToken_d1_1.Expiration == nil {
		t.Fatalf("Expected SaveToken to set an Expiration")
	}
	nowDiff := authToken_d1_1.Expiration.Sub(time.Now())
	if time.Hour*24*14+time.Minute < nowDiff || nowDiff < time.Hour*24*14-time.Minute {
		t.Fatalf("Expected SaveToken to set a token Expiration 2 weeks in the future.")
	}

	// Get and confirm the tokens we just put in
	gotToken, err = s.GetToken(authToken_d1_1.PubKey, authToken_d1_1.DeviceID)
	if err != nil {
		t.Fatalf("Unexpected error in GetToken: %+v", err)
	}
	if gotToken == nil || !reflect.DeepEqual(*gotToken, authToken_d1_1) {
		t.Fatalf("token: \n  expected %+v\n  got:    %+v", authToken_d1_1, gotToken)
	}
	gotToken, err = s.GetToken(authToken_d2_1.PubKey, authToken_d2_1.DeviceID)
	if err != nil {
		t.Fatalf("Unexpected error in GetToken: %+v", err)
	}
	if gotToken == nil || !reflect.DeepEqual(*gotToken, authToken_d2_1) {
		t.Fatalf("token: expected %+v, got: %+v", authToken_d2_1, gotToken)
	}

	// Save Version 2 tokens for both devices
	if err = s.SaveToken(&authToken_d1_2); err != nil {
		t.Fatalf("Unexpected error in SaveToken: %+v", err)
	}
	if err = s.SaveToken(&authToken_d2_2); err != nil {
		t.Fatalf("Unexpected error in SaveToken: %+v", err)
	}

	// Check that the expiration of this new token is marginally later
	if authToken_d1_2.Expiration == nil {
		t.Fatalf("Expected SaveToken to set an Expiration")
	}
	expDiff := authToken_d1_2.Expiration.Sub(*authToken_d1_1.Expiration)
	if time.Second < expDiff || expDiff < 0 {
		t.Fatalf("Expected new expiration to be slightly later than previous expiration. diff: %+v", expDiff)
	}

	// Get and confirm the tokens we just put in
	gotToken, err = s.GetToken(authToken_d1_2.PubKey, authToken_d1_2.DeviceID)
	if err != nil {
		t.Fatalf("Unexpected error in GetToken: %+v", err)
	}
	if gotToken == nil || !reflect.DeepEqual(*gotToken, authToken_d1_2) {
		t.Fatalf("token: \n  expected %+v\n  got:    %+v", authToken_d1_2, gotToken)
	}
	gotToken, err = s.GetToken(authToken_d2_2.PubKey, authToken_d2_2.DeviceID)
	if err != nil {
		t.Fatalf("Unexpected error in GetToken: %+v", err)
	}
	if gotToken == nil || !reflect.DeepEqual(*gotToken, authToken_d2_2) {
		t.Fatalf("token: expected %+v, got: %+v", authToken_d2_2, gotToken)
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// test GetToken using insertToken and updateToken as helpers (so we can set expiration timestamps)
// normal
// not found for pubkey
// not found for device (one for another device does exist)
// expired token not returned
func TestStoreGetToken(t *testing.T) {
	s, tmpFile := StoreTestInit(t)
	defer StoreTestCleanup(tmpFile)

	// created for addition to the DB (no expiration attached)
	authToken := auth.AuthToken{
		Token:    "seekrit-d1",
		DeviceID: "dID",
		Scope:    "*",
		PubKey:   "pubKey",
	}

	// The value expected when we pull it from the database.
	authTokenDB := authToken
	authTokenDB.Expiration = timePtr(time.Time(time.Now().UTC().Add(time.Hour * 24 * 14)))

	// Not found (nothing saved for this pubkey)
	gotToken, err := s.GetToken(authToken.PubKey, authToken.DeviceID)
	if gotToken != nil || err != ErrNoToken {
		t.Fatalf("Expected ErrNoToken. token: %+v err: %+v", gotToken, err)
	}

	// Put in a token
	if err := s.insertToken(&authToken, *authTokenDB.Expiration); err != nil {
		t.Fatalf("Unexpected error in insertToken: %+v", err)
	}

	// Confirm it saved
	gotToken, err = s.GetToken(authToken.PubKey, authToken.DeviceID)
	if err != nil {
		t.Fatalf("Unexpected error in GetToken: %+v", err)
	}
	if gotToken == nil || !reflect.DeepEqual(*gotToken, authTokenDB) {
		t.Fatalf("token: \n  expected %+v\n  got:     %+v", authTokenDB, gotToken)
	}

	// Fail to get for another device
	gotToken, err = s.GetToken(authToken.PubKey, "other-device")
	if gotToken != nil || err != ErrNoToken {
		t.Fatalf("Expected ErrNoToken for nonexistent device. token: %+v err: %+v", gotToken, err)
	}

	// Update the token to be expired
	expirationOld := time.Now().Add(time.Second * (-1))
	if err := s.updateToken(&authToken, expirationOld); err != nil {
		t.Fatalf("Unexpected error in updateToken: %+v", err)
	}

	// Fail to get the expired token
	gotToken, err = s.GetToken(authToken.PubKey, authToken.DeviceID)
	if gotToken != nil || err != ErrNoToken {
		t.Fatalf("Expected ErrNoToken, for expired token. token: %+v err: %+v", gotToken, err)
	}
}

func TestStoreSanitizeEmptyFields(t *testing.T) {
	// Make sure expiration doesn't get set if sanitization fails
	t.Fatalf("Test me")
}

func TestStoreTimeZones(t *testing.T) {
	// Make sure the tz situation is as we prefer in the DB. Probably just do UTC.
	t.Fatalf("Test me")
}

func TestStoreSaveWalletSuccess(t *testing.T) {
	/*
	  Sequence 1 - works via insert
	  Sequence 2 - works via update
	  Sequence 3 - works via update
	*/
	t.Fatalf("Test me: Wallet Store successes")
}

func TestStoreSaveWalletFail(t *testing.T) {
	/*
	  Sequence 1 - fails via insert - fail by having something there already
	  Sequence 2 - fails via update - fail by not having something there already
	  Sequence 3 - fails via update - fail by having something with wrong sequence number
	  Sequence 4 - fails via update - fail by having something with non-matching device sequence history

	  Maybe some of the above gets put off to wallet util
	*/
	t.Fatalf("Test me: Wallet Store failures")
}

func TestStoreInsertWalletSuccess(t *testing.T) {
	t.Fatalf("Test me: Wallet insert successes")
}

func TestStoreInsertWalletFail(t *testing.T) {
	t.Fatalf("Test me: Wallet insert failures")
}

func TestStoreUpdateWalletSuccess(t *testing.T) {
	t.Fatalf("Test me: Wallet update successes")
}

func TestStoreUpdateWalletFail(t *testing.T) {
	t.Fatalf("Test me: Wallet update failures")
}

func TestStoreGetWalletSuccess(t *testing.T) {
	t.Fatalf("Test me: Wallet get success")
}

func TestStoreGetWalletFail(t *testing.T) {
	t.Fatalf("Test me: Wallet get failures")
}

func TestStoreGetDownloadKeySuccess(t *testing.T) {
	t.Fatalf("Test me: Wallet get success")
}

func TestStoreGetDownloadKeyFail(t *testing.T) {
	t.Fatalf("Test me: Wallet get failures")
}

func TestStoreSetEmailSuccess(t *testing.T) {
	t.Fatalf("Test me: Wallet get success")
}

func TestStoreSetEmailFail(t *testing.T) {
	t.Fatalf("Test me: Wallet get failures")
}

func TestStoreGetPublicKeySuccess(t *testing.T) {
	t.Fatalf("Test me: Wallet get success")
}

func TestStoreGetPublicKeyFail(t *testing.T) {
	t.Fatalf("Test me: Wallet get failures")
}
