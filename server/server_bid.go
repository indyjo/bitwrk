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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"bitbucket.org/ww/goautoneg"
	"github.com/indyjo/bitwrk-common/bitwrk"
	"github.com/indyjo/bitwrk/server/config"
	db "github.com/indyjo/bitwrk/server/gae"
	"github.com/indyjo/bitwrk/server/util"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
)

const bidCreateHtml = `
<!doctype html>
<html>
<head><title>Enter Bid</title></head>
<script src="/js/createbid.js" ></script>
<script src="/js/getnonce.js" ></script>
<body onload="getnonce()">
<form action="/bid" method="post">
<select id="article" name="article" placeholder="Article" onchange="update()">
<option>fnord</option>
<option>snafu</option>
<option>foobar</option>
</select> &larr; Choose article you would like to trade<br />
<input id="typebuy" type="radio" name="type" value="BUY" checked="checked" onchange="update()"/>Buy
<input id="typesell" type="radio" name="type" value="SELL"  onchange="update()"/>Sell
<input id="price" type="text" name="price" value="mBTC 1.00" onchange="update()"/> &larr; Max/min price<br/>
<input id="address" type="text" name="address" size="50" placeholder="Your account's Bitcoin address" onchange="update()"/>
<input id="nonce" type="hidden" name="nonce" onchange="update()"/> <br/>
<input type="text" name="signature" size="80" placeholder="Signature of query parameters using above address" />
<input type="submit" />
</form>
<br />
Sign this text to confirm bid:<br />
<input id="query" type="text" size="100" value="nonce=&article=fnord&type=buy&price=mBTC%201.0&address=" onclick="select()" readonly/>
</body>
</html>
`
const bidViewHtml = `
<!doctype html>
<html>
<head><title>View Bid</title></head>
<body>
<table>
<tr><th>Bid</th><td>{{.Id}}</td></tr>
<tr><th>Participant</th><td><a href="/account/{{.Bid.Participant}}">{{.Bid.Participant}}</a></td></tr>
<tr><th>Type</th><td>{{.Bid.Type}}</td></tr>
<tr><th>Article</th><td>{{.Bid.Article}}</td></tr>
<tr><th>Price</th><td>{{.Bid.Price}}</td></tr>
<tr><th>Fee</th><td>{{.Bid.Fee}}</td></tr>
<tr><th>State</th><td>{{.Bid.State}}</td></tr>
<tr><th>Created</th><td>{{.Bid.Created}}</td></tr>
<tr><th>Expires</th><td>{{.Bid.Expires}}</td></tr>
{{if .Bid.Transaction}}
<tr><th>Matched</th><td>{{.Bid.Matched}}</td></tr>
<tr><th>Transaction</th><td><a href="/tx/{{.Bid.Transaction}}">Matched</a></td></tr>
{{end}}
</table>
<script src="/js/getjson.js" ></script>
</body>
</html>
`

var bidCreateTemplate = template.Must(template.New("bidCreate").Parse(bidCreateHtml))
var bidViewTemplate = template.Must(template.New("bidView").Parse(bidViewHtml))

