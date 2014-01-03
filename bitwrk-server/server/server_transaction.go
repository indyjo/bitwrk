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
	"appengine"
	"appengine/datastore"
	"bitbucket.org/ww/goautoneg"
	"bitwrk"
	"bitwrk/bitcoin"
	db "bitwrk/gae"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const txViewHtml = `
<!doctype html>
<html>
<head><title>View Transaction</title></head>
<script src="/js/createmessage.js" ></script>
<body onload="update()">
<h1>Transaction</h1>
<table>
<tr><th>TxId</th><td colspan="2">{{.Id}}</td></tr>
<tr><th>Bids</th><td><a href="/bid/{{.Tx.BuyerBid}}">Buyer</a></td><td><a href="/bid/{{.Tx.SellerBid}}">Seller</a></td></tr>
<tr><th>Addresses</th><td><a href="/account/{{.Tx.Buyer}}">{{.Tx.Buyer}}</a></td><td><a href="/account/{{.Tx.Seller}}">{{.Tx.Seller}}</a></td></tr>
<tr><th>Price</th><td colspan="2">{{.Tx.Price}}</td></tr>
<tr><th>Phase</th><td colspan="2">{{.Tx.Phase}}</td></tr>
{{if .Tx.WorkerURL}}
<tr><th>Worker's URL</th><td colspan="2">{{.Tx.WorkerURL}}</td></tr>
{{end}}
</table>

<script src="/js/getjson.js" ></script>

{{if .Messages}}
<h1>Received Messages</h1>
<table>
<tr>
<th>Received</th>
<th>From</th>
<th>Pre-phase</th>
<th>Post-phase</th>
</tr>
{{range .Messages}}
{{if .Accepted}}
<tr><td>{{.Received}}</td><td>{{.From}}</td><td>{{.PrePhase}}</td><td>{{.PostPhase}}</td></tr>
{{end}}
{{end}}
</table>
{{end}}

<h1>Send Message</h1>
<input id="txid" type="hidden" name="txid" value="{{.Id}}" />
<input id="query" type="text" readonly onclick="select()" size="120" /> &larr; Paste this text into Bitcoin and sign it (with the right address)

<table>

<tr>
<form action="/tx/{{.Id}}" method="POST">
<th>Buyer</th>
<input type="hidden" name="address" value="{{.Tx.Buyer}}" />
<td><input id="workhash" type="text" name="workhash" placeholder="H(work)" onchange="update()"/></td>
<td><input id="worksecrethash" type="text" name="worksecrethash" placeholder="H(H(work)|bsecret)" onchange="update()"/></td>
<td/>
<td><input type="signature" name="signature" placeholder="Paste signature here"/></td>
<td><input type="submit" /></td>
</form>
</tr>

<tr>
<form action="/tx/{{.Id}}" method="POST">
<th>Seller</th>
<input type="hidden" name="address" value="{{.Tx.Seller}}" />
<td><input id="workerurl" type="text" name="workerurl" placeholder="Worker's URL" onchange="update()"/></td>
<td/>
<td/>
<td><input type="signature" name="signature" placeholder="Paste signature here"/></td>
<td><input type="submit" /></td>
</form>
</tr>

<tr>
<form action="/tx/{{.Id}}" method="POST">
<th>Seller</th>
<input type="hidden" name="address" value="{{.Tx.Seller}}" />
<td><input id="buyersecret" type="text" name="buyersecret" placeholder="Buyer's secret" onchange="update()"/></td>
<td/>
<td/>
<td><input type="signature" name="signature" placeholder="Paste signature here"/></td>
<td><input type="submit" /></td>
</form>
</tr>

<tr>
<form action="/tx/{{.Id}}" method="POST">
<th>Seller</th>
<input type="hidden" name="address" value="{{.Tx.Seller}}" />
<td><input id="rejectwork" type="checkbox" name="rejectwork" onchange="update()"/>Reject work</td>
<td/>
<td/>
<td><input type="signature" name="signature" placeholder="Paste signature here"/></td>
<td><input type="submit" /></td>
</form>
</tr>

<tr>
<form action="/tx/{{.Id}}" method="POST">
<th>Seller</th>
<input type="hidden" name="address" value="{{.Tx.Seller}}" />
<td><input id="encresulthash" type="text" name="encresulthash" placeholder="H(Encrypted result)" onchange="update()"/></td>
<td><input id="encresulthashsig" type="text" name="encresulthashsig" placeholder="Buyer_Sig(H(Enc.res.))" onchange="update()"/></td>
<td><input id="encresultkey" type="text" name="encresultkey" placeholder="Key to decrypt" onchange="update()"/></td>
<td><input type="signature" name="signature" placeholder="Paste signature here"/></td>
<td><input type="submit" /></td>
</form>
</tr>

<tr>
<form action="/tx/{{.Id}}" method="POST">
<th>Buyer</th>
<input type="hidden" name="address" value="{{.Tx.Buyer}}" />
<td><input id="rejectresult" type="checkbox" name="rejectresult" onchange="update()"/>Reject result</td>
<td/>
<td/>
<td><input type="signature" name="signature" placeholder="Paste signature here"/></td>
<td><input type="submit" /></td>
</form>
</tr>

<tr>
<form action="/tx/{{.Id}}" method="POST">
<th>Buyer</th>
<input type="hidden" name="address" value="{{.Tx.Buyer}}" />
<td><input id="acceptresult" type="checkbox" name="acceptresult" onchange="update()"/>Accept result</td>
<td/>
<td/>
<td><input type="signature" name="signature" placeholder="Paste signature here"/></td>
<td><input type="submit" /></td>
</form>
</tr>
</table>

</body>
</html>
`

