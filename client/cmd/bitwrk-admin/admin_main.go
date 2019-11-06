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
	"flag"
	"log"
	"os"

	"github.com/indyjo/bitwrk/common/bitcoin"
	"github.com/indyjo/bitwrk/common/protocol"
	"github.com/indyjo/bitwrk/client/common"
)

const ToolName = "bitwrk-admin"

func main() {
	log.Printf("%v %v %v", ToolName, common.ClientVersion, common.CommitSHA)
	protocol.BitwrkUserAgent = ToolName + "/" + common.ClientVersion

	flags := flag.NewFlagSet(ToolName, flag.ExitOnError)

	var bitwrkUrl string
	flags.StringVar(&bitwrkUrl, "bitwrkurl", "http://bitwrk.appspot.com/",
		"URL to contact the bitwrk service at")

	var identityFile string
	flags.StringVar(&identityFile, "identity", "",
		"WIF file to read private key from")

	err := flags.Parse(os.Args[1:])
	if err == flag.ErrHelp {
		flags.Usage()
	} else if err != nil {
		log.Fatalf("Error parsing command line: %v", err)
	}

	var identity *bitcoin.KeyPair
	if identityFile == "" {
		identity = common.MustLoadOrCreateIdentity("bitwrk-client", bitcoin.AddrVersionBitcoin)
	} else if kp, err := common.LoadIdentityFrom(identityFile, bitcoin.AddrVersionBitcoin); err != nil {
		log.Fatalf("Can't load identity from [%v]: %v", identityFile, err)
	} else {
		identity = kp
	}

	log.Printf("Bitwrk URL: %v", bitwrkUrl)
	protocol.BitwrkUrl = bitwrkUrl

	log.Printf("Identity: %v", identity.GetAddress())

	args := flags.Args()

	var command func() error
	if len(args) == 0 {
		command = listCommandsAndExit
	} else if args[0] == "info" {
		command = cmdInfo
	} else if args[0] == "relation" {
		command = func() error { return cmdRelation(identity, args) }
	} else {
		command = listCommandsAndExit
	}

	err = command()
	if err != nil {
		log.Fatalf("Failure: %v", err)
	} else {
		log.Print("Success")
	}
}

func listCommandsAndExit() error {
	log.Print("Valid commands:")
	log.Print("  info")
	log.Print("     Just print info about arguments and account and quit.")
	log.Print("  relation (trusts|worksfor) <target participant> (true|false)")
	log.Print("     Updates a relation between the current and another participant.")
	os.Exit(1)
	return nil
}

func cmdInfo() error {
	return nil
}
