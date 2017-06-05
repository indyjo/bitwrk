//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2014-2015  Jonas Eschenburg <jonas@bitwrk.net>
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
	"appengine/memcache"
	"appengine/user"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/indyjo/bitwrk-common/bitwrk"
	"github.com/indyjo/bitwrk-common/money"
	db "github.com/indyjo/bitwrk-server/gae"
	"io"
	"net/http"
	"strconv"
	"time"
)

func handleQueryAccounts(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	limitStr := r.FormValue("limit")
	var limit int
	if limitStr == "" {
		limit = 100
	} else if n, err := strconv.ParseUint(limitStr, 10, 10); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		limit = int(n)
	}

	requestdepositaddress := r.FormValue("requestdepositaddress") != ""

	buffer := new(bytes.Buffer)
	handler := func(key string) {
		fmt.Fprintf(buffer, "%v\n", key)
	}

	if err := db.QueryAccountKeys(c, limit, requestdepositaddress, handler); err != nil {
		c.Errorf("QueryAccountKeys failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	buffer.WriteTo(w)
}

type timeslot struct {
	Begin time.Time   `json:"begin"`
	End   time.Time   `json:"end"`
	Sum   money.Money `json:"sum"`
	Min   money.Money `json:"min"`
	Max   money.Money `json:"max"`
	Count int         `json:"count"`
}

// Adds a single price to the statistic
func (s *timeslot) addPrice(price money.Money) {
	if s.Count == 0 {
		s.Sum, s.Min, s.Max = price, price, price
	} else {
		s.Sum = s.Sum.Add(price)
		if price.Amount < s.Min.Amount {
			s.Min.Amount = price.Amount
		}
		if price.Amount > s.Max.Amount {
			s.Max.Amount = price.Amount
		}
	}
	s.Count++
}

// Moves the window one position forward and clears accumulators
func (s *timeslot) advance() {
	interval := s.End.Sub(s.Begin)
	s.Begin = s.End
	s.End = s.End.Add(interval)
	s.Count = 0
	s.Sum.Amount = 0
	s.Min.Amount = 0
	s.Max.Amount = 0
}

type resolution struct {
	name     string
	interval time.Duration
}

func (a resolution) finerThan(b resolution) bool {
	return a.interval < b.interval
}

var resolutions = []resolution{
	{"1y", 9 * 42 * 24 * time.Hour},
	{"6w", 42 * 24 * time.Hour},
	{"1w", 7 * 24 * time.Hour},
	{"1d", 24 * time.Hour},
	{"6h", 6 * time.Hour},
	{"1h", 1 * time.Hour},
	{"12m", 12 * time.Minute},
	{"3m", 3 * time.Minute},
	{"30s", 30 * time.Second},
	{"6s", 6 * time.Second},
	{"1s", 1 * time.Second},
}

var resolutionsByName = func() map[string]int {
	result := make(map[string]int)
	for idx, r := range resolutions {
		result[r.name] = idx
	}
	return result
}()

func resolutionExists(name string) bool {
	_, ok := resolutionsByName[name]
	return ok
}

func resolutionByName(name string) resolution {
	return resolutions[resolutionsByName[name]]
}

func (r resolution) nextFiner() resolution {
	return resolutions[resolutionsByName[r.name]+1]
}

func (r resolution) isFinest() bool {
	return resolutionsByName[r.name] == len(resolutions)-1
}

func (r resolution) coarsestTileResolution() resolution {
	idx := resolutionsByName[r.name] - 5
	if idx < 0 {
		idx = 0
	}
	return resolutions[idx]
}

func (r resolution) finestTileResolution() resolution {
	idx := resolutionsByName[r.name] - 2
	if idx < 0 {
		idx = 0
	}
	return resolutions[idx]
}

func handleQueryPrices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	c := appengine.NewContext(r)

	needLogin := false

	articleStr := r.FormValue("article")
	var article bitwrk.ArticleId
	if articleStr == "" {
		http.Error(w, "article argument missing", http.StatusNotFound)
		return
	} else if err := checkArticle(c, articleStr); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	} else {
		article = bitwrk.ArticleId(articleStr)
	}

	periodStr := r.FormValue("period")
	if periodStr == "" {
		periodStr = "1d"
	} else if !resolutionExists(periodStr) {
		http.Error(w, "period unknown", http.StatusNotFound)
		return
	}
	period := resolutionByName(periodStr)

	resolutionStr := r.FormValue("resolution")
	if resolutionStr == "" {
		resolutionStr = "3m"
	} else if !resolutionExists(resolutionStr) {
		http.Error(w, "resolution unknown", http.StatusNotFound)
		return
	}
	resolution := resolutionByName(resolutionStr)

	if period.interval > 10000*resolution.interval {
		// User requested a lot of data... restrict for now
		needLogin = true
	}

	// If begin is not given, calculate from period
	beginStr := r.FormValue("begin")
	var begin time.Time
	if beginStr == "" {
		begin = time.Now().Add(-period.interval)
	} else if t, err := time.Parse(time.RFC3339, beginStr); err != nil {
		http.Error(w, "Invalid begin time", http.StatusNotFound)
		return
	} else {
		begin = t
		// Explicitly stating begin requires athentication for now
		needLogin = true
	}

	unitStr := r.FormValue("unit")
	var unit money.Unit
	if unitStr == "" {
		unit = money.MustParseUnit("mBTC")
	} else if u, err := money.ParseUnit(unitStr); err != nil {
		http.Error(w, "Invalid unit parameter", http.StatusNotFound)
	} else {
		unit = u
	}

	// Calculate end from begin and period
	end := begin.Add(period.interval)

	// Lookup coarsest tile resolution
	tile := resolution.coarsestTileResolution()
	if period.finerThan(tile) {
		tile = period
	}
	if tile.finerThan(resolution.finestTileResolution()) {
		tile = resolution.finestTileResolution()
	}

	// Truncate begin to a multiple of coarsest tile resolution.
	begin = begin.Truncate(tile.interval)

	// Enforce admin permissions if necessary
	if needLogin && !user.IsAdmin(c) {
		http.Error(w, "Action requires admin privileges", http.StatusForbidden)
		return
	}

	if prices, err := queryPrices(c, article, tile, resolution, begin, end); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if r.FormValue("format") == "flot" {
		w.Header().Set("Content-Type", "application/json")
		renderPricesForFlot(w, prices, unit)
	} else if data, err := json.Marshal(prices); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		// Write result back to requester
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