// Handler function for /bid/<bidid>
func handleRenderBid(w http.ResponseWriter, r *http.Request) {
	bidId := r.URL.Path[5:]

	if r.Method == "GET" {
		acceptable := []string{"text/html", "application/json"}
		contentType := goautoneg.Negotiate(r.Header.Get("Accept"), acceptable)
		if contentType == "" {
			http.Error(w,
				fmt.Sprintf("No accepted content type found. Supported: %v", acceptable),
				http.StatusNotAcceptable)
			return
		}

		c := appengine.NewContext(r)
		bid, err := db.GetBid(c, bidId)
		if err != nil {
			http.Error(w, "Bid not found: "+bidId, http.StatusNotFound)
			log.Warningf(c, "Non-existing bid queried: '%v'", bidId)
			return
		}

		// ETag handling using status and content-type
		etag := fmt.Sprintf("\"s%v-c%v\"", bid.State, len(contentType))
		if cachedEtag := r.Header.Get("If-None-Match"); cachedEtag == etag {
			w.Header().Del("Content-Type")
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("X-ETag", etag)
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", contentType)
		if contentType == "application/json" {
			err = renderBidJson(w, bidId, bid)
		} else {
			err = renderBidHtml(w, bidId, bid)
		}

		if err != nil {
			log.Errorf(c, "Error rendering %v as %v: %v", r.URL, contentType, err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// Handler function for /bid
func handleCreateBid(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		if err := bidCreateTemplate.Execute(w, nil); err != nil {
			http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
		}
	} else if r.Method == "POST" {
		c := appengine.NewContext(r)
		if err := r.ParseForm(); err != nil {
			log.Errorf(c, "Couldn't parse form data: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Infof(c, "Bid: %v", r.PostForm)
		if err := enqueueBid(c, w, r); err != nil {
			log.Errorf(c, "enqueueBid failed: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// The server's set of defaults for new bids:
//  - State is InQueue
//  - Fee is 3 percent
//  - Created is time.Now()
//  - Exprires is 120s from now
var newBidDefaults = bitwrk.NewBidDefaults{
	InitialState:        bitwrk.InQueue,
	FeeRatioNumerator:   3,
	FeeRatioDenominator: 100,
	Timeout:             120 * time.Second,
}

func enqueueBid(c context.Context, w http.ResponseWriter, r *http.Request) (err error) {
	bidType := r.FormValue("type")
	bidArticle := r.FormValue("article")
	bidPrice := r.FormValue("price")
	bidAddress := strings.TrimSpace(r.FormValue("address"))
	bidNonce := r.FormValue("nonce")
	bidSignature := r.FormValue("signature")

	// Important: checking (and invalidating) the nonce must be the first thing we do!
	err = checkNonce(c, bidNonce)
	if config.CfgRequireValidNonce && err != nil {
		return fmt.Errorf("Error in checkNonce: %v", err)
	}

	err = util.CheckArticle(c, bidArticle)
	if err != nil {
		return
	}

	err = util.CheckBitcoinAddress(bidAddress)
	if err != nil {
		return
	}

	bid, err := bitwrk.ParseBid(bidType, bidArticle, bidPrice, bidAddress, bidNonce, bidSignature,
		&newBidDefaults)
	if err != nil {
		return
	}

	if config.CfgRequireValidSignature {
		err = bid.Verify()
		if err != nil {
			return
		}
	}

	bidKey, err := db.EnqueueBid(c, bid)
	if err != nil {
		return fmt.Errorf("Error in db.EnqueueBid: %v", err)
	}

	// Send headers to client
	redirectToBid(bidKey, w, r)

	// Trigger batch processing
	if err := db.TriggerBatchProcessing(c, bid.MatchKey()); err != nil {
		log.Errorf(c, "Batch processing bids failed: %v", err)
	}

	return
}

func redirectToBid(bidKey *datastore.Key, w http.ResponseWriter, r *http.Request) {
	bidUrl, _ := url.Parse("/bid/" + bidKey.Encode())
	bidUrl = r.URL.ResolveReference(bidUrl)
	w.Header().Set("Location", bidUrl.RequestURI())
	w.Header().Set("X-Bid-Key", bidKey.Encode())
	w.WriteHeader(http.StatusSeeOther)
}

func renderBidHtml(w http.ResponseWriter, bidId string, bid *bitwrk.Bid) (err error) {
	type context struct {
		Id  string
		Bid *bitwrk.Bid
	}
	return bidViewTemplate.Execute(w, context{bidId, bid})
}

func renderBidJson(w http.ResponseWriter, bidId string, bid *bitwrk.Bid) (err error) {
	return json.NewEncoder(w).Encode(*bid)
}
