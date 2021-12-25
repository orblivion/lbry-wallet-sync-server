package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"orblivion/lbry-id/auth"
	"orblivion/lbry-id/store"
	"orblivion/lbry-id/wallet"
	"testing"
	"time"
)

// Whereas sever_test.go stubs out auth store and wallet, these will use the real thing, but test fewer paths.

// TODO - test some unhappy paths? Don't want to retest all the unit tests though.

func checkStatusCode(t *testing.T, statusCode int, responseBody []byte, expectedStatusCodeSlice ...int) {
	var expectedStatusCode int
	if len(expectedStatusCodeSlice) == 1 {
		expectedStatusCode = expectedStatusCodeSlice[0]
	} else {
		expectedStatusCode = http.StatusOK
	}

	if want, got := expectedStatusCode, statusCode; want != got {
		t.Errorf("StatusCode: expected %s (%d), got %s (%d)", http.StatusText(want), want, http.StatusText(got), got)
		var errorResponse ErrorResponse
		err := json.Unmarshal(responseBody, &errorResponse)
		if err == nil {
			t.Fatalf("http response: %+v", errorResponse)
		} else {
			t.Fatalf("%s", err)
		}
	}
}

func request(t *testing.T, method string, handler func(http.ResponseWriter, *http.Request), path string, jsonResult interface{}, requestBody string) ([]byte, int) {
	req := httptest.NewRequest(
		method,
		path,
		bytes.NewBuffer([]byte(requestBody)),
	)
	w := httptest.NewRecorder()

	handler(w, req)
	responseBody, _ := ioutil.ReadAll(w.Body)

	err := json.Unmarshal(responseBody, &jsonResult)
	if err != nil {
		t.Errorf("Error unmarshalling response body err: %+v body: %s", err, responseBody)
	}

	return responseBody, w.Result().StatusCode
}

