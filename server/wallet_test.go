package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"orblivion/lbry-id/auth"
	"orblivion/lbry-id/store"
	"orblivion/lbry-id/wallet"
)

func TestServerGetWallet(t *testing.T) {
	tt := []struct {
		name string

		expectedStatusCode  int
		expectedErrorString string

		storeErrors TestStoreFunctionsErrors
	}{
		{
			name:               "success",
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "auth error",

			expectedStatusCode:  http.StatusUnauthorized,
			expectedErrorString: http.StatusText(http.StatusUnauthorized) + ": Token Not Found",

			storeErrors: TestStoreFunctionsErrors{GetToken: store.ErrNoToken},
		},
		{
			name: "db error getting wallet",

			expectedStatusCode:  http.StatusInternalServerError,
			expectedErrorString: http.StatusText(http.StatusInternalServerError),

			storeErrors: TestStoreFunctionsErrors{GetWallet: fmt.Errorf("Some random DB Error!")},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			testAuth := TestAuth{}
			testStore := TestStore{
				TestAuthToken: auth.AuthToken{
					Token: auth.TokenString("seekrit"),
					Scope: auth.ScopeFull,
				},

				TestEncryptedWallet: wallet.EncryptedWallet("my-encrypted-wallet"),
				TestSequence:        wallet.Sequence(2),
				TestHmac:            wallet.WalletHmac("my-hmac"),

				Errors: tc.storeErrors,
			}

			s := Server{&testAuth, &testStore}

			req := httptest.NewRequest(http.MethodGet, PathWallet, nil)
			q := req.URL.Query()
			q.Add("token", string(testStore.TestAuthToken.Token))
			req.URL.RawQuery = q.Encode()
			w := httptest.NewRecorder()

			// test handleWallet while we're at it, which is a dispatch for get and post
			// wallet
			s.handleWallet(w, req)

			// Make sure we tried to get an auth based on the `token` param (whether or
			// not it was a valid `token`)
			if want, got := testStore.TestAuthToken.Token, testStore.Called.GetToken; want != got {
				t.Errorf("testStore.Called.GetToken called with: expected %s, got %s", want, got)
			}

			expectStatusCode(t, w, tc.expectedStatusCode)

			if len(tc.expectedErrorString) > 0 {
				// Only do this check if we're expecting an error and only an error,
				// since it reads the body and would break the ioutil.ReadAll below.
				expectErrorString(t, w, tc.expectedErrorString)
				return
			}

			body, _ := ioutil.ReadAll(w.Body)
			var result WalletResponse
			err := json.Unmarshal(body, &result)

			if err != nil ||
				result.EncryptedWallet != testStore.TestEncryptedWallet ||
				result.Hmac != testStore.TestHmac ||
				result.Sequence != testStore.TestSequence {
				t.Errorf("Expected wallet response to have the test wallet values: result: %+v err: %+v", string(body), err)
			}

			if !testStore.Called.GetWallet {
				t.Errorf("Expected Store.GetWallet to be called")
			}
		})
	}
}

