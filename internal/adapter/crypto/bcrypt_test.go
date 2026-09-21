package crypto_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thelong0705/deuce/internal/adapter/crypto"
)

func TestBcryptHasher(t *testing.T) {
	tests := []struct {
		name string
		// password is hashed, compareWith is checked against that hash
		password    string
		compareWith string
		wantMatch   bool
		wantHashErr bool
	}{
		{
			name:        "the same password matches",
			password:    "supersecret",
			compareWith: "supersecret",
			wantMatch:   true,
		},
		{
			name:        "a different password does not match",
			password:    "supersecret",
			compareWith: "wrong password",
		},
		{
			name:        "comparison is case sensitive",
			password:    "supersecret",
			compareWith: "SuperSecret",
		},
		{
			name:        "a prefix of the password does not match",
			password:    "supersecret",
			compareWith: "super",
		},
		{
			name:        "non-ascii passwords round-trip",
			password:    "mật-khẩu-siêu-bí-mật",
			compareWith: "mật-khẩu-siêu-bí-mật",
			wantMatch:   true,
		},
		{
			name:        "72 bytes is the limit and still works",
			password:    strings.Repeat("a", 72),
			compareWith: strings.Repeat("a", 72),
			wantMatch:   true,
		},
		{
			// The domain rejects these before they reach the hasher, but bcrypt
			// refusing them is why that rule exists.
			name:        "73 bytes is rejected by bcrypt",
			password:    strings.Repeat("a", 73),
			wantHashErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := crypto.NewBcryptHasher()

			hash, err := h.Hash(tt.password)

			if tt.wantHashErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotEqual(t, tt.password, hash, "the password must not be stored as-is")

			if tt.wantMatch {
				require.NoError(t, h.Compare(hash, tt.compareWith))
			} else {
				require.Error(t, h.Compare(hash, tt.compareWith))
			}
		})
	}
}

// Not table-driven: this is one property about two hashes of the same input,
// not a matrix of inputs. bcrypt salts every hash, so two must differ --
// identical output would mean the salt was not applied.
func TestBcryptHashesAreSalted(t *testing.T) {
	h := crypto.NewBcryptHasher()

	first, err := h.Hash("supersecret")
	require.NoError(t, err)

	second, err := h.Hash("supersecret")
	require.NoError(t, err)

	require.NotEqual(t, first, second)
	require.NoError(t, h.Compare(first, "supersecret"))
	require.NoError(t, h.Compare(second, "supersecret"))
}
