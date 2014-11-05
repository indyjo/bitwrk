//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013-2014  Jonas Eschenburg <jonas@bitwrk.net>
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
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/indyjo/bitwrk-common/ripemd160"
	"github.com/indyjo/bitwrk-common/bitecdsa"
	"github.com/indyjo/bitwrk-common/bitelliptic"
	"io"
	"math/big"
)

func Digest256(message []byte) (res []byte) {
	sha := sha256.New()
	sha.Write(message)
	res = sha.Sum(res)
	sha.Reset()
	sha.Write(res)
	res = sha.Sum(res[:0])
	return
}

func Digest160(message []byte) (res []byte) {
	sha := sha256.New()
	sha.Write(message)
	res = sha.Sum(res)
	rip := ripemd160.New()
	rip.Write(res)
	res = rip.Sum(res[:0])
	return
}

func byteAt(v uint64, bits uint) byte {
	return byte((v >> bits) & 0xff)
}

// Encodes a uint64 in variable-length encoding as specified in
// https://en.bitcoin.it/wiki/Protocol_specification#Variable_length_integer
func VarUInt64Encode(val uint64) (result []byte) {
	if val < 0xfd {
		result = []byte{byteAt(val, 0)}
	} else if val <= 0xffff {
		result = []byte{0xfd, byteAt(val, 0), byteAt(val, 1)}
	} else if val <= 0xffffffff {
		result = []byte{
			0xfe,
			byteAt(val, 0),
			byteAt(val, 8),
			byteAt(val, 16),
			byteAt(val, 24)}
	} else {
		result = []byte{
			0xff,
			byteAt(val, 0),
			byteAt(val, 8),
			byteAt(val, 16),
			byteAt(val, 24),
			byteAt(val, 32),
			byteAt(val, 40),
			byteAt(val, 48),
			byteAt(val, 56)}
	}
	return
}

func VarIntEncode(val int) []byte {
	return VarUInt64Encode(uint64(val))
}

func SignatureDigest(message string) []byte {
	bytes := make([]byte, 0, 200)
	prefix := "Bitcoin Signed Message:\n"
	bytes = append(bytes, byte(len(prefix)))
	bytes = append(bytes, prefix...)
	bytes = append(bytes, VarIntEncode(len(message))...)
	bytes = append(bytes, message...)

	return Digest256(bytes)
}

func VerifySignatureBase64(message, address, signature_b64 string) error {
	signature, err := base64.StdEncoding.DecodeString(signature_b64)
	if err != nil {
		return err
	}

	return VerifySignature(message, address, signature)
}

// Verifies that a message was signed by the specified address.
func VerifySignature(message, address string, signature []byte) error {
	if len(signature) != 65 {
		return fmt.Errorf("Bad signature length %v (should be 65 bytes)",
			len(signature))
	}

	networkId, _, err := DecodeBitcoinAddress(address)
	if err != nil {
		return err
	}

	sigr := new(big.Int).SetBytes(signature[1:33])
	sigs := new(big.Int).SetBytes(signature[33:65])
	recid := uint(signature[0] - 27)
	compressed := false
	if recid > 3 {
		recid -= 4
		compressed = true
	}

	digest := SignatureDigest(message)

	qx, qy, err := RecoverPubKeyFromSignature(sigr, sigs, digest, bitelliptic.S256(), recid&3)
	if err != nil {
		return fmt.Errorf("Error recovering public key from signature: %v", err)
	}

	if !bitecdsa.Verify(&bitecdsa.PublicKey{bitelliptic.S256(), qx, qy},
		digest, sigr, sigs) {
		return fmt.Errorf("Invalid signature")
	}

	pubkey, err := EncodePublicKey(qx, qy, compressed)
	if err != nil {
		return fmt.Errorf("Error encoding public key: %v", err)
	}

	recoveredAddress := PublicKeyToBitcoinAddress(networkId, pubkey)

	if address != recoveredAddress {
		return fmt.Errorf("Signature doesn't match document %#v", message)
	}

	return nil
}

var ErrCouldNotSign = fmt.Errorf("Couldn't sign message")

func SignMessage(message string, key *bitecdsa.PrivateKey, compressed bool, rand io.Reader) (string, error) {

	digest := SignatureDigest(message)
	r, s, err := bitecdsa.Sign(rand, key, digest)
	if err != nil {
		return "", err
	}
	data := make([]byte, 65)
	rbytes := r.Bytes()
	copy(data[33-len(rbytes):33], rbytes)
	sbytes := s.Bytes()
	copy(data[65-len(sbytes):65], sbytes)

	for recid := 0; recid < 4; recid++ {
		data[0] = byte(27 + recid)
		qx, qy, err := RecoverPubKeyFromSignature(r, s, digest, bitelliptic.S256(), uint(recid))
		if err != nil {
			return "", err
		}
		if 0 == qx.Cmp(key.PublicKey.X) && 0 == qy.Cmp(key.PublicKey.Y) {
			if compressed {
				data[0] += 4
			}
			return base64.StdEncoding.EncodeToString(data), nil
		}
	}
	return "", ErrCouldNotSign
}
