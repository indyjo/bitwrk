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

package client

import (
	"bitwrk"
	"bitwrk/bitcoin"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

var ErrRedirect = fmt.Errorf("Redirect encountered")

// Disallow redirects (or explicitly handle them)
var client = http.Client{
	CheckRedirect: func(r *http.Request, _ []*http.Request) error {
		return ErrRedirect
	},
}

func newServerRequest(method, relpath string, body io.Reader) (r *http.Request, err error) {
	r, err = http.NewRequest(method, BitwrkUrl+relpath, body)
	if err != nil {
		return
	}
	r.Header.Set("User-Agent", BitwrkUserAgent)
	return r, nil
}

func getFromServer(relpath string) (*http.Response, error) {
	req, err := newServerRequest("GET", relpath, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func GetJsonFromServer(relpath, etag string) (*http.Response, error) {
	req, err := newServerRequest("GET", relpath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	return client.Do(req)
}

func postFormToServer(relpath, query string) (*http.Response, error) {
	req, err := newServerRequest("POST", relpath, strings.NewReader(query))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return client.Do(req)
}

func getStringFromServer(relpath string, limit int64) (string, error) {
	r, err := getFromServer(relpath)
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(&io.LimitedReader{r.Body, limit})
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func DetermineIpAddress() (string, error) {
	return getStringFromServer("myip", 120)
}

func GetNonce() (string, error) {
	return getStringFromServer("nonce", 80)
}

func PlaceBid(bid *bitwrk.RawBid, identity *bitcoin.KeyPair) (bidId string, err error) {
	var nonce string
	if _nonce, err := GetNonce(); err != nil {
		return "", err
	} else {
		nonce = _nonce
	}

	articleString := bid.Article.FormString()
	priceString := normalize(bid.Price.String())
	bidTypeString := bid.Type.FormString()
	document := fmt.Sprintf(
		"article=%s&type=%s&price=%s&address=%s&nonce=%s",
		articleString,
		bidTypeString,
		priceString,
		identity.GetAddress(),
		nonce)
	signature, err := identity.SignMessage(document, rand.Reader)
	if err != nil {
		err = fmt.Errorf("Error signing message: %v", err)
		return
	}

	resp, err := postFormToServer("bid", document+"&signature="+url.QueryEscape(signature))
	if err == nil && resp.StatusCode == http.StatusSeeOther && resp.Header.Get("X-Bid-Key") != "" {
		bidId = resp.Header.Get("X-Bid-Key")
	} else if err == nil {
		var more []byte
		if resp != nil {
			more, _ = ioutil.ReadAll(resp.Body)
		}
		err = fmt.Errorf("Got status: %#v\nResponse: %v", resp.Status, string(more))
	}

	return
}

func normalize(s string) string {
	return url.QueryEscape(strings.Replace(s, " ", "", -1))
}
