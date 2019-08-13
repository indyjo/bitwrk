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

// Package util provides simple utility functions for the App Engine bitWrk server
package util

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/indyjo/bitwrk-common/bitcoin"
	"github.com/indyjo/bitwrk/server/config"
)

// Check whether a bitcoin address is from the 'right' network,
// i.e. main or test network (depends on config)
func CheckBitcoinAddress(address string) error {
	networkId, _, err := bitcoin.DecodeBitcoinAddress(address)
	if err != nil {
		return err
	}

	if networkId != config.CfgBitcoinNetworkId {
		return fmt.Errorf("Invalid bitcoin network id %v in address %#v."+
			" This server accepts %v.", networkId, address, config.CfgBitcoinNetworkId)
	}

	return nil
}

var blenderRegexp = regexp.MustCompile(`^(net\.bitwrk/blender/0/2\.(69|7[0-9])/(512M|2G|8G|32G))$`)

func CheckArticle(_ context.Context, article string) error {
	switch article {
	case "fnord", "snafu", "foobar",
		"net.bitwrk/gorays/0":
		// TODO: add real article management
	default:
		if blenderRegexp.MatchString(article) {
		} else {
			return fmt.Errorf("Article not traded here: %#v", article)
		}
	}

	return nil
}

// Given a string in format host, host:port or [host]:port, returns the host part.
func StripPort(hostport string) string {
	if i := strings.IndexByte(hostport, ']'); i != -1 {
		return strings.TrimPrefix(hostport[:i], "[")
	}
	colon := strings.IndexByte(hostport, ':')
	if colon == -1 {
		// No colon, i.e. IP (v4) only
		return hostport
	}
	if colon == strings.LastIndexByte(hostport, ':') {
		// IPv4 case, only one colon to separate IP and port
		return hostport[:colon]
	} else {
		// IPv6 case, more than one colon, but no port number
		return hostport
	}
}
