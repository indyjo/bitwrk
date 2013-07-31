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
	"bitwrk"
	"bitwrk/bitcoin"
	"bitwrk/money"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
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

func getJsonFromServer(relpath, etag string) (*http.Response, error) {
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

func PlaceBid(bidType bitwrk.BidType, article bitwrk.ArticleId, price money.Money) (bidId string, err error) {
	nonce, err := GetNonce()

	articleString := article.FormString()
	priceString := normalize(price.String())
	bidTypeString := bidType.FormString()
	document := fmt.Sprintf(
		"article=%s&type=%s&price=%s&address=%s&nonce=%s",
		articleString,
		bidTypeString,
		priceString,
		BitcoinAddress,
		nonce)
	signature, err := bitcoin.SignMessage(document, ECDSAKey, BitcoinKeyCompressed, rand.Reader)
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

func TestPlaceBid() {
	randombyte := make([]byte, 1)
	rand.Read(randombyte)
	buyOrSell := bitwrk.Buy
	if randombyte[0] > 127 {
		buyOrSell = bitwrk.Sell
	}
	bidId, err := PlaceBid(buyOrSell, "foobar", money.MustParse("BTC 0.001"))
	if err != nil {
		log.Fatalf("Place Bid failed: %v", err)
		return
	}
	log.Printf("bidId: %v", bidId)

	etag := ""
	for i := 0; i < 30; i++ {
		resp, err := getJsonFromServer("bid/"+bidId, etag)
		if err != nil {
			panic(err)
		}
		log.Printf("Response: %v", resp)
		if resp.StatusCode == http.StatusOK {
			etag = resp.Header.Get("ETag")
		} else if resp.StatusCode == http.StatusNotModified {
			log.Printf("  Cache hit")
		} else {
			log.Fatalf("UNEXPECTED STATUS CODE %v", resp.StatusCode)
		}
		time.Sleep(5 * time.Second)
	}
}

func normalize(s string) string {
	return url.QueryEscape(strings.Replace(s, " ", "", -1))
}
