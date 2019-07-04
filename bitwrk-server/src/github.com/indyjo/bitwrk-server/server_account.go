//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2014-2019  Jonas Eschenburg <jonas@bitwrk.net>
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
	"bitbucket.org/ww/goautoneg"
	"context"
	"encoding/json"
	"fmt"
	"github.com/indyjo/bitwrk-common/bitwrk"
	"github.com/indyjo/bitwrk-server/config"
	db "github.com/indyjo/bitwrk-server/gae"
	"github.com/indyjo/bitwrk-server/util"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"html/template"
	"net/http"
	"net/url"
	"time"
)

const accountViewHtml = `
<!doctype html>
<html>
<head><title>View Account</title></head>
<body>
<table>
<tr><th>Participant</th><td>{{.Account.Participant}}</td></tr>
<tr><th>Available</th><td>{{.Account.Available}}</td></tr>
<tr><th>Blocked</th><td>{{.Account.Blocked}}</td></tr>
{{if .Account.LastMovementKey}}
<tr><th></th><td><a href="/ledger/{{.Account.LastMovementKey}}">Last ledger entry</a></td></tr>
{{end}}
</table>
{{if .DeveloperMode}}
<script src="/js/getnonce.js" ></script>
<script src="/js/createdepositinfo.js" ></script>
<script src="/js/requestdepositaddress.js" ></script>
<script src="/js/getjson.js" ></script>
<h1>Request Deposit Address</h1>
<form method="post">
<input id="rda_participant" type="text" name="participant" size="64" value="{{.Account.Participant}}" onchange="update()" /> &larr; Participant for whom to request a deposit address<br>
<input id="rda_signer" type="text" name="signer" size="64" value="{{.Account.Participant}}" onclick="select()" onchange="update()" /> &larr; The signer of this request.<br>
<input id="rda_nonce" type="hidden" name="nonce" onchange="update()"/> <br/>
<input type="rda_text" name="signature" size="64" placeholder="Signature of parameters" />
<button type="submit" name="action" value="requestdepositaddress">Request new deposit address</button>
</form>
<input id="rda_query" type="text" size="64" value="" onclick="select()" readonly/> &larr; Sign this text<br />
<h1>Store Deposit Info</h1>
<form method="post">
<input id="depositaddress" type="text" name="depositaddress" size="64" placeholder="A Bitcoin address" onfocus="select()" onchange="update()" /> &larr; Bitcoin address for deposits<br>
<input id="participant" type="text" name="participant" size="64" value="{{.Account.Participant}}" onchange="update()" /> &larr; Participant to store deposit info with<br>
<input id="signer" type="text" name="signer" size="64" value="{{.TrustedAccount}}" onclick="select()" onchange="update()" /> &larr; The signer of this message.<br>
<input id="reference" type="text" name="reference" size="64" onclick="select()" onchange="update()" /> &larr; Reference information.<br>
<input id="nonce" type="hidden" name="nonce" onchange="update()"/> <br/>
<input type="text" name="signature" size="64" placeholder="Signature of parameters" />
<button type="submit" name="action" value="storedepositinfo">Store deposit info</button>
</form>
<input id="query" type="text" size="64" value="" onclick="select()" readonly/> &larr; Sign this text<br />
<script>
	function update() {
		updateDepositInfoSignature(document.getElementById("query"))
		updateRequestDepositAddressSignature(document.getElementById("rda_query"))
	}
	update();
	getNonceFor(document.getElementById("nonce"));
	getNonceFor(document.getElementById("rda_nonce"));
</script>
{{else}}
<form><button type="submit" name="developermode" value="on">Enable developer features</button></form>
{{end}}
</body>
</html>
`

var accountViewTemplate = template.Must(template.New("accountView").Parse(accountViewHtml))

// Handler function for /account/<accountId>
func handleAccount(w http.ResponseWriter, r *http.Request) {
	accountId := r.URL.Path[9:]

	if r.Method == "GET" {
		acceptable := []string{"text/html", "application/json"}
		contentType := goautoneg.Negotiate(r.Header.Get("Accept"), acceptable)
		if contentType == "" {
			http.Error(w,
				fmt.Sprintf("No accepted content type found. Supported: %v", acceptable),
				http.StatusNotAcceptable)
			return
		}

		if err := util.CheckBitcoinAddress(accountId); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		c := appengine.NewContext(r)
		dao := db.NewGaeAccountingDao(c, false)
		var err error
		account, err := dao.GetAccount(accountId)

		if err != nil {
			http.Error(w, "Error retrieving account", http.StatusInternalServerError)
			log.Errorf(c, "Error getting account %v: %v", accountId, err)
			return
		}

		w.Header().Set("Content-Type", contentType)
		if contentType == "application/json" {
			err = renderAccountJson(w, &account)
		} else {
			devmode := r.FormValue("developermode") != ""
			err = renderAccountHtml(w, &account, devmode)
		}

		if err != nil {
			http.Error(w, "Error rendering account", http.StatusInternalServerError)
			log.Errorf(c, "Error rendering %v as %v: %v", r.URL, contentType, err)
		}
	} else if r.Method == "POST" {
		c := appengine.NewContext(r)
		log.Infof(c, "Got POST for account: %v", accountId)
		action := r.FormValue("action")
		if action == "storedepositinfo" {
			if err := storeDepositInfo(c, r, accountId); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else {
				http.Redirect(w, r, r.RequestURI, http.StatusSeeOther)
			}
		} else if action == "requestdepositaddress" {
			if err := requestDepositAddress(c, r, accountId); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else {
				http.Redirect(w, r, r.RequestURI, http.StatusSeeOther)
			}
		} else {
			http.Error(w, "invalid action: "+action, http.StatusInternalServerError)
		}
	} else {
		http.Error(w, "Method not allowed: "+r.Method, http.StatusMethodNotAllowed)
	}
}

