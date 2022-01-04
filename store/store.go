package store

// TODO - DeviceID - What about clients that lie about deviceID? Maybe require a certain format to make sure it gives a real value? Something it wouldn't come up with by accident.

import (
	"database/sql"

	"errors"
	"fmt"
	"github.com/mattn/go-sqlite3"
	"log"
	"orblivion/lbry-id/auth"
	"time"
)

var (
	ErrDuplicateToken = fmt.Errorf("Token already exists for this user and device")
	ErrNoToken        = fmt.Errorf("Token does not exist for this user and device")

	ErrDuplicateWalletState = fmt.Errorf("WalletState already exists for this user")
	ErrNoWalletState        = fmt.Errorf("WalletState does not exist for this user at this sequence")

	ErrDuplicateEmail   = fmt.Errorf("Email already exists for this user")
	ErrDuplicateAccount = fmt.Errorf("User already has an account")

	ErrNoPubKey = fmt.Errorf("Public Key not found with these credentials")
)

// For test stubs
type StoreInterface interface {
	SaveToken(*auth.AuthToken) error
	GetToken(auth.PublicKey, string) (*auth.AuthToken, error)
	SetWalletState(auth.PublicKey, string, int, auth.Signature, auth.DownloadKey) (string, auth.Signature, bool, error)
	GetWalletState(auth.PublicKey) (string, auth.Signature, error)
	GetPublicKey(string, auth.DownloadKey) (auth.PublicKey, error)
	InsertEmail(auth.PublicKey, string) (err error)
}

type Store struct {
	db *sql.DB
}

func (s *Store) Init(fileName string) {
	db, err := sql.Open("sqlite3", fileName)
	if err != nil {
		log.Fatal(err)
	}
	s.db = db
}

func (s *Store) Migrate() error {
	// We store `sequence` as a seprate field in the `wallet_state` table, even
	// though it's also saved as part of the `walle_state_blob` column. We do
	// this for transaction safety. For instance, let's say two different clients
	// are trying to update the sequence from 5 to 6. The update command will
	// specify "WHERE sequence=5". Only one of these commands will succeed, and
	// the other will get back an error message.

	// TODO does it actually fail with empty "NOT NULL" fields?
	query := `
		CREATE TABLE IF NOT EXISTS auth_tokens(
			token TEXT NOT NULL,
			public_key TEXT NOT NULL,
			device_id TEXT NOT NULL,
			scope TEXT NOT NULL,
			expiration DATETIME NOT NULL,
			PRIMARY KEY (device_id)
		);
		CREATE TABLE IF NOT EXISTS wallet_states(
			public_key TEXT NOT NULL,
			wallet_state_blob TEXT NOT NULL,
			sequence INTEGER NOT NULL,
			signature INTEGER NOT NULL,
			download_key TEXT NOT NULL,
			PRIMARY KEY (public_key)
		);
		CREATE TABLE IF NOT EXISTS accounts(
			email TEXT NOT NULL UNIQUE,
			public_key TEXT NOT NULL,
			PRIMARY KEY (public_key),
			FOREIGN KEY (public_key) REFERENCES wallet_states(public_key)
		);
	`

	_, err := s.db.Exec(query)
	return err
}

////////////////
// Auth Token //
////////////////

func (s *Store) GetToken(pubKey auth.PublicKey, deviceID string) (*auth.AuthToken, error) {
	expirationCutoff := time.Now().UTC()

	rows, err := s.db.Query(
		"SELECT * FROM auth_tokens WHERE public_key=? AND device_id=? AND expiration>?",
		pubKey, deviceID, expirationCutoff,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authToken auth.AuthToken
	for rows.Next() {

		err := rows.Scan(
			&authToken.Token,
			&authToken.PubKey,
			&authToken.DeviceID,
			&authToken.Scope,
			&authToken.Expiration,
		)

		if err != nil {
			return nil, err
		}
		return &authToken, nil
	}
	return nil, ErrNoToken // TODO - will need to test
}

func (s *Store) insertToken(authToken *auth.AuthToken, expiration time.Time) (err error) {
	_, err = s.db.Exec(
		"INSERT INTO auth_tokens (token, public_key, device_id, scope, expiration) values(?,?,?,?,?)",
		authToken.Token, authToken.PubKey, authToken.DeviceID, authToken.Scope, expiration,
	)

	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		// I initially expected to need to check for ErrConstraintUnique.
		// Maybe for psql it will be?
		if errors.Is(sqliteErr.ExtendedCode, sqlite3.ErrConstraintPrimaryKey) {
			err = ErrDuplicateToken
		}
	}

	return
}

func (s *Store) updateToken(authToken *auth.AuthToken, experation time.Time) (err error) {
	res, err := s.db.Exec(
		"UPDATE auth_tokens SET token=?, expiration=?, scope=? WHERE public_key=? AND device_id=?",
		authToken.Token, experation, authToken.Scope, authToken.PubKey, authToken.DeviceID,
	)
	if err != nil {
		return
	}

	numRows, err := res.RowsAffected()
	if err != nil {
		return
	}
	if numRows == 0 {
		err = ErrNoToken
	}
	return
}

func (s *Store) SaveToken(token *auth.AuthToken) (err error) {
	// TODO: For psql, do upsert here instead of separate insertToken and updateToken functions

	// TODO - Should we auto-delete expired tokens?

	expiration := time.Now().UTC().Add(time.Hour * 24 * 14)

	// This is most likely not the first time calling this function for this
	// device, so there's probably already a token in there.
	err = s.updateToken(token, expiration)

	if err == ErrNoToken {
		// If we don't have a token already saved, insert a new one:
		err = s.insertToken(token, expiration)

		if err == ErrDuplicateToken {
			// By unlikely coincidence, a token was created between trying `updateToken`
			// and trying `insertToken`. At this point we can safely `updateToken`.
			err = s.updateToken(token, expiration)
		}
	}
	if err == nil {
		token.Expiration = &expiration
	}
	return
}