// Test some flows with syncing two devices that have the wallet locally.
func TestIntegrationWalletUpdates(t *testing.T) {
	st, tmpFile := store.StoreTestInit(t)
	defer store.StoreTestCleanup(tmpFile)

	s := Init(
		&auth.Auth{},
		&st,
		&wallet.WalletUtil{},
	)

	////////////////////
	// Get auth token - device 1
	////////////////////

	var authToken1 auth.AuthToken
	responseBody, statusCode := request(
		t,
		http.MethodPost,
		s.getAuthTokenFull,
		PathAuthTokenFull,
		&authToken1,
		fmt.Sprintf(`{
      "tokenRequestJSON": "{\"deviceID\": \"dev-1\", \"requestTime\": %d}",
      "publickey": "testPubKey",
      "signature": "Good Signature"
    }`,
			time.Now().Unix(),
		),
	)

	checkStatusCode(t, statusCode, responseBody)

	// result.Token is in hex, auth.AuthTokenLength is bytes in the original
	expectedTokenLength := auth.AuthTokenLength * 2
	if len(authToken1.Token) != expectedTokenLength {
		t.Fatalf("Expected auth response to contain token length 32: result: %+v", string(responseBody))
	}
	if authToken1.DeviceID != "dev-1" {
		t.Fatalf("Unexpected response DeviceID. want: %+v got: %+v", "dev-1", authToken1.DeviceID)
	}
	if authToken1.Scope != "*" {
		t.Fatalf("Unexpected response Scope. want: %+v got: %+v", "*", authToken1.Scope)
	}

	////////////////////
	// Get auth token - device 2
	////////////////////

	var authToken2 auth.AuthToken
	responseBody, statusCode = request(
		t,
		http.MethodPost,
		s.getAuthTokenFull,
		PathAuthTokenFull,
		&authToken2,
		fmt.Sprintf(`{
      "tokenRequestJSON": "{\"deviceID\": \"dev-2\", \"requestTime\": %d}",
      "publickey": "testPubKey",
      "signature": "Good Signature"
    }`,
			time.Now().Unix(),
		),
	)

	checkStatusCode(t, statusCode, responseBody)

	if authToken2.DeviceID != "dev-2" {
		t.Fatalf("Unexpected response DeviceID. want: %+v got: %+v", "dev-2", authToken2.DeviceID)
	}

	////////////////////
	// Put first wallet state - device 1
	////////////////////

	var walletStateResponse WalletStateResponse
	responseBody, statusCode = request(
		t,
		http.MethodPost,
		s.postWalletState,
		PathWalletState,
		&walletStateResponse,
		fmt.Sprintf(`{
      "token": "%s",
      "bodyJSON": "{\"encryptedWallet\": \"blah\", \"lastSynced\":{\"dev-1\": 1}, \"deviceId\": \"dev-1\" }",
      "publickey": "testPubKey",
      "downloadKey": "myDownloadKey",
      "signature": "Good Signature"
    }`, authToken1.Token),
	)

	checkStatusCode(t, statusCode, responseBody)

	var walletState wallet.WalletState
	err := json.Unmarshal([]byte(walletStateResponse.BodyJSON), &walletState)

	if err != nil {
		t.Fatalf("Unexpected error: %+v", err)
	}

	sequence := walletState.Sequence()
	if sequence != 1 {
		t.Fatalf("Unexpected response Sequence(). want: %+v got: %+v", 1, sequence)
	}

	////////////////////
	// Get wallet state - device 2
	////////////////////

	responseBody, statusCode = request(
		t,
		http.MethodGet,
		s.getWalletState,
		fmt.Sprintf(
			"%s?token=%s&publicKey=%s&deviceId=%s",
			PathWalletState, authToken2.Token, authToken2.PubKey, "dev-2"),
		&walletStateResponse,
		"",
	)

	checkStatusCode(t, statusCode, responseBody)

	err = json.Unmarshal([]byte(walletStateResponse.BodyJSON), &walletState)

	if err != nil {
		t.Fatalf("Unexpected error: %+v", err)
	}

	sequence = walletState.Sequence()
	if sequence != 1 {
		t.Fatalf("Unexpected response Sequence(). want: %+v got: %+v", 1, sequence)
	}

	////////////////////
	// Put second wallet state - device 2
	////////////////////

	responseBody, statusCode = request(
		t,
		http.MethodPost,
		s.postWalletState,
		PathWalletState,
		&walletStateResponse,
		fmt.Sprintf(`{
      "token": "%s",
      "bodyJSON": "{\"encryptedWallet\": \"blah2\", \"lastSynced\":{\"dev-1\": 1, \"dev-2\": 2}, \"deviceId\": \"dev-2\" }",
      "publickey": "testPubKey",
      "downloadKey": "myDownloadKey",
      "signature": "Good Signature"
    }`, authToken2.Token),
	)

	checkStatusCode(t, statusCode, responseBody)

	err = json.Unmarshal([]byte(walletStateResponse.BodyJSON), &walletState)

	if err != nil {
		t.Fatalf("Unexpected error: %+v", err)
	}

	sequence = walletState.Sequence()
	if sequence != 2 {
		t.Fatalf("Unexpected response Sequence(). want: %+v got: %+v", 2, sequence)
	}

	////////////////////
	// Get wallet state - device 1
	////////////////////

	responseBody, statusCode = request(
		t,
		http.MethodGet,
		s.getWalletState,
		fmt.Sprintf(
			"%s?token=%s&publicKey=%s&deviceId=%s",
			PathWalletState, authToken1.Token, authToken1.PubKey, "dev-1"),
		&walletStateResponse,
		"",
	)

	checkStatusCode(t, statusCode, responseBody)

	err = json.Unmarshal([]byte(walletStateResponse.BodyJSON), &walletState)

	if err != nil {
		t.Fatalf("Unexpected error: %+v", err)
	}

	sequence = walletState.Sequence()
	if sequence != 2 {
		t.Fatalf("Unexpected response Sequence(). want: %+v got: %+v", 2, sequence)
	}
}

