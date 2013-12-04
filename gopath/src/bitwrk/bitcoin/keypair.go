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

// KeyPair keeps all Bitcoin identity data in one place

package bitcoin

import (
	"bitecdsa"
	"bitelliptic"
	"io"
	"math/big"
)

type KeyPair struct {
	privKey     []byte
	compressed  bool
	ecdsakey    *bitecdsa.PrivateKey
	addrVersion byte
	address     string
}

func FromPrivateKeyWIF(key string, addrVersion byte) (*KeyPair, error) {
	if priv, comp, err := DecodePrivateKeyWIF(key); err != nil {
		return nil, err
	} else {
		return FromPrivateKeyRaw(priv, comp, addrVersion)
	}
	return nil, nil // not reached
}

func FromPrivateKeyRaw(privKey []byte, compressed bool, addrVersion byte) (*KeyPair, error) {
	curve := bitelliptic.S256()
	k := KeyPair{
		privKey:     privKey,
		compressed:  compressed,
		addrVersion: addrVersion,
	}

	if ecdsakey, err := bitecdsa.GenerateFromPrivateKey(new(big.Int).SetBytes(k.privKey), curve); err != nil {
		return nil, err
	} else {
		k.ecdsakey = ecdsakey
	}

	if pubkey, err := EncodePublicKey(k.ecdsakey.X, k.ecdsakey.Y, k.compressed); err != nil {
		panic(err) // Shouldn't happen
	} else {
		k.address = PublicKeyToBitcoinAddress(addrVersion, pubkey)
	}

	return &k, nil
}

func (k *KeyPair) GetAddress() string {
	return k.address
}

func (k *KeyPair) SignMessage(message string, rand io.Reader) (string, error) {
	return SignMessage(message, k.ecdsakey, k.compressed, rand)
}

func (k *KeyPair) GetPrivateKeyWIF() (string, error) {
	return EncodePrivateKeyWIF(k.privKey, k.compressed)
}
