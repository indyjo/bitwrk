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

package main

import (
	"bitecdsa"
	"bitelliptic"
	"bitwrk/bitcoin"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"time"
)

var ExternalAddress string
var ExternalPort int
var InternalPort int
var BitcoinPrivateKeyEncoded string
var BitcoinPrivateKey []byte
var BitcoinKeyCompressed bool
var BitcoinAddress string
var ECDSAKey *bitecdsa.PrivateKey

func decodeBitcoinKey(privKey string) (key []byte, compressed bool, address string, err error) {
	key, compressed, err = bitcoin.DecodePrivateKeyWIF(privKey)
	if err != nil {
		err = fmt.Errorf("Error decoding private key: %v", err)
		return
	}

	curve := bitelliptic.S256()
	x, y := curve.ScalarBaseMult(key)

	pubkey, err := bitcoin.EncodePublicKey(x, y, compressed)
	if err != nil {
		err = fmt.Errorf("Error encoding public key: %v", err)
	}

	address = bitcoin.PublicKeyToBitcoinAddress(0, pubkey)
	return
}

func makeECDSAKey(key []byte) (*bitecdsa.PrivateKey, error) {
	result, err := bitecdsa.GenerateFromPrivateKey(new(big.Int).SetBytes(key), bitelliptic.S256())
	if err != nil {
		return nil, err
	}
	return result, nil
}

func main() {
	flags := flag.NewFlagSet("", flag.ExitOnError)
	flags.StringVar(&ExternalAddress, "extaddr", "auto",
		"IP address or name this host can be reached under from the internet")
	flags.IntVar(&ExternalPort, "extport", -1, "Port that can be reached from the Internet")
	flags.IntVar(&InternalPort, "intport", 8081, "Maintenance port for admin interface")
	flags.StringVar(&BitcoinPrivateKeyEncoded, "bitcoinprivkey",
		"random",
		"The private key of the Bitcoin address to use for authentication")
	err := flags.Parse(os.Args[1:])
	if err == flag.ErrHelp {
		flags.Usage()
	} else if err != nil {
		log.Fatalf("Error parsing command line: %v", err)
		os.Exit(1)
	}

	if BitcoinPrivateKeyEncoded == "random" {
		data := make([]byte, 32)
		if _, err := rand.Reader.Read(data); err != nil {
			log.Fatalf("Error generating random key: %v", err)
			os.Exit(1)
		}
		if key, err := bitcoin.EncodePrivateKeyWIF(data, true); err != nil {
			log.Fatalf("Error encoding random key: %v", err)
			os.Exit(1)
		} else {
			BitcoinPrivateKeyEncoded = key
		}
	}

	BitcoinPrivateKey, BitcoinKeyCompressed, BitcoinAddress, err = decodeBitcoinKey(BitcoinPrivateKeyEncoded)
	if err != nil {
		log.Fatalf("Error decoding Bitcoin key: %v", err)
		os.Exit(1)
	}
	ECDSAKey, err = makeECDSAKey(BitcoinPrivateKey)
	if err != nil {
		log.Fatalf("Error decoding Bitcoin key for ECDSA: %v", err)
		os.Exit(1)
	}

	_ = BitcoinPrivateKey
	_ = BitcoinKeyCompressed

	if ExternalAddress == "auto" {
		ExternalAddress, err = DetermineIpAddress()
		if err != nil {
			log.Fatalf("Error auto-determining IP address: %v", err)
			os.Exit(1)
		}
	}

	log.Printf("External address: %v\n", ExternalAddress)
	log.Printf("External port: %v\n", ExternalPort)
	log.Printf("Internal port: %v\n", InternalPort)
	log.Printf("Bitcoin address: %v\n", BitcoinAddress)

	exit := make(chan error)
	if InternalPort > 0 {
		go serveInternal(exit)
	}

	if ExternalPort > 0 {
		go serveExternal(exit)
	}

	TestPlaceBid()
	os.Exit(0)

	err = <-exit
	if err != nil {
		log.Fatalf("Exiting because of: %v", err)
		os.Exit(1)
	}
}

func serveInternal(exit chan<- error) {
	mux := http.NewServeMux()
	s := &http.Server{
		Addr:         fmt.Sprintf("localhost:%v", InternalPort),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello World!")
	})
	exit <- s.ListenAndServe()
}

func serveExternal(exit chan<- error) {
	mux := http.NewServeMux()
	s := &http.Server{
		Addr:         fmt.Sprintf(":%v", ExternalPort),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	exit <- s.ListenAndServe()
}
