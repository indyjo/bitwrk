//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013  Jonas Eschenburg <jonas@bitwrk.net>
//
//  This program is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with this program.  If not, see <http://www.gnu.org/licenses/>.
package bitcoin

import (
	"bitecdsa"
	"bitelliptic"
	"bytes"
	"math/big"
	"testing"
)

var privKey = "5JeWZ1z6sRcLTJXdQEDdB986E6XfLAkj9CgNE4EHzr5GmjrVFpf"
var plaintext = "C'est par mon ordre et pour le bien de l'Etat que le porteur" +
	" du présent a fait ce qu'il a fait."
var digest = []byte{
	245, 80, 4, 87, 187, 108, 33, 247,
	126, 90, 90, 40, 24, 204, 230, 60,
	249, 249, 10, 245, 1, 199, 52, 117,
	231, 244, 17, 233, 74, 12, 172, 228}
var signature = "HDTbD+paOyf7BJ3Fwlvu3Ul+goCQqZF7mfijfva5CkX6NpXrKN7wi9O/MQ/a" +
	"EWFcXVrSBQXYyTb4FuvPdKn0gmw="
var address = "17mDAmveV5wBwxajBsY7g1trbMW1DVWcgL"

func Test_SignatureDigest(t *testing.T) {
	result := SignatureDigest(plaintext)
	if !bytes.Equal(result, digest) {
		t.Errorf("Bad result from SignatureDigest: %v", result)
	}
}

func Test_VerifySignature(t *testing.T) {
	type testcase struct {
		plaintext, address, signature string
	}
	for i, c := range []testcase{
		{plaintext, address, signature},
		{"test",
			"13mgtNQiU7gCWpfmypGfijyX9jzqpPr6co",
			"IJlJB/iYZ/j/Edh2EBLrHfBD5E1FF10Gf0QuS/jZWsHzHswQSxTRCBNQ7oiB939COu53jfOOuBe+tW8wJ80WXRs="},
		{"test2",
			"13mgtNQiU7gCWpfmypGfijyX9jzqpPr6co",
			"H9i6klOr/QYanX+0KGFBImAJ/g4JMqqjFvA+4+/QnCk0WorcBvgll378N3cmwKNAfSpEIDjT5/xf4bSt/tnNn9A="},
		{"article=fnord&type=buy&price=mBTC1.00&address=13mgtNQiU7gCWpfmypGfijyX9jzqpPr6co&nonce=b58d10ad7eaf93e4246c069830e6e27f",
			"13mgtNQiU7gCWpfmypGfijyX9jzqpPr6co",
			"IM60m+j5YvuKNUAu23Si9FQeMdwtzuajh2wCGGQ3U3kULM3dsya9hyJfJnzGpAXy9kKlC/STnhrcVuBy4WSuVLM="},
		{"article=fnord&type=buy&price=mBTC1.00&address=13mgtNQiU7gCWpfmypGfijyX9jzqpPr6co&nonce=ab934e9f221e5b64595a24e3343600f1",
			"13mgtNQiU7gCWpfmypGfijyX9jzqpPr6co",
			"IJcnLRN4oz9dAL0FSjEUSDU3uYml22+7GomNfXaj3cydiAuqn4JvKak3lvNw8C7TkvKMW2oqwPj85VeHYjwYiNw="},
	} {
		err := VerifySignatureBase64(c.plaintext, c.address, c.signature)
		if err != nil {
			t.Errorf("Unexpected signature verification error: %v", err)
		} else {
			t.Logf("Signature %v verified successfully", i)
		}
	}
}

func Test_SignMessage(t *testing.T) {
	type testcase struct {
		plaintext, privKeyEnc, address string
	}

	for _, c := range []testcase{
		// correct horse battery staple from brainwallet.org
		{plaintext, "5KJvsngHeMpm884wtkJNzQGaCErckhHJBGFsvd3VyK5qMZXj3hS", "1JwSSubhmg6iPtRjtyqhUYYH7bZg3Lfy1T"},
		{plaintext, "L3p8oAcQTtuokSCRHQ7i4MhjWc9zornvpJLfmg62sYpLRJF9woSu", "1C7zdTfnkzmr13HfA2vNm5SJYRK6nEKyq8"},
	} {
		privKeyBytes, compressed, err := DecodePrivateKeyWIF(c.privKeyEnc)
		if err != nil {
			t.Errorf("Couldn't decode private key: %v", err)
		}
		privKey, err := bitecdsa.GenerateFromPrivateKey(new(big.Int).SetBytes(privKeyBytes), bitelliptic.S256())
		if err != nil {
			t.Errorf("Couldn't generate public key: %v", err)
		}
		zeros := bytes.NewReader(make([]byte, 256))
		signature, err := SignMessage(c.plaintext, privKey, compressed, zeros)
		t.Logf("Signature of %#v using private key %#v (compressed=%v): %#v",
			c.plaintext, c.privKeyEnc, compressed, signature)
		err = VerifySignatureBase64(c.plaintext, c.address, signature)
		if err != nil {
			t.Errorf("Couldn't verify signature: %v", err)
		}

		err = VerifySignatureBase64(c.plaintext+"-", c.address, signature)
		if err == nil {
			t.Errorf("Falsely verified signature!")
		}
	}
}