func renderAccountHtml(w http.ResponseWriter, account *bitwrk.ParticipantAccount, devmode bool) (err error) {
	return accountViewTemplate.Execute(w, struct {
		Account        *bitwrk.ParticipantAccount
		DeveloperMode  bool
		TrustedAccount string
	}{account, devmode, config.CfgTrustedAccount})
}

func renderAccountJson(w http.ResponseWriter, account *bitwrk.ParticipantAccount) (err error) {
	return json.NewEncoder(w).Encode(*account)
}

func requestDepositAddress(c context.Context, r *http.Request, participant string) (err error) {
	// Important: checking (and invalidating) the nonce must be the first thing we do!
	err = checkNonce(c, r.FormValue("nonce"))
	if config.CfgRequireValidNonce && err != nil {
		return fmt.Errorf("Error in checkNonce: %v", err)
	}

	m := bitwrk.DepositAddressRequest{}
	m.FromValues(r.Form)

	if m.Participant != participant {
		return fmt.Errorf("Participant must be %#v", participant)
	}

	if m.Signer != config.CfgTrustedAccount && m.Signer != participant {
		return fmt.Errorf("Signer must be participant or %#v", config.CfgTrustedAccount)
	}

	// Verify that the request was indeed signed correctly
	if config.CfgRequireValidSignature {
		if err := m.VerifyWith(m.Signer); err != nil {
			return fmt.Errorf("After verifying %#v against %v: %v", m, m.Signer, err)
		}
	}

	f := func(c context.Context) error {
		dao := db.NewGaeAccountingDao(c, true)
		if account, err := dao.GetAccount(participant); err != nil {
			return err
		} else if account.DepositAddressRequest != "" {
			return fmt.Errorf("There is a pending address request already.")
		} else if account.LastDepositInfo.Add(24 * time.Hour).After(time.Now()) {
			return fmt.Errorf("Next deposit address can be requested %v", account.LastDepositInfo.Add(24*time.Hour))
		} else {
			v := url.Values{}
			m.ToValues(v)
			account.DepositAddressRequest = v.Encode()
			log.Infof(c, "New deposit address request: %v", account.DepositAddressRequest)
			if err := dao.SaveAccount(&account); err != nil {
				return err
			}
		}
		return dao.Flush()
	}

	if err := datastore.RunInTransaction(c, f, &datastore.TransactionOptions{XG: true}); err != nil {
		// Transaction failed
		log.Errorf(c, "Transaction failed: %v", err)
		return err
	}

	return
}

func storeDepositInfo(c context.Context, r *http.Request, participant string) (err error) {
	// Important: checking (and invalidating) the nonce must be the first thing we do!
	err = checkNonce(c, r.FormValue("nonce"))
	if config.CfgRequireValidNonce && err != nil {
		return fmt.Errorf("Error in checkNonce: %v", err)
	}

	m := bitwrk.DepositAddressMessage{}
	m.FromValues(r.Form)

	if m.Participant != participant {
		return fmt.Errorf("Participant must be %#v", participant)
	}

	if m.Signer != config.CfgTrustedAccount {
		return fmt.Errorf("Signer must be %#v", config.CfgTrustedAccount)
	}

	// Bitcoin addresses must have the right network id
	if err := util.CheckBitcoinAddress(m.DepositAddress); err != nil {
		return err
	}

	// Verify that the message was indeed signed by the trusted account
	if config.CfgRequireValidSignature {
		if err := m.VerifyWith(config.CfgTrustedAccount); err != nil {
			return err
		}
	}

	f := func(c context.Context) error {
		dao := db.NewGaeAccountingDao(c, true)
		if account, err := dao.GetAccount(participant); err != nil {
			return err
		} else {
			if account.DepositInfo != "" {
				log.Infof(c, "Replacing old deposit info: %v", account.DepositInfo)
			}
			v := url.Values{}
			m.ToValues(v)
			account.DepositInfo = v.Encode()
			account.LastDepositInfo = time.Now()
			account.DepositAddressRequest = ""
			log.Infof(c, "New deposit info: %v", account.DepositInfo)
			if err := dao.SaveAccount(&account); err != nil {
				return err
			}
		}
		return dao.Flush()
	}

	if err := datastore.RunInTransaction(c, f, &datastore.TransactionOptions{XG: true}); err != nil {
		// Transaction failed
		log.Errorf(c, "Transaction failed: %v", err)
		return err
	}

	return
}
