//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2014  Jonas Eschenburg <jonas@bitwrk.net>
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

func getMessageOfTheDay(r *http.Request) motd {
	ua := r.Header.Get("User-Agent")
	matches := uaRegexp.FindStringSubmatch(ua)
	if matches == nil {
		return motd{"Welcome to the BitWrk network, stranger!", false}
	}

	major, _ := strconv.ParseInt(matches[1], 10, 16)
	minor, _ := strconv.ParseInt(matches[2], 10, 16)
	micro, _ := strconv.ParseInt(matches[3], 10, 16)

	return motd{fmt.Sprintf("There will be a test of BitWrk today, between 14:00 and 16:00 UTC."+
		" Bitcoins earned during this time period will actually be paid out."+
		" That's why all accounts have been reset to zero - except <a target=\"_blank\" href=\"http://bitwrk.appspot.com/account/1MwvTNehPz7U5XYn3h1G7LVPANv3GFq6JR\">this one</a>."+
		" Please help the BitWrk project and take part in the test!"+
		" For more information <a target=\"_blank\" href=\"https://bitcointalk.org/index.php?topic=780506.0\">click here.</a>"+
		" You are currently running client version %d.%d.%d.",
		major, minor, micro), true}
}
