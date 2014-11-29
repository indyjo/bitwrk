//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013-2014  Jonas Eschenburg <jonas@bitwrk.net>
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
	"encoding/json"
	"errors"
	"fmt"
	client "github.com/indyjo/bitwrk-client"
	"github.com/indyjo/bitwrk-common/bitwrk"
	"html/template"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

var _templatesInitialized = sync.Once{}
var _homeTemplate *template.Template
var _translations = map[string]string{
	"page.home":    "Home",
	"page.account": "Account",
}

func initTemplates() {
	_templatesInitialized.Do(func() {
		p := path.Join(ResourceDir, "templates", "index.html")

		// see http://stackoverflow.com/questions/18276173
		_homeTemplate = template.Must(template.New("").Funcs(template.FuncMap{
			"dict": func(values ...interface{}) (map[string]interface{}, error) {
				if len(values)%2 != 0 {
					return nil, errors.New("invalid dict call")
				}
				dict := make(map[string]interface{}, len(values)/2)
				for i := 0; i < len(values); i += 2 {
					key, ok := values[i].(string)
					if !ok {
						return nil, errors.New("dict keys must be strings")
					}
					dict[key] = values[i+1]
				}
				return dict, nil
			},
			"text": func(values ...interface{}) (string, error) {
				if len(values) != 1 {
					return "", errors.New("text() needs exatly one argument")
				}
				if s, ok := values[0].(string); !ok {
					return "", errors.New("text() takes a string as first argument")
				} else {
					return _translations[s], nil
				}
			},
		}).ParseFiles(p)).Lookup("index.html")
	})
}

func getHomeTemplate() *template.Template {
	initTemplates()
	return _homeTemplate
}

type clientContext struct {
	ParticipantId string
	ServerURL     string
	Page          string
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
	} else if action != "" {
		http.Error(w, fmt.Sprintf("Unrecognized form request: %v", r.Form), http.StatusBadRequest)
		return
	}

	var page string
	if strings.HasPrefix(r.URL.Path, "/ui/") {
		page = r.URL.Path[4:]
	} else {
		page = "home"
	}

	if err := getHomeTemplate().Execute(w, &clientContext{
		BitcoinIdentity.GetAddress(),
		BitwrkUrl,
		page,
	}); err != nil {
		log.Println("Error rendering UI:", err)
	}
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
