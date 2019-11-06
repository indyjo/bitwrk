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

package main

import (
	"crypto/rand"
	"fmt"
	"log"

	"github.com/indyjo/bitwrk-common/bitcoin"
	"github.com/indyjo/bitwrk-common/bitwrk"
	"github.com/indyjo/bitwrk-common/protocol"
)

func cmdRelation(identity *bitcoin.KeyPair, args []string) error {
	if len(args) != 4 {
		return fmt.Errorf("Wrong number of arguments for relation. Expected: 4, got: %v.", len(args))
	}

	nonce, err := protocol.GetNonce()
	if err != nil {
		return fmt.Errorf("failed to get nonce: %v", err)
	}
	relation, err := bitwrk.ParseRelation(args[3], nonce, identity.GetAddress(), args[2], args[1], "")
	if err != nil {
		return err
	}

	log.Printf("Setting relation: %v", relation)

	err = relation.SignWith(identity, rand.Reader, nonce)
	if err != nil {
		return err
	}

	return protocol.SendRelation(relation)
}