func renderPricesForFlot(w io.Writer, slots []timeslot, unit money.Unit) {
	fmt.Fprintf(w, "[\n")
	firstLine := true
	var lastSlot timeslot
	for _, slot := range slots {
		var comma string
		if firstLine {
			firstLine = false
			comma = " "
		} else {
			if lastSlot.End != slot.Begin {
				fmt.Fprintln(w, ", null")
			}
			comma = ","
		}
		
		avg := money.Money{Currency: slot.Sum.Currency, Amount: slot.Sum.Amount / int64(slot.Count)}
		avgStr := avg.Format(unit, false)
		fmt.Fprintf(w, "%v[%v, %v], [%v, %v]\n",
			comma,
			slot.Begin.UnixNano() / 1000000, avgStr,
			slot.End.UnixNano() / 1000000, avgStr)
		
		lastSlot = slot
	}
	fmt.Fprintf(w, "]\n")
} 

// Returns prices for trades between 'begin' and 'end', in resolution 'res', recursing from tile size 'tile' down to the
// most appropriate tile size, employing caching on the go.
// Assumes that 'begin' is aligned with the current tile size.
func queryPrices(c appengine.Context, article bitwrk.ArticleId, tile, res resolution, begin, end time.Time) ([]timeslot, error) {
	finest := res.finestTileResolution()
	if tile.finerThan(finest) {
		panic("Tile size too small")
	}

	// Does the interval fit into a single tile? If no, fan out and don't cache.
	if begin.Add(tile.interval).Before(end) {
		result := make([]timeslot, 0)
		count := 0
		for begin.Before(end) {
			tileEnd := begin.Add(tile.interval)
			if tileEnd.After(end) {
				tileEnd = end
			}

			if r, err := queryPrices(c, article, tile, res, begin, tileEnd); err != nil {
				return nil, err
			} else {
				c.Infof("Fan-out to tile #%v returned %v slots", count, len(r))
				result = append(result, r...)
			}
			begin = tileEnd
			count++
		}
		c.Infof("Fan-out to %v tiles returned %v slots", count, len(result))
		return result, nil
	}

	// First try to answer from cache
	key := fmt.Sprintf("prices-tile-%v/%v-%v-%v", tile.name, res.name, begin.Format(time.RFC3339), article)
	if item, err := memcache.Get(c, key); err == nil {
		result := make([]timeslot, 0)
		if err := json.Unmarshal(item.Value, &result); err != nil {
			// Shouldn't happen
			c.Errorf("Couldn't unmarshal memcache entry for: %v : %v", key, err)
		} else {
			return result, nil
		}
	}

	// Cache miss. Need to fetch data.
	// If tile size is the smallest for the desired resolution, ask the datastore.
	// Otherwise, recurse with next smaller tile size.
	var result []timeslot
	if tile == res.finestTileResolution() {

		count := 0
		currentSlot := timeslot{
			Begin: begin,
			End:   begin.Add(res.interval),
		}
		result = make([]timeslot, 0)

		// Handler function that is called for every transaction in the current tile
		handler := func(key string, tx bitwrk.Transaction) {
			// Flush current interval to result list
			for currentSlot.End.Before(tx.Matched) {
				if currentSlot.Count > 0 {
					result = append(result, currentSlot)
					count += currentSlot.Count
				}
				currentSlot.advance()
			}
			currentSlot.addPrice(tx.Price)
		}

		// Query database
		if err := db.QueryTransactions(c, 10000, article, begin, end, handler); err != nil {
			return nil, err
		}

		// Flush last interval
		if currentSlot.Count > 0 {
			result = append(result, currentSlot)
			count += currentSlot.Count
		}

		c.Infof("QueryTransactions from %v to %v: %v slots/%v tx",
			begin, end, len(result), count)
	} else if r, err := queryPrices(c, article, tile.nextFiner(), res, begin, end); err != nil {
		return nil, err
	} else {
		result = r
	}

	// Before returning, update the cache.
	item := memcache.Item{Key: key}
	if data, err := json.Marshal(result); err != nil {
		// Shouldn't happen
		c.Errorf("Error marshalling result: %v", err)
	} else {
		item.Value = data
	}

	// Tiles very close to now expire after 10 seconds
	if begin.Add(tile.interval).After(time.Now().Add(-2 * time.Minute)) {
		item.Expiration = 10 * time.Second
	}

	if err := memcache.Add(c, &item); err != nil {
		c.Errorf("Error caching item for %v: %v", key, err)
	}

	return result, nil
}