// Test a flow with a new device that needs to use the download key
func TestIntegrationNewDevice(t *testing.T) {
	st, tmpFile := store.StoreTestInit(t)
	defer store.StoreTestCleanup(tmpFile)

	s := Server{
		&auth.Auth{},
		&st,
		&wallet.WalletUtil{},
	}

	////////////////////
	// Get full auth token - device 1
	////////////////////

	var authToken1 auth.AuthToken
	responseBody, statusCode := request(
		t,
		http.MethodPost,
		s.getAuthTokenFull,
		PathAuthTokenFull,
		&authToken1,
		fmt.Sprintf(`{
      "tokenRequestJSON": "{\"deviceID\": \"dev-1\", \"requestTime\": %d}",
      "publickey": "testPubKey",
      "signature": "Good Signature"
    }`,
			time.Now().Unix(),
		),
	)

	checkStatusCode(t, statusCode, responseBody)

	if authToken1.DeviceID != "dev-1" {
		t.Fatalf("Unexpected response DeviceID. want: %+v got: %+v", "dev-1", authToken1.DeviceID)
	}
	if authToken1.Scope != "*" {
		t.Fatalf("Unexpected response Scope. want: %+v got: %+v", "*", authToken1.Scope)
	}

	////////////////////
	// Register email address - device 1
	////////////////////

	var registerResponse struct{}
	responseBody, statusCode = request(
		t,
		http.MethodPost,
		s.register,
		PathRegister,
		&registerResponse,
		fmt.Sprintf(`{
      "token": "%s",
      "publicKey": "testPubKey",
      "deviceId": "dev-1",
      "email": "address@example.com"
    }`, authToken1.Token),
	)

	checkStatusCode(t, statusCode, responseBody, http.StatusCreated)

	////////////////////
	// Put wallet state - device 1
	////////////////////

	var walletStateResponse WalletStateResponse
	responseBody, statusCode = request(
		t,
		http.MethodPost,
		s.postWalletState,
		PathWalletState,
		&walletStateResponse,
		fmt.Sprintf(`{
      "token": "%s",
      "bodyJSON": "{\"encryptedWallet\": \"blah\", \"lastSynced\":{\"dev-1\": 1}, \"deviceId\": \"dev-1\" }",
      "publickey": "testPubKey",
      "downloadKey": "myDownloadKey",
      "signature": "Good Signature"
    }`, authToken1.Token),
	)

	checkStatusCode(t, statusCode, responseBody)

	var walletState wallet.WalletState
	err := json.Unmarshal([]byte(walletStateResponse.BodyJSON), &walletState)

	if err != nil {
		t.Fatalf("Unexpected error: %+v", err)
	}

	sequence := walletState.Sequence()
	if sequence != 1 {
		t.Fatalf("Unexpected response Sequence(). want: %+v got: %+v", 1, sequence)
	}

	////////////////////
	// Get get-wallet-state auth token - device 2
	////////////////////

	var authToken2 auth.AuthToken
	responseBody, statusCode = request(
		t,
		http.MethodPost,
		s.getAuthTokenForGetWalletState,
		PathAuthTokenGetWalletState,
		&authToken2,
		`{
      "email": "address@example.com",
      "downloadKey": "myDownloadKey",
      "deviceID": "dev-2"
    }`,
	)

	checkStatusCode(t, statusCode, responseBody)

	// result.Token is in hex, auth.AuthTokenLength is bytes in the original
	expectedTokenLength := auth.AuthTokenLength * 2
	if len(authToken2.Token) != expectedTokenLength {
		t.Fatalf("Expected auth response to contain token length 32: result: %+v", string(responseBody))
	}
	if authToken2.DeviceID != "dev-2" {
		t.Fatalf("Unexpected response DeviceID. want: %+v got: %+v", "dev-2", authToken2.DeviceID)
	}

	////////////////////
	// Get wallet state - device 2
	////////////////////

	responseBody, statusCode = request(
		t,
		http.MethodGet,
		s.getWalletState,
		fmt.Sprintf(
			"%s?token=%s&publicKey=%s&deviceId=%s",
			PathWalletState, authToken2.Token, authToken2.PubKey, "dev-2"),
		&walletStateResponse,
		"",
	)

	checkStatusCode(t, statusCode, responseBody)

	err = json.Unmarshal([]byte(walletStateResponse.BodyJSON), &walletState)

	if err != nil {
		t.Fatalf("Unexpected error: %+v", err)
	}

	sequence = walletState.Sequence()
	if sequence != 1 {
		t.Fatalf("Unexpected response Sequence(). want: %+v got: %+v", 1, sequence)
	}
}
