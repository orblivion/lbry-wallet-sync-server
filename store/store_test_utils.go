package store

import (
	"io/ioutil"
	"os"
	"testing"
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
