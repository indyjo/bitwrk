package bitcoin

import (
	"bitelliptic"
	"testing"
)

func Test_DecodePrivateKeyWif(t *testing.T) {
	type positive struct {
		privKey    string
		compressed bool
		address    string
	}

	for _, c := range []positive{
		{"5KJvsngHeMpm884wtkJNzQGaCErckhHJBGFsvd3VyK5qMZXj3hS", false,
			"1JwSSubhmg6iPtRjtyqhUYYH7bZg3Lfy1T"},
		{"L3p8oAcQTtuokSCRHQ7i4MhjWc9zornvpJLfmg62sYpLRJF9woSu", true,
			"1C7zdTfnkzmr13HfA2vNm5SJYRK6nEKyq8"},
	} {
		key, compressed, err := DecodePrivateKeyWIF(c.privKey)
		if err != nil {
			t.Errorf("Unexpected error decoding private key: %v", err)
			return
		}

		if compressed != c.compressed {
			t.Errorf("Compressed flag is %v but should be %v", compressed, c.compressed)
			return
		}

		curve := bitelliptic.S256()
		x, y := curve.ScalarBaseMult(key)

		pubkey, err := EncodePublicKey(x, y, compressed)
		if err != nil {
			t.Errorf("Unexpected error encoding public key: %v", err)
		}

		address := PublicKeyToBitcoinAddress(0, pubkey)
		if address != c.address {
			t.Errorf("Test case failed: %#v", c)
		}
	}
}
