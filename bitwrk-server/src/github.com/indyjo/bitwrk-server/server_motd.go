//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2014-2017  Jonas Eschenburg <jonas@bitwrk.net>
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
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
)

type motd struct {
	Text    string
	Warning bool
}

// Handler for the /motd URL path. Returns a JSON document containing a message for the user.
func handleMessageOfTheDay(w http.ResponseWriter, r *http.Request) {
	m := getMessageOfTheDay(r)

	bytes, err := json.Marshal(m)
	if err != nil {
		http.Error(w, "Error marshaling motd", http.StatusInternalServerError)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(bytes)
	}
}

var uaRegexp = regexp.MustCompile("^BitWrkGoClient/([0-9]{1,4})\\.([0-9]{1,4})\\.([0-9]{1,4})")

// Returns a message based on analyzing the HTTP "User-Agent" header.
func getMessageOfTheDay(r *http.Request) motd {
	ua := r.Header.Get("User-Agent")
	matches := uaRegexp.FindStringSubmatch(ua)
	if matches == nil {
		return motd{"Welcome to the BitWrk network, stranger!", false}
	}

	major, _ := strconv.ParseInt(matches[1], 10, 16)
	minor, _ := strconv.ParseInt(matches[2], 10, 16)
	micro, _ := strconv.ParseInt(matches[3], 10, 16)

	const currentMajor = 0
	const currentMinor = 6
	const currentMicro = 1

	if major > currentMajor || major == currentMajor && (minor > currentMinor || minor == currentMinor && micro >= currentMicro) {
		return motd{fmt.Sprintf("Welcome to the BitWrk network!"+
			" Your client is up to date (version %d.%d.%d).", major, minor, micro), false}
	} else {
		return motd{fmt.Sprintf("BitWrk proudly announces version %v.%v.%v!"+
			" You are currently running client version %d.%d.%d."+
			" For information on what's new and how to upgrade please visit"+
			" <a target=\"_blank\" href=\"https://bitwrk.net/\">bitwrk.net</a>.",
			currentMajor, currentMinor, currentMicro, major, minor, micro), true}
	}
}
