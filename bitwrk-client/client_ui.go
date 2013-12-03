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
	"bitwrk/client"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"
)

var homeTemplate = template.Must(template.New("home").Parse(`
<!doctype html>
<html>
<head>
<meta charset="utf-8" />
<title>BitWrk Client</title>
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<link href="css/bootstrap-theme.min.css" rel="stylesheet" media="screen" />
<link href="css/bootstrap.min.css" rel="stylesheet" media="screen" />
<link href="css/client.css" rel="stylesheet" />
<body>
<header>
<div id="account-info" />
</header>
<script src="/js/jquery-1.10.2.js" ></script>
<script src="/js/bootstrap.min.js" ></script>
<script src="/js/js-iso8601.js" ></script>
<script src="/js/accountinfo.js" ></script>
<script src="/js/mandate-dialog.js" ></script>
<script src="/js/activity.js" ></script>
<script src="/js/workers.js" ></script>
<script src="/js/mandates.js" ></script>
<div class="col-sm-4"><h3>Activities</h3><div id="activities"></div></div>
<div class="col-sm-4"><h3>Workers</h3><div id="workers"></div></div>
<div class="col-sm-4"><h3>Mandates</h3><div id="mandates"></div></div>
<div id="mandateModal" class="modal fade">
<div class="modal-dialog">
<div class="modal-content">
<form method="POST">
<input type="hidden" name="action" value="permit" />
<input type="hidden" name="type" />
<input type="hidden" name="articleid" />
<div class="modal-header">
<button type="button" class="close" data-dismiss="modal" aria-hidden="true">&times;</button>
<h4 class="modal-title">Permit trade...</h4>
The BitWrk client is asking for your permission to perform a trade.</div>
<div id="mandateModalBody" class="modal-body">
<table>
<tr>
<th>Type</th>
<td colspan="2"><span class="text-info" id="perm-type-span"
>_BUY_</span> of <span class="text-info" id="perm-articleid-span"
>_ARTICLEID_</span></td>
</tr>
<tr>
<th>Price</th>
<td><input type="text" name="price" value="BTC 0.0001" /></td>
</tr>
<tr>
<th><label><input type="checkbox" name="usetradesleft"/> Valid for up to</label></th>
<td><input type="number" name="tradesleft" value="100" min="1"/> trades.</td>
</tr>
<tr>
<th><label><input type="checkbox" name="usevaliduntil"/> Valid for</label></th>
<td><input type="number" name="validminutes" value="20" min="1"/> minutes.</td>
</tr>
<tr>
<td colspan="3" class="text-muted">If none of the above options is checked, exactly one trade will be permitted.</td>
</tr>
</table>
</div>
<div class="modal-footer">
<button type="button" class="btn btn-default" data-dismiss="modal" data-target="#mandateModal">Cancel</button>
<input type="submit" class="btn btn-primary"/>
</div><!-- modal-footer -->
</form>
</div><!-- modal-body -->
</div><!-- modal-dialog -->
</div><!-- modal-col-sm-4 -->
<script>
function updateAccountInfo() {
    updateAccountInfoFor("{{.ParticipantId}}");
}
setInterval(updateAccountInfo, 30000);
updateAccountInfo();
setInterval(updateActivities, 500);
updateActivities();
setInterval(updateWorkers, 500);
updateWorkers();
setInterval(updateMandates, 500);
updateMandates();
</script></body>
</html>
`))

type clientContext struct {
	ParticipantId string
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	if action := r.FormValue("action"); action == "permit" {
		if err := handleGrantMandate(r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			// Success! Send back to home page
			http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
		}
		return
	}

	homeTemplate.Execute(w, &clientContext{
		BitcoinIdentity.GetAddress(),
	})
}

type activityInfo struct {
	Key  client.ActivityKey
	Info *client.ActivityState
}

func handleActivities(w http.ResponseWriter, r *http.Request) {
	activities := client.GetActivityManager().GetActivitiesSorted()
	infos := make([]activityInfo, len(activities))
	w.Header().Set("Content-Type", "application/json")
	for k, v := range activities {
		infos[k] = activityInfo{v.GetKey(), v.GetState()}
	}
	if err := json.NewEncoder(w).Encode(infos); err != nil {
		panic(err)
	}
}

func handleForbid(w http.ResponseWriter, r *http.Request) {
	activity := r.FormValue("activity")
	if result, err := forbidActivity(activity); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		fmt.Fprintf(w, "%v", result)
	}
}

func forbidActivity(activity string) (bool, error) {
	var k client.ActivityKey
	if err := k.Parse(activity); err != nil {
		return false, err
	} else if a := client.GetActivityManager().GetActivityByKey(k); a == nil {
		return false, fmt.Errorf("Activity %#v not found.", activity)
	} else {
		return a.Forbid(), nil
	}
}

func handleGrantMandate(r *http.Request) error {
	var mandate client.Mandate
	mandate.Identity = BitcoinIdentity
	if r.FormValue("type") == "BUY" {
		mandate.BidType = bitwrk.Buy
	} else if r.FormValue("type") == "SELL" {
		mandate.BidType = bitwrk.Sell
	} else {
		return fmt.Errorf("Illegal trade type: %v", r.FormValue("type"))
	}
	mandate.Article = bitwrk.ArticleId(r.FormValue("articleid"))
	if err := mandate.Price.Parse(r.FormValue("price")); err != nil {
		return err
	}
	mandate.UseTradesLeft = "on" == r.FormValue("usetradesleft")
	mandate.UseUntil = "on" == r.FormValue("usevaliduntil")
	if n, err := strconv.ParseInt(r.FormValue("tradesleft"), 10, 32); err != nil {
		return fmt.Errorf("Illegal value for trades left: %v", err)
	} else if n <= 0 {
		return fmt.Errorf("Number of trades left must be positive, but is: %v", n)
	} else {
		mandate.TradesLeft = int(n)
	}
	if n, err := strconv.ParseInt(r.FormValue("validminutes"), 10, 32); err != nil {
		return fmt.Errorf("Illegal value for minutes left: %v", err)
	} else if n <= 0 {
		return fmt.Errorf("Number of minutes left must be positive, but is: %v", n)
	} else {
		mandate.Until = time.Now().Add(time.Duration(n) * time.Minute)
	}
	if !mandate.UseTradesLeft && !mandate.UseUntil {
		mandate.UseTradesLeft = true
		mandate.TradesLeft = 1
	}
	key := client.GetActivityManager().NewKey()
	client.GetActivityManager().RegisterMandate(key, &mandate)
	return nil
}

func handleRevokeMandate(r *http.Request) error {
	if key, err := strconv.ParseInt(r.FormValue("key"), 10, 64); err != nil {
		return err
	} else {
		client.GetActivityManager().UnregisterMandate(client.ActivityKey(key))
	}
	return nil
}

func handleWorkers(workerManager *client.WorkerManager, w http.ResponseWriter, r *http.Request) {
	workerStates := workerManager.ListWorkers()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(workerStates); err != nil {
		panic(err)
	}
}

type keyedMandateInfo struct {
	Key  client.ActivityKey
	Info *client.MandateInfo
}

func handleMandates(activityManager *client.ActivityManager, w http.ResponseWriter, r *http.Request) {
	mandates := activityManager.GetMandates()
	keyedInfos := make([]keyedMandateInfo, 0, len(mandates))
	for k, v := range mandates {
		keyedInfos = append(keyedInfos, keyedMandateInfo{
			k,
			v.GetInfo()})
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(keyedInfos); err != nil {
		panic(err)
	}
}
