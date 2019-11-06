//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013-2019  Jonas Eschenburg <jonas@bitwrk.net>
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

package common

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"

	"github.com/indyjo/bitwrk-common/bitcoin"
)

func getMainConfigDir(name string) (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("Failed to query current user from OS: %v", err)
	}
	return filepath.Join(usr.HomeDir, "."+name), nil
}

func MustLoadOrCreateIdentity(name string, addrVersion byte) *bitcoin.KeyPair {
	result, err := LoadOrCreateIdentity(name, addrVersion)
	if err != nil {
		log.Fatal(err)
	}
	return result
}

func LoadOrCreateIdentity(name string, addrVersion byte) (*bitcoin.KeyPair, error) {
	mainCfgDir := ""
	if d, err := getMainConfigDir(name); err != nil {
		log.Fatal(err)
	} else {
		mainCfgDir = d
	}

	keyFilePath := filepath.Join(mainCfgDir, "privatekey.wif")

	// Try opening the key file
	result, err := LoadIdentityFrom(keyFilePath, addrVersion)

	if os.IsNotExist(err) {
		// Key file doesn't exist -> create a new random key
		result = createRandomKey(addrVersion)
		err = saveIdentity(mainCfgDir, keyFilePath, result)
	} else if err != nil {
		return nil, err
	}

	return result, err
}

func saveIdentity(mainCfgDir string, keyFilePath string, result *bitcoin.KeyPair) error {
	// Make .bitwrk-client directory with secretive permissions. On error, quit.
	err := os.Mkdir(mainCfgDir, os.ModeDir|0700)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("couldn't create configuration directory [%v]: %v", mainCfgDir, err)
	}
	// Make key file with even more secretive permissions. On error, quit.
	outfile, err := os.OpenFile(keyFilePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("couldn't create configuration file [%v]: %v", keyFilePath, err)
	}
	defer func() { _ = outfile.Close() }()
	key, err := result.GetPrivateKeyWIF()
	if err != nil {
		return err
	}
	_, err = outfile.WriteString(key)
	if err != nil {
		return fmt.Errorf("Couldn't write to file: %v", err)
	}
	return outfile.Close()
}

func LoadIdentityFrom(path string, addrVersion byte) (*bitcoin.KeyPair, error) {
	infile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	// Key file exists -> read and parse. On error, quit.
	defer func() { _ = infile.Close() }()
	if encoded, err := ioutil.ReadAll(infile); err != nil {
		return nil, fmt.Errorf("error reading from %#v: %v", path, err)
	} else if key, err := bitcoin.FromPrivateKeyWIF(string(encoded), bitcoin.AddrVersionBitcoin); err != nil {
		return nil, fmt.Errorf("error creating key: %v", err)
	} else {
		return key, nil
	}
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
