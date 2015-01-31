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

package server

import (
	"fmt"
	"github.com/indyjo/bitwrk-common/bitcoin"
)

// Check whether a bitcoin address is from the 'right' network,
// i.e. main or test network (depends on config)
func checkBitcoinAddress(address string) error {
	networkId, _, err := bitcoin.DecodeBitcoinAddress(address)
	if err != nil {
		return err
	}

	if networkId != CfgBitcoinNetworkId {
		return fmt.Errorf("Invalid bitcoin network id %v in address %#v."+
			" This server accepts %v.", networkId, address, CfgBitcoinNetworkId)
	}

	return nil
}
