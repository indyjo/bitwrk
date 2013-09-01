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
	"bitwrk/client"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
)

var homeTemplate = template.Must(template.New("home").Parse(`
<!doctype html>
<html>
<head>
<meta charset="utf-8" />
<title>BitWrk Client</title>
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<link href="css/bootstrap.min.css" rel="stylesheet" media="screen" />
<link href="css/client.css" rel="stylesheet" />
<body>
<header>
<div id="account-info" />
</header>
<script src="/js/accountinfo.js" ></script>
<script src="/js/activity.js" ></script>

<div class="col-sm-4">
<h3>Activities</h3>
<div id="activities"></div>
</div>
<div class="col-sm-4"><h3>Workers</h3></div>
<div class="col-sm-4"><h3>Permissions</h3></div>
<script>
function updateAccountInfo() {
    updateAccountInfoFor("{{.ParticipantId}}");
}
setInterval(updateAccountInfo, 30000);
updateAccountInfo();
setInterval(updateActivities, 500);
updateActivities();
</script></body>
</html>
`))

type clientContext struct {
	ParticipantId string
}

func handleHome(w http.ResponseWriter, r *http.Request) {
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