/////////////////////////////////
// Wallet State / Download Key //
/////////////////////////////////

func (s *Store) GetWalletState(pubKey auth.PublicKey) (walletStateJSON string, signature auth.Signature, err error) {
	rows, err := s.db.Query(
		"SELECT wallet_state_blob, signature FROM wallet_states WHERE public_key=?",
		pubKey,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(
			&walletStateJSON,
			&signature,
		)
		return
	}
	err = ErrNoWalletState
	return
}

func (s *Store) insertFirstWalletState(
	pubKey auth.PublicKey,
	walletStateJSON string,
	signature auth.Signature,
	downloadKey auth.DownloadKey,
) (err error) {
	// The database will enforce that this will not be set if this user already
	//   has a walletState
	// If it fails, we return ErrDuplicateWalletState below
	_, err = s.db.Exec(
		"INSERT INTO wallet_states (public_key, wallet_state_blob, sequence, signature, download_key) values(?,?,?,?,?)",
		pubKey, walletStateJSON, 1, signature, downloadKey.Obfuscate(),
	)

	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		// I initially expected to need to check for ErrConstraintUnique.
		// Maybe for psql it will be?
		if errors.Is(sqliteErr.ExtendedCode, sqlite3.ErrConstraintPrimaryKey) {
			err = ErrDuplicateWalletState
		}
	}

	return
}

func (s *Store) updateWalletStateToSequence(
	pubKey auth.PublicKey,
	walletStateJSON string,
	sequence int,
	signature auth.Signature,
	downloadKey auth.DownloadKey,
) (err error) {
	// Use the database to enforce that we only update if we are incrementing the sequence
	// If it fails, we return ErrNoWalletState below
	res, err := s.db.Exec(
		"UPDATE wallet_states SET wallet_state_blob=?, sequence=?, signature=?, download_key=? WHERE public_key=? AND sequence=?",
		walletStateJSON, sequence, signature, downloadKey.Obfuscate(), pubKey, sequence-1,
	)
	if err != nil {
		return
	}

	numRows, err := res.RowsAffected()
	if err != nil {
		return
	}
	if numRows == 0 {
		err = ErrNoWalletState
	}
	return
}

// Assumption: walletState has been validated (sequence >=1, etc)
// Assumption: Sequence matches walletState.Sequence()
// Sequence is only passed in here to avoid deserializing walletStateJSON again
// walletState itself is not passed in because we need the exact signed string
func (s *Store) SetWalletState(
	pubKey auth.PublicKey,
	walletStateJSON string,
	sequence int,
	signature auth.Signature,
	downloadKey auth.DownloadKey,
) (latestWalletStateJSON string, latestSignature auth.Signature, updated bool, err error) {
	if sequence == 1 {
		// If sequence == 1, the client assumes that this is our first
		// walletState. Try to insert. If we get a conflict, the client
		// assumed incorrectly and we proceed below.
		err = s.insertFirstWalletState(pubKey, walletStateJSON, signature, downloadKey)
		if err == nil {
			latestWalletStateJSON = walletStateJSON
			latestSignature = signature
			updated = true
			return
		} else if err != ErrDuplicateWalletState {
			return
		}
	} else {
		// If sequence > 1, the client assumes that it is replacing walletState
		// with sequence - 1. Explicitly try to update the walletState with
		// sequence - 1. If we updated no rows, the client assumed incorrectly
		// and we proceed below.
		err = s.updateWalletStateToSequence(pubKey, walletStateJSON, sequence, signature, downloadKey)
		if err == nil {
			latestWalletStateJSON = walletStateJSON
			latestSignature = signature
			updated = true
			return
		} else if err != ErrNoWalletState {
			return
		}
	}

	// We failed to update above. Perhaps the client was unaware of an update
	// done by another client. Let's send the latest version right away so it
	// can take care of it.
	latestWalletStateJSON, latestSignature, err = s.GetWalletState(pubKey)
	return

	// ... wait...
	// need walletstate json because it's signed
	// need... signature too.
	// need to json deserialize to compare to the one we "get"
	// hmm....
	// unless the only thing we compare is the Sequence? which is saved. may avoid needing to deserialize here if we just need that. But if we need to compare the lastSynced thing, it may not be avoided.
	// but who cares?
	// maybe we just deserialize in auth too later.
	return
}

func (s *Store) GetPublicKey(email string, downloadKey auth.DownloadKey) (pubKey auth.PublicKey, err error) {
	rows, err := s.db.Query(
		`SELECT ws.public_key from wallet_states ws INNER JOIN accounts a
     ON a.public_key=ws.public_key
     WHERE email=? AND download_key=?`,
		email, downloadKey.Obfuscate(),
	)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&pubKey)
		return
	}
	err = ErrNoPubKey
	return
}

///////////
// Email //
///////////

func (s *Store) InsertEmail(pubKey auth.PublicKey, email string) (err error) {
	_, err = s.db.Exec(
		"INSERT INTO accounts (public_key, email) values(?,?)",
		pubKey, email,
	)

	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		// I initially expected to need to check for ErrConstraintUnique.
		// Maybe for psql it will be?
		if errors.Is(sqliteErr.ExtendedCode, sqlite3.ErrConstraintPrimaryKey) {
			err = ErrDuplicateEmail
		}
		if errors.Is(sqliteErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
			err = ErrDuplicateAccount
		}
	}

	return
}
