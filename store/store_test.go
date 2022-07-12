package store

import (
	"io/ioutil"
	"os"
	"testing"

	"lbryio/lbry-id/auth"
)

func StoreTestInit(t *testing.T) (s Store, tmpFile *os.File) {
	s = Store{}

	tmpFile, err := ioutil.TempFile(os.TempDir(), "sqlite-test-")
	if err != nil {
		t.Fatalf("DB setup failure: %+v", err)
		return
	}

	s.Init(tmpFile.Name())

	err = s.Migrate()
	if err != nil {
		t.Fatalf("DB setup failure: %+v", err)
	}

	return
}

func StoreTestCleanup(tmpFile *os.File) {
	if tmpFile != nil {
		os.Remove(tmpFile.Name())
	}
}

func makeTestUser(t *testing.T, s *Store) (userId auth.UserId, email auth.Email, password auth.Password) {
	email, password = auth.Email("abc@example.com"), auth.Password("123")

	rows, err := s.db.Query(
		"INSERT INTO accounts (email, password) values(?,?) returning user_id",
		email, password.Obfuscate(),
	)
	if err != nil {
		t.Fatalf("Error setting up account")
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&userId)
		if err != nil {
			t.Fatalf("Error setting up account")
		}
		return
	}
	t.Fatalf("Error setting up account")
	return
}
