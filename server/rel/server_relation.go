//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2019-2019  Jonas Eschenburg <jonas@bitwrk.net>
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

// Package rel deals with the management of relations between accounts.
package rel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"

	"github.com/indyjo/bitwrk/common/bitwrk"
	"github.com/indyjo/bitwrk/server/config"
	"github.com/indyjo/bitwrk/server/gae"
	nonce2 "github.com/indyjo/bitwrk/server/nonce"
	"github.com/indyjo/bitwrk/server/util"
)

const createHtml = `
<!doctype html>
<html>
<head><title>Enter Relation</title></head>
<script src="/js/getnonce.js" ></script>
<script src="/js/createrel.js" ></script>
<body onload="getnonce()">
<form action="/rel" method="post">
<input id="source" type="text" name="source" size="64" value="1BiTWrKBPKT2yKdfEw77EAsCHgpjkqgPkv" onclick="select()" onchange="update()" /> &larr; The source account.<br />
<select id="type" name="type">
<option value="trusts" selected>trusts</option>
<option value="worksfor">works for</option>
</select> &larr; Choose the type of relation you would like to establish<br />
<input id="target" type="text" name="target" size="64" value="1BiTWrKBPKT2yKdfEw77EAsCHgpjkqgPkv" onclick="select()" onchange="update()" /> &larr; The target account.<br />
<select id="enabled" name="enabled">
<option value="true" selected>enabled</option>
<option value="false">disabled</option>
</select> &larr; Choose whether you would like to enable or disable this relation.<br />
<input id="nonce" type="hidden" name="nonce" onchange="update()"/> <br/>
<input type="text" name="signature" size="64" placeholder="Signature of query parameters" />
<input type="submit" />
</form>
<br />
Sign this text using the source address to confirm:<br />
<input id="query" type="text" size="180" onclick="select()" readonly/>
</body>
</html>
`

var createTemplate = template.Must(template.New("depositCreate").Parse(createHtml))

// Handler function for /rel/*
func HandleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		if err := createTemplate.Execute(w, nil); err != nil {
			http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
		}
	} else if r.Method == "POST" {
		c := appengine.NewContext(r)

		relType := r.FormValue("type")
		source := r.FormValue("source")
		target := r.FormValue("target")
		nonce := r.FormValue("nonce")
		enabled := r.FormValue("enabled")
		signature := r.FormValue("signature")

		if err := createRelation(c, relType, source, target, nonce, enabled, signature); err != nil {
			http.Error(w, "Error creating relation: "+err.Error(), http.StatusInternalServerError)
		} else {
			http.Redirect(w, r,
				"/rel/"+source+"/"+relType+"/"+target,
				http.StatusFound)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

var errSourceEqualsTarget = errors.New("source and target must differ")

func createRelation(c context.Context, relType, source, target, nonce, enabled, signature string) (err error) {
	// Important: checking (and invalidating) the nonce must be the first thing we do!
	err = nonce2.CheckNonce(c, nonce)
	if config.CfgRequireValidNonce && err != nil {
		return fmt.Errorf("Error in checkNonce: %v", err)
	}

	// Bitcoin addresses must have the right network id
	err = util.CheckBitcoinAddress(source)
	if err != nil {
		return
	}

	err = util.CheckBitcoinAddress(target)
	if err != nil {
		return
	}

	if source == target {
		err = errSourceEqualsTarget
		return
	}

	relation, err := bitwrk.ParseRelation(enabled, nonce, source, target, relType, signature)
	if err != nil {
		return
	}

	if config.CfgRequireValidSignature {
		if err := relation.Verify(); err != nil {
			return err
		}
	}

	// No need to run in transaction, there is only one write operation and no read
	dao := gae.NewGaeAccountingDao(c, false)
	return dao.SaveRelation(relation)
}

// Handler function for /rel/source/type/target
func HandleRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Only GET allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.SplitN(r.URL.Path[5:], "/", 4)
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}

	var rtype bitwrk.RelationType
	if t, err := bitwrk.ParseRelationType(parts[1]); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		rtype = t
	}

	c := appengine.NewContext(r)
	dao := gae.NewGaeAccountingDao(c, false)

	var relation *bitwrk.Relation
	if rn, err := dao.GetRelation(parts[0], parts[2], rtype); err == bitwrk.ErrNoSuchObject {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		relation = rn
	}

	if err := json.NewEncoder(w).Encode(relation); err != nil {
		log.Errorf(c, "Error encoding JSON: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