var txViewTemplate = template.Must(template.New("txView").Parse(txViewHtml))

// Handler for /tx/* URLs
func handleTx(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	acceptable := []string{"text/html", "application/json"}
	contentType := goautoneg.Negotiate(r.Header.Get("Accept"), acceptable)
	if contentType == "" {
		http.Error(w,
			fmt.Sprintf("No accepted content type found. Supported: %v", acceptable),
			http.StatusNotAcceptable)
		return
	}

	c := appengine.NewContext(r)
	txId := r.URL.Path[4:]

	txKey, err := datastore.DecodeKey(txId)
	if err != nil {
		c.Warningf("Illegal tx id queried: '%v'", txId)
		http.Error(w, "Transaction not found: "+txId, http.StatusNotFound)
		return
	}

	var tx *bitwrk.Transaction
	var messages []bitwrk.Tmessage
	if r.Method == "POST" {
		err = updateTransaction(c, r, txId, txKey)
		if err != nil {
			message := fmt.Sprintf("Couldn't update transaction %#v: %v", txId, err)
			c.Warningf("%v", message)
			http.Error(w, message, http.StatusInternalServerError)
		} else {
			redirectToTransaction(txId, w, r)
		}
		return
	}

	// GET only
	tx, err = db.GetTransaction(c, txKey)
	if err != nil {
		c.Warningf("Datastore lookup failed for tx id: '%v'", txId)
		c.Warningf("Reason: %v", err)
		http.Error(w, "Transaction not found: "+txId, http.StatusNotFound)
		return
	}

	// ETag handling using transaction's revision number and content type
	etag := fmt.Sprintf("r%v-c%v", tx.Revision, contentType)
	if cachedEtag := r.Header.Get("If-None-Match"); cachedEtag == etag {
		w.Header().Del("Content-Type")
		w.WriteHeader(http.StatusNotModified)
		return
	}

	messages, _ = db.GetTransactionMessages(c, txKey)

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("ETag", etag)
	if contentType == "application/json" {
		err = renderTxJson(w, txId, tx)
	} else {
		err = renderTxHtml(w, txId, tx, messages)
	}

	if err != nil {
		c.Errorf("Error rendering %v as %v: %v", r.URL, contentType, err)
	}
}

func redirectToTransaction(txId string, w http.ResponseWriter, r *http.Request) {
	txUrl, _ := url.Parse("/tx/" + txId)
	txUrl = r.URL.ResolveReference(txUrl)
	w.Header().Set("Location", txUrl.RequestURI())
	w.WriteHeader(http.StatusSeeOther)
}

func renderTxHtml(w http.ResponseWriter, txId string, tx *bitwrk.Transaction, messages []bitwrk.Tmessage) (err error) {
	type context struct {
		Id       string
		Tx       *bitwrk.Transaction
		Messages []bitwrk.Tmessage
	}
	return txViewTemplate.Execute(w, context{txId, tx, messages})
}

func renderTxJson(w http.ResponseWriter, txId string, tx *bitwrk.Transaction) (err error) {
	return json.NewEncoder(w).Encode(*tx)
}

// Creates a canonical representation of the passed arguments (the document),
// which is expected to have been signed by the message sender.
func makeDocument(m map[string]string) string {
	arguments := make([]string, 0, len(m))
	for k, v := range m {
		var queryPart string
		if v == "" {
			queryPart = url.QueryEscape(k)
		} else {
			queryPart = fmt.Sprintf("%v=%v", url.QueryEscape(k), url.QueryEscape(v))
		}
		arguments = append(arguments, queryPart)
	}

	sort.Strings(arguments)

	return strings.Join(arguments, "&")
}

func updateTransaction(c appengine.Context, r *http.Request, txId string, txKey *datastore.Key) error {
	now := time.Now()

	r.ParseForm()
	values := make(map[string]string)

	// Check that we don't have any multi-occurring parameters,
	// copy into a simple key value map for easier handling
	for k, v := range r.Form {
		if len(v) == 1 {
			values[k] = v[0]
		} else if len(v) > 1 {
			return fmt.Errorf("Multiple occurrences of argument %#v", k)
		}
	}

	// Verify "txid" parameter, add if not found
	if txId2, ok := values["txid"]; ok {
		if txId2 != txId {
			return fmt.Errorf("Transaction ID parameter doesn't match %#v", txId)
		}
	} else {
		values["txid"] = txId
	}

	// Filter out "signature" parameter
	var signature string
	if _signature, ok := values["signature"]; ok {
		delete(values, "signature")
		signature = _signature
	} else {
		return fmt.Errorf("Missing signature parameter in message")
	}

	// Filter out "address" parameter
	var address string
	if _address, ok := values["address"]; ok {
		delete(values, "address")
		address = _address
	} else {
		return fmt.Errorf("Missing address parameter in message")
	}

	document := makeDocument(values)
	if CfgRequireValidSignature {
		if err := bitcoin.VerifySignatureBase64(document, address, signature); err != nil {
			return err
		}
	}

	// no need for txid in values anymore
	delete(values, "txid")

	if tx, err := db.UpdateTransaction(c, txKey, now, address, values, document, signature); err != nil {
		return err
	} else {
		addRetireTransactionTask(c, txId, tx)
	}

	return nil
}