func TestServerPostWallet(t *testing.T) {
	tt := []struct {
		name string

		expectedStatusCode  int
		expectedErrorString string

		// There is one case where we expect both the error field and the normal
		// body fields. So, this needs to be separate.
		expectWalletBody bool

		// `new...` refers to what is being passed into the via POST request (and
		//   what gets passed into SetWallet for the *non-error* cases below)
		// `returned...` refers to what the SetWallet function returns (and what
		//   gets returned in the request response for the *non-error* cases below)
		newEncryptedWallet      wallet.EncryptedWallet
		returnedEncryptedWallet wallet.EncryptedWallet
		newSequence             wallet.Sequence
		returnedSequence        wallet.Sequence
		newHmac                 wallet.WalletHmac
		returnedHmac            wallet.WalletHmac

		// Passed-in sequence was correct. No conflict.
		sequenceCorrect bool

		storeErrors TestStoreFunctionsErrors
	}{
		{
			name:               "success",
			expectedStatusCode: http.StatusOK,
			expectWalletBody:   true,

			// Simulates a situation where the existing sequence is 1, the new
			// sequence is 2. We don't see the existing data in this case because it
			// successfully updates to and returns the new data. New and returned are
			// the same here.

			sequenceCorrect:         true,
			newEncryptedWallet:      wallet.EncryptedWallet("my-encrypted-wallet"),
			returnedEncryptedWallet: wallet.EncryptedWallet("my-encrypted-wallet"),
			newSequence:             wallet.Sequence(2),
			returnedSequence:        wallet.Sequence(2),
			newHmac:                 wallet.WalletHmac("my-hmac"),
			returnedHmac:            wallet.WalletHmac("my-hmac"),
		},
		{
			name:               "conflict",
			expectedStatusCode: http.StatusConflict,
			// In the special case of "conflict" where there *is* a stored wallet, we
			// process the function in a normal way, but we still have the Error field.
			// So, we can't rely on `tc.expectedErrorString == ""` to indicate that it
			// is the type of error without a body. So, we need to specify this
			// separately. In this case we will check the error string along with the
			// body.
			expectWalletBody: true,

			// Simulates a situation where the existing sequence is 3, the new sequence
			// is 2. This is a conflict, because the new sequence should be 4. A new
			// sequence of 3 would also be a conflict, but we want two different
			// sequence numbers for a clear test. We return the existing data in this
			// case for the client to merge in. Note that we're passing in a sequence
			// that makes sense for a conflict case, the actual behavior is triggered by
			// sequenceCorrect=false

			expectedErrorString: http.StatusText(http.StatusConflict) + ": Bad sequence number",

			sequenceCorrect:         false,
			newEncryptedWallet:      wallet.EncryptedWallet("my-encrypted-wallet-new"),
			returnedEncryptedWallet: wallet.EncryptedWallet("my-encrypted-wallet-existing"),
			newSequence:             wallet.Sequence(2),
			returnedSequence:        wallet.Sequence(3),
			newHmac:                 wallet.WalletHmac("my-hmac-new"),
			returnedHmac:            wallet.WalletHmac("my-hmac-existing"),
		},
		{
			name:               "conflict with no stored wallet",
			expectedStatusCode: http.StatusConflict,

			// Simulates a situation where there is no stored wallet. In such a case the
			// correct sequence would be 1, which implies the wallet should be inserted
			// (as opposed to updated). We will be passing in a sequence of 2 for
			// clarity, but what will actually trigger the desired error we are testing
			// for is SetWallet returning ErrNoWallet, which is what the store is
			// expected to return in this situation.

			expectedErrorString: http.StatusText(http.StatusConflict) + ": Bad sequence number (No existing wallet)",

			// In this case the "returned" values are blank because there will be
			// nothing to return
			sequenceCorrect:         false,
			newEncryptedWallet:      wallet.EncryptedWallet("my-encrypted-wallet"),
			returnedEncryptedWallet: wallet.EncryptedWallet(""),
			newSequence:             wallet.Sequence(2),
			returnedSequence:        wallet.Sequence(0),
			newHmac:                 wallet.WalletHmac("my-hmac"),
			returnedHmac:            wallet.WalletHmac(""),

			storeErrors: TestStoreFunctionsErrors{SetWallet: store.ErrNoWallet},
		},
		{
			name:                "auth error",
			expectedStatusCode:  http.StatusUnauthorized,
			expectedErrorString: http.StatusText(http.StatusUnauthorized) + ": Token Not Found",

			// Putting in valid data here so it's clear that this isn't what causes
			// the error
			sequenceCorrect:         true,
			newEncryptedWallet:      wallet.EncryptedWallet("my-encrypted-wallet"),
			returnedEncryptedWallet: wallet.EncryptedWallet("my-encrypted-wallet"),
			newSequence:             wallet.Sequence(2),
			returnedSequence:        wallet.Sequence(2),
			newHmac:                 wallet.WalletHmac("my-hmac"),
			returnedHmac:            wallet.WalletHmac("my-hmac"),

      // What causes the error
			storeErrors: TestStoreFunctionsErrors{GetToken: store.ErrNoToken},
		},
		{
			name:                "db error setting or getting wallet",
			expectedStatusCode:  http.StatusInternalServerError,
			expectedErrorString: http.StatusText(http.StatusInternalServerError),

			// Putting in valid data here so it's clear that this isn't what causes
			// the error
			sequenceCorrect:         true,
			newEncryptedWallet:      wallet.EncryptedWallet("my-encrypted-wallet"),
			returnedEncryptedWallet: wallet.EncryptedWallet("my-encrypted-wallet"),
			newSequence:             wallet.Sequence(2),
			returnedSequence:        wallet.Sequence(2),
			newHmac:                 wallet.WalletHmac("my-hmac"),
			returnedHmac:            wallet.WalletHmac("my-hmac"),

      // What causes the error
			storeErrors: TestStoreFunctionsErrors{SetWallet: fmt.Errorf("Some random db problem")},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			testAuth := TestAuth{}
			testStore := TestStore{
				TestAuthToken: auth.AuthToken{
					Token: auth.TokenString("seekrit"),
					Scope: auth.ScopeFull,
				},

				TestEncryptedWallet: tc.returnedEncryptedWallet,
				TestSequence:        tc.returnedSequence,
				TestHmac:            tc.returnedHmac,
				TestSequenceCorrect: tc.sequenceCorrect,

				Errors: tc.storeErrors,
			}

			s := Server{&testAuth, &testStore}

			requestBody := []byte(
				fmt.Sprintf(`{
          "token": "%s",
          "encryptedWallet": "%s",
          "sequence": %d,
          "hmac": "%s"
        }`, testStore.TestAuthToken.Token, tc.newEncryptedWallet, tc.newSequence, tc.newHmac),
			)

			req := httptest.NewRequest(http.MethodPost, PathWallet, bytes.NewBuffer(requestBody))
			w := httptest.NewRecorder()

			// test handleWallet while we're at it, which is a dispatch for get and post
			// wallet
			s.handleWallet(w, req)

			// Make sure we tried to get an auth based on the `token` param (whether or
			// not it was a valid `token`)
			if want, got := testStore.TestAuthToken.Token, testStore.Called.GetToken; want != got {
				t.Errorf("testStore.Called.GetToken called with: expected %s, got %s", want, got)
			}

			expectStatusCode(t, w, tc.expectedStatusCode)

			if !tc.expectWalletBody {
				// Only do this check if we're expecting an error and only an error,
				// since it reads the body and would break the ioutil.ReadAll below.
				expectErrorString(t, w, tc.expectedErrorString)
				return
			}

			body, _ := ioutil.ReadAll(w.Body)
			var result WalletResponse
			err := json.Unmarshal(body, &result)

			if err != nil ||
				result.EncryptedWallet != tc.returnedEncryptedWallet ||
				result.Hmac != tc.returnedHmac ||
				result.Sequence != tc.returnedSequence ||
				result.Error != tc.expectedErrorString {

				t.Errorf("Expected wallet response to have the test wallet values and error string: result: %+v err: %+v", string(body), err)
			}

			if want, got := (SetWalletCall{tc.newEncryptedWallet, tc.newSequence, tc.newHmac}), testStore.Called.SetWallet; want != got {
				t.Errorf("Store.SetWallet called with: expected %+v, got %+v", want, got)
			}
		})
	}
}

func TestServerPostWalletErrors(t *testing.T) {
	// (malformed json, db fail, auth token not found, wallet metadata invalid (via stub, make sure the validation function is even called), sequence too high, device id doesn't match token device id)
	// Client sends sequence != 1 for first entry
	// Client sends sequence == x + 10 for xth entry or whatever
	t.Fatalf("Test me: PostWallet fails for various reasons")
}

func TestServerValidateWalletRequest(t *testing.T) {
	// also add a basic test case for this in TestServerAuthHandlerSuccess to make sure it's called at all
	t.Fatalf("Test me: Implement and test WalletRequest.validate()")
}
