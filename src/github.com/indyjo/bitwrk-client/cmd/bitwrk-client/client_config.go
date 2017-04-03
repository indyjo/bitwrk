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

package main

import (
	"crypto/rand"
	"fmt"
	"github.com/indyjo/bitwrk-common/bitcoin"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path"
)

func getMainConfigDir(name string) (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("Failed to query current user from OS: %v", err)
	}
	return path.Join(usr.HomeDir, "."+name), nil
}

func LoadOrCreateIdentity(name string, addrVersion byte) *bitcoin.KeyPair {
	mainCfgDir := ""
	if d, err := getMainConfigDir(name); err != nil {
		log.Fatal(err)
	} else {
		mainCfgDir = d
	}

	keyFilePath := path.Join(mainCfgDir, "privatekey.wif")

	// Try to open key file
	if infile, err := os.Open(keyFilePath); err != nil {
		if !os.IsNotExist(err) {
			// On a real failure, stop here
			log.Fatal(err)
		}

		// Key file doesn't exist -> create a new random key
		result := createRandomKey(addrVersion)

		// Make .bitwrk-client directory with secretive permissions. On error, quit.
		if err := os.Mkdir(mainCfgDir, os.ModeDir|0700); err != nil && !os.IsExist(err) {
			log.Fatalf("Couldn't create configuration directory [%v]: %v", mainCfgDir, err)
		}

		// Make key file with even more secretive permissions. On error, quit.
		if outfile, err := os.OpenFile(keyFilePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_EXCL, 0600); err != nil {
			log.Fatalf("Couldn't create configuration file [%v]: %v", keyFilePath, err)
		} else {
			defer outfile.Close()
			if key, err := result.GetPrivateKeyWIF(); err != nil {
				log.Fatalf("Couldn't export WIF key: %v", err)
			} else if _, err := outfile.WriteString(key); err != nil {
				log.Fatalf("Couldn't write to file: %v", err)
			}
		}

		return result
	} else {
		// Key file exists -> read and parse. On error, quit.
		defer infile.Close()
		if encoded, err := ioutil.ReadAll(infile); err != nil {
			log.Fatalf("Error reading from %#v: %v", keyFilePath, err)
		} else if key, err := bitcoin.FromPrivateKeyWIF(string(encoded), bitcoin.AddrVersionBitcoin); err != nil {
			log.Fatalf("Error creating key: %v", err)
		} else {
			return key
		}
	}
	log.Fatal("Unreachable")
	return nil
}

func createRandomKey(addrVersion byte) *bitcoin.KeyPair {
	data := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, data); err != nil {
		log.Fatalf("Error generating random key: %v", err)
	}
	if key, err := bitcoin.FromPrivateKeyRaw(data, true, addrVersion); err != nil {
		log.Fatalf("Error creating key: %v", err)
	} else {
		return key
	}
	log.Panic("Unreachable")
	return nil
}
