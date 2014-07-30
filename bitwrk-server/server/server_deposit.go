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
	db "bitwrk/gae"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
)

const depositCreateHtml = `
<!doctype html>
<html>
<head><title>Enter Deposit</title></head>
<script src="/js/getnonce.js" ></script>
<script src="/js/createdeposit.js" ></script>
<body onload="getnonce()">
<form action="/deposit" method="post">
<select id="type" name="type">
<option value="0" selected>Injection</option>
<option value="1">Bitcoin</option>
<option value="2">Coupon</option>
</select> &larr; Choose the type of deposit you would like to register<br />
<input id="amount" type="text" name="amount" value="mBTC 23.42" onchange="update()" /> &larr; Amount to deposit<br/>
<input id="account" type="text" name="account" size="64" value="1BiTWrKBPKT2yKdfEw77EAsCHgpjkqgPkv" onclick="select()" onchange="update()" /> &larr; The receiver's account.<br>
<input id="uid" type="text" name="uid" size="64" onclick="select()" onchange="update()" /> &larr; A unique key assigned to this deposit by you. There can only be one deposit with this key. Once created, a deposit cannot be changed.<br>
<input id="ref" type="text" name="ref" size="64" onclick="select()" onchange="update()" /> &larr; Reference information for you to track this deposit.<br>
<input id="nonce" type="hidden" name="nonce" onchange="update()"/> <br/>
<input type="text" name="signature" size="64" placeholder="Signature of query parameters" />
<input type="submit" />
</form>
<br />
Sign this text using address ` + CfgTrustedAccount + ` to confirm bid:<br />
<input id="query" type="text" size="180" onclick="select()" readonly/>
</body>
<script>
  var a = document.getElementById("uid");
  a.value = 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    var r = Math.random()*16|0, v = c == 'x' ? r : (r&0x3|0x8);
    return v.toString(16);
  });
</script>
</html>
`
const depositViewHtml = `
<!doctype html>
<html>
<head><title>View Deposit</title></head>
<body>
<table>
<tr><th>Deposit</th><td>{{.Uid}}</td></tr>
<tr><th>Type</th><td>{{.Deposit.Type}}</td></tr>
<tr><th>Account</th><td><a href="/account/{{.Deposit.Account}}">{{.Deposit.Account}}</a></td></tr>
<tr><th>Amount</th><td>{{.Deposit.Amount}}</td></tr>
<tr><th>Reference</th><td>{{.Deposit.Reference}}</td></tr>
<tr><th>Created</th><td>{{.Deposit.Created}}</td></tr>
</table>
<script src="/js/getjson.js" ></script>
</body>
</html>
`

var depositCreateTemplate = template.Must(template.New("depositCreate").Parse(depositCreateHtml))
var depositViewTemplate = template.Must(template.New("depositView").Parse(depositViewHtml))

// Handler function for /deposit
func handleCreateDeposit(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		if err := depositCreateTemplate.Execute(w, nil); err != nil {
			http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
		}
	} else if r.Method == "POST" {
		c := appengine.NewContext(r)
		depositType := r.FormValue("type")
		depositAccount := r.FormValue("account")
		depositAmount := r.FormValue("amount")
		depositNonce := r.FormValue("nonce")
		depositUid := r.FormValue("uid")
		depositRef := r.FormValue("ref")
		depositSig := r.FormValue("signature")

		if err := createDeposit(c, depositType, depositAccount, depositAmount, depositNonce, depositUid, depositRef, depositSig); err != nil {
			http.Error(w, "Error creating deposit: "+err.Error(), http.StatusInternalServerError)
		} else {
			http.Redirect(w, r, "/deposit/"+depositUid, http.StatusFound)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func createDeposit(c appengine.Context, depositType, depositAccount, depositAmount, depositNonce, depositUid, depositRef, depositSig string) (err error) {
	// Important: checking (and invalidating) the nonce must be the first thing we do!
	err = checkNonce(c, depositNonce)
	if CfgRequireValidNonce && err != nil {
		return fmt.Errorf("Error in checkNonce: %v", err)
	}

	// Bitcoin addresses must have the right network id
	err = checkBitcoinAddress(depositAccount)
	if err != nil {
		return
	}

	deposit, err := bitwrk.ParseDeposit(depositType, depositAccount, depositAmount, depositNonce, depositUid, depositRef, depositSig)
	if err != nil {
		return fmt.Errorf("Error in ParseDeposit: %v", err)
	}

	if CfgRequireValidSignature {
		err = deposit.Verify(CfgTrustedAccount)
		if err != nil {
			return
		}
	}

	f := func(c appengine.Context) error {
		dao := db.NewGaeAccountingDao(c)
		if err := deposit.Place(depositUid, dao); err != nil {
			return err
		}
		dao.Flush()
		return nil
	}

	if err := datastore.RunInTransaction(c, f, &datastore.TransactionOptions{XG: true}); err != nil {
		// Transaction failed
		return err
	}

	_ = deposit
	c.Infof("Deposit: %#v", deposit)

	return
}

// Handler function for /deposit/<uid>
func handleRenderDeposit(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Path[9:]

	if r.Method != "GET" {
		http.Error(w, "Only GET allowed", http.StatusMethodNotAllowed)
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
	dao := db.NewGaeAccountingDao(c)

	deposit, err := dao.GetDeposit(uid)
	if err != nil {
		http.Error(w, "Deposit not found: "+uid, http.StatusNotFound)
		c.Warningf("Non-existing deposit queried: '%v'", uid)
		return
	}

	w.Header().Set("Content-Type", contentType)
	if contentType == "application/json" {
		err = renderDepositJson(w, deposit)
	} else {
		err = renderDepositHtml(w, uid, deposit)
	}

	if err != nil {
		c.Errorf("Error rendering %v as %v: %v", r.URL, contentType, err)
	}
}

func renderDepositHtml(w io.Writer, uid string, deposit bitwrk.Deposit) error {
	type context struct {
		Uid     string
		Deposit bitwrk.Deposit
	}
	return depositViewTemplate.Execute(w, context{uid, deposit})
}

func renderDepositJson(w io.Writer, deposit bitwrk.Deposit) error {
	return json.NewEncoder(w).Encode(deposit)
}