// Query for a list of transactions. Admin-only for now.
func handleQueryTrades(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	if !user.IsAdmin(c) {
		http.Error(w, "Action requires admin privileges", http.StatusForbidden)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")

	limitStr := r.FormValue("limit")
	var limit int
	if limitStr == "" {
		limit = 1000
	} else if n, err := strconv.ParseUint(limitStr, 10, 14); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		limit = int(n)
	}

	articleStr := r.FormValue("article")
	var article bitwrk.ArticleId
	if articleStr == "" {
		http.Error(w, "article argument missing", http.StatusNotFound)
		return
	} else if err := checkArticle(c, articleStr); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	} else {
		article = bitwrk.ArticleId(articleStr)
	}

	periodStr := r.FormValue("period")
	if periodStr == "" {
		periodStr = "1d"
	} else if !resolutionExists(periodStr) {
		http.Error(w, "period unknown", http.StatusNotFound)
		return
	}
	period := resolutionByName(periodStr)

	// If begin is not given, calculate
	beginStr := r.FormValue("begin")
	var begin time.Time
	if beginStr == "" {
		begin = time.Now().Add(-period.interval)
	} else if t, err := time.Parse(time.RFC3339, beginStr); err != nil {
		http.Error(w, "Invalid begin time", http.StatusNotFound)
		return
	} else {
		begin = t
	}

	end := begin.Add(period.interval)

	unit := money.MustParseUnit("mBTC")
	buffer := new(bytes.Buffer)
	fmt.Fprintf(buffer, "{\"begin\": %#v, \"end\": %#v, \"unit\": \"%v\", \"data\": [\n",
		begin.Format(time.RFC3339), end.Format(time.RFC3339), unit)
	count := 0
	priceSum := money.MustParse("BTC 0")
	feeSum := money.MustParse("BTC 0")

	firstLine := true
	handler := func(key string, tx bitwrk.Transaction) {
		var comma string
		if firstLine {
			firstLine = false
			comma = " "
		} else {
			comma = ","
		}
		fmt.Fprintf(buffer, "%v[% 5d, %#v, %v, %v, %v, \"%v\", \"%v\", %#v]\n", comma,
			count,
			tx.Matched.Format(time.RFC3339Nano), tx.Matched.UnixNano()/1000000,
			tx.Price.Format(unit, false),
			tx.Fee.Format(unit, false),
			tx.State, tx.Phase, key)

		priceSum = priceSum.Add(tx.Price)
		feeSum = feeSum.Add(tx.Fee)
		count++
	}
	if err := db.QueryTransactions(c, limit, article, begin, end, handler); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		c.Errorf("Error querying transactions: %v", err)
		return
	}
	fmt.Fprintf(buffer, "], \"price_sum\": %v,  \"fee_sum\": %v}\n",
		priceSum.Format(unit, false), feeSum.Format(unit, false))

	// Write result back to requester
	w.Header().Set("Content-Type", "application/json")
	buffer.WriteTo(w)
}
