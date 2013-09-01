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
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// Disallow redirects (or explicitly handle them)
var client = http.Client{
	CheckRedirect: func(r *http.Request, _ []*http.Request) error {
		return fmt.Errorf("Redirect encountered in request %v", r)
	},
}

func GetClient() *http.Client {
	return &client
}

func NewRequest(method, url string, body io.Reader) (*http.Request, error) {
	return newRequest(method, url, body)
}

func newRequest(method, url string, body io.Reader) (*http.Request, error) {
	if r, err := http.NewRequest(method, url, body); err != nil {
		return nil, err
	} else {
		r.Header.Set("User-Agent", BitwrkUserAgent)
		return r, nil
	}
	return nil, nil // never reached
}

func newServerRequest(method, relpath string, body io.Reader) (r *http.Request, err error) {
	return newRequest(method, BitwrkUrl+relpath, body)
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
		return nil, fmt.Errorf("newServerRequest failed: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	return client.Do(req)
}

func FetchBid(bidId, etag string) (*bitwrk.Bid, string, error) {
	var response *http.Response
	if r, err := getJsonFromServer("bid/"+bidId, etag); err != nil {
		return nil, "", fmt.Errorf("getJsonFromServer (etag=%v) failed: %v", etag, err)
	} else {
		response = r
	}

	if response.StatusCode == http.StatusOK {
		decoder := json.NewDecoder(response.Body)
		var bid bitwrk.Bid
		if err := decoder.Decode(&bid); err != nil {
			return nil, "", err
		}
		return &bid, response.Header.Get("ETag"), nil
	} else if response.StatusCode == http.StatusNotModified {
		return nil, etag, nil
	}

	return nil, "", fmt.Errorf("Error fetching bid: %v", response.Status)
}

func FetchTx(txId, etag string) (*bitwrk.Transaction, string, error) {
	var response *http.Response
	if r, err := getJsonFromServer("tx/"+txId, etag); err != nil {
		return nil, "", err
	} else {
		response = r
	}

	if response.StatusCode == http.StatusOK {
		decoder := json.NewDecoder(response.Body)
		var tx bitwrk.Transaction
		if err := decoder.Decode(&tx); err != nil {
			return nil, "", fmt.Errorf("Error decoding transaction JSON: %v", err)
		}
		return &tx, response.Header.Get("ETag"), nil
	} else if response.StatusCode == http.StatusNotModified {
		return nil, etag, nil
	}

	return nil, "", fmt.Errorf("Error fetching transaction: %v", response.Status)
}

func postFormToServer(relpath, query string) (*http.Response, error) {
	req, err := newServerRequest("POST", relpath, strings.NewReader(query))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
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
	if err == nil || resp.StatusCode == http.StatusSeeOther && resp.Header.Get("X-Bid-Key") != "" {
		bidId = resp.Header.Get("X-Bid-Key")
		err = nil
	} else if err == nil {
		var more []byte
		if resp != nil {
			more, _ = ioutil.ReadAll(resp.Body)
		}
		err = fmt.Errorf("Got status: %#v\nResponse: %v", resp.Status, string(more))
	}

	return
}

func SendTxMessage(txId string, identity *bitcoin.KeyPair, arguments map[string]string) error {
	arguments["txid"] = txId

	keys := make([]string, len(arguments))
	i := 0
	for k, _ := range arguments {
		keys[i] = k
		i++
	}
	sort.Strings(keys)

	// Prepare document for signature. Document consists of alphabetically sorted
	// query arguments, plus "txid"
	document := make([]byte, 0, 100)
	for _, k := range keys {
		if len(document) > 0 {
			document = append(document, '&')
		}
		document = append(document, (k + "=" + normalize(arguments[k]))...)
	}

	signature := ""
	if s, err := identity.SignMessage(string(document), rand.Reader); err != nil {
		return err
	} else {
		signature = s
	}

	query := string(document) +
		"&signature=" + url.QueryEscape(signature) +
		"&address=" + url.QueryEscape(identity.GetAddress())
	if r, err := postFormToServer("tx/"+txId, query); err != nil {
		return err
	} else if r.StatusCode != http.StatusSeeOther {
		return fmt.Errorf("Unexpected status: %v", r.Status)
	}

	return nil
}

func SendTxMessageEstablishBuyer(txId string, identity *bitcoin.KeyPair, workHash, workSecretHash bitwrk.Thash) error {
	arguments := make(map[string]string)
	arguments["workhash"] = hex.EncodeToString(workHash[:])
	arguments["worksecrethash"] = hex.EncodeToString(workSecretHash[:])
	return SendTxMessage(txId, identity, arguments)
}

func SendTxMessageEstablishSeller(txId string, identity *bitcoin.KeyPair, workerURL string) error {
	arguments := make(map[string]string)
	arguments["workerurl"] = workerURL
	return SendTxMessage(txId, identity, arguments)
}

func SendTxMessagePublishBuyerSecret(txId string, identity *bitcoin.KeyPair, buyerSecret *bitwrk.Thash) error {
	arguments := make(map[string]string)
	arguments["buyersecret"] = buyerSecret.String()
	return SendTxMessage(txId, identity, arguments)
}

func SendTxMessageTransmitFinished(txId string, identity *bitcoin.KeyPair, encResultHash, encResultHashSig, encResultKey string) error {
	arguments := make(map[string]string)
	arguments["encresulthash"] = encResultHash
	arguments["encresulthashsig"] = encResultHashSig
	arguments["encresultkey"] = encResultKey
	return SendTxMessage(txId, identity, arguments)
}

func SendTxMessageAcceptResult(txId string, identity *bitcoin.KeyPair) error {
	arguments := make(map[string]string)
	arguments["acceptresult"] = "on"
	return SendTxMessage(txId, identity, arguments)
}

func normalize(s string) string {
	return url.QueryEscape(strings.Replace(s, " ", "", -1))
}
