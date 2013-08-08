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
	"base58"
	"bytes"
	"fmt"
	"math/big"
)

const (
	AddrVersionBitcoin byte = 0
	AddrVersionTestnet byte = 0x6f
)

func EncodePublicKey(x, y *big.Int, compressed bool) ([]byte, error) {
	var pubkey []byte
	if compressed {
		pubkey = make([]byte, 33)
		pubkey[0] = 2 + byte(y.Bit(0))
	} else {
		pubkey = make([]byte, 65)
		pubkey[0] = 4
	}

	// Right-align x coordinate
	bytes := x.Bytes()
	if len(bytes) > 32 {
		return nil, fmt.Errorf("Value of x has > 32 bytes")
	}
	copy(pubkey[1+(32-len(bytes)):33], bytes)

	if !compressed {
		// Right-align y coordinate
		bytes = y.Bytes()
		if len(bytes) > 32 {
			return nil, fmt.Errorf("Value of y has > 32 bytes")
		}
		copy(pubkey[33+(32-len(bytes)):65], bytes)
	}

	return pubkey, nil
}

func PublicKeyToBitcoinAddress(addrVersion byte, pubkey []byte) string {
	digest := Digest160(pubkey)

	bytes := append(make([]byte, 0, 21), addrVersion)
	bytes = append(bytes, digest...) // Prepend network id byte

	check := Digest256(bytes)

	return string(base58.EncodeBase58(append(bytes, check[0:4]...)))
}

func DecodeBitcoinAddress(address string) (addrVersion byte, pubkeyHash []byte, err error) {
	data := base58.DecodeBase58([]byte(address))
	if data == nil {
		return 0, nil, fmt.Errorf("Invalid base58-encoded bitcoin address: %#v", address)
	}
	if len(data) != 25 {
		return 0, nil, fmt.Errorf("Wrong size for bitcoin address: %v bytes", len(data))
	}

	check := Digest256(data[0:21])

	addrVersion = data[0]
	pubkeyHash = data[1:17]

	if !bytes.Equal(check[0:4], data[21:25]) {
		err = fmt.Errorf("Checksum test failed for bitcoin address: %#v", address)
	}

	return
}

func DecodeWIF(encoded string) (version byte, payload []byte, err error) {
	data := base58.DecodeBase58([]byte(encoded))
	if data == nil || len(data) < 25 {
		return 0, nil, fmt.Errorf("Invalid Wallet Import Format data: %#v", encoded)
	}

	check := Digest256(data[0 : len(data)-4])

	version = data[0]
	payload = data[1 : len(data)-4]

	if !bytes.Equal(check[0:4], data[len(data)-4:]) {
		err = fmt.Errorf("Checksum test failed for Wallet Import Format data: %#v", encoded)
	}

	return
}

func EncodeWIF(version byte, payload []byte) (string, error) {
	// Prepend version byte
	data := append(make([]byte, 0, len(payload)+5), version)
	data = append(data, payload...)

	check := Digest256(data)

	// append checksum
	data = append(data, check[0:4]...)
	return string(base58.EncodeBase58(data)), nil
}

func DecodePrivateKeyWIF(encoded string) (key []byte, compressed bool, err error) {
	version, payload, err := DecodeWIF(encoded)
	if err != nil {
		return
	}

	if version != 128 {
		err = fmt.Errorf("Wrong version id %v. Expected: 128", version)
		return
	}
	if len(payload) != 32 && len(payload) != 33 {
		err = fmt.Errorf("Wrong payload length %v. Expected: 32 or 33", len(payload))
		return
	}
	if len(payload) == 33 && payload[32] != 1 {
		err = fmt.Errorf("Invalid last byte of payload %v. Expected: 1", payload[32])
		return
	}
	key = payload[:32]
	compressed = len(payload) == 33
	return
}

func EncodePrivateKeyWIF(key []byte, compressed bool) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("Invalid key length: %v (extpected 32)", len(key))
	}
	data := make([]byte, 0, 33)
	data = append(data, key...)
	if compressed {
		data = append(data, 1)
	}
	return EncodeWIF(128, data)
}
