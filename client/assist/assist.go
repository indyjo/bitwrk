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

// Package assist contains logic for managing a network of peers where peers can establish assistive transmissions to
// forward file data between each other.
package assist

import (
	"bytes"
	"github.com/indyjo/cafs"
	"github.com/indyjo/cafs/remotesync"
	"sync"
)

// Tickets is a global instance of TicketStore
var Tickets = NewTicketStore()

type nodeid = string
type setOfNodeIds map[nodeid]bool
type ticket = string
type key string
type setOfTickets map[ticket]bool
type index map[key]setOfTickets

// Suggested name of HTTP header containing a ticket
const HeaderName = "X-AssistiveDownloadTicket"

type FilterFunc = func(interface{}) bool

// Interface TicketStore handles tickets necessary for managing an assistive transfer network.
type TicketStore interface {
	// Function ResetSource tells the TicketStore that a source node is now ready to offer new tickets.
	ResetSource(source nodeid)

	// Function AddTicket associates an assistive download ticket with a handprint.
	// When an upload connection to a peer has been established, and that peer has indicated
	// a ticket for other peers to establish an assistive download, this function stores the
	// ticket.
	AddTicket(ticket ticket, handprint *Handprint, source nodeid, userdata interface{})

	// Function GetTicket retrieves a ticket to assist a transfer matching `handprint`.
	// The ticket is consumed. This function is used when esdtablishing a new upload connection.
	TakeTicket(handprint *Handprint, target nodeid, filter FilterFunc) *ticket
}

const numFingers = 4
const numIndexes = numFingers + 1

func NewTicketStore() TicketStore {
	result := ticketMan{
		entries:  make(map[ticket]entry),
		bySource: make(map[nodeid]setOfTickets),
		edges:    make(map[nodeid]setOfNodeIds),
	}
	for i := range result.indexes {
		result.indexes[i] = make(index)
	}
	return &result
}

type ticketMan struct {
	m        sync.Mutex
	entries  map[ticket]entry
	indexes  [numIndexes]index
	bySource map[nodeid]setOfTickets
	edges    map[nodeid]setOfNodeIds
}

type entry struct {
	ticket    ticket
	userdata  interface{}
	handprint *Handprint
	source    nodeid
}

func (t *ticketMan) ResetSource(source nodeid) {
	t.m.Lock()
	defer t.m.Unlock()
	for ticket := range t.bySource[source] {
		t.remove(ticket)
	}
	delete(t.bySource, source)
	delete(t.edges, source)
}

func (t *ticketMan) AddTicket(ticket ticket, handprint *Handprint, source nodeid, userdata interface{}) {
	keys := handprint.keys()
	t.m.Lock()
	defer t.m.Unlock()
	if _, ok := t.entries[ticket]; ok {
		return // Ticket already stored? NOP
	}
	t.entries[ticket] = entry{
		ticket:    ticket,
		userdata:  userdata,
		handprint: handprint,
		source:    source,
	}
	for i, k := range keys {
		tickets := t.indexes[i][k]
		if tickets == nil {
			tickets = make(setOfTickets)
			t.indexes[i][k] = tickets
		}
		tickets[ticket] = true
	}
	tickets, ok := t.bySource[source]
	if !ok {
		tickets = make(setOfTickets)
		t.bySource[source] = tickets
	}
	tickets[ticket] = true
}

func (t *ticketMan) TakeTicket(handprint *Handprint, target nodeid, filter FilterFunc) *ticket {
	keys := handprint.keys()
	t.m.Lock()
	defer t.m.Unlock()
	for i, k := range keys {
		tickets := t.indexes[i][k]
		if tickets == nil {
			continue
		}
		// Take the first ticket matching our criteria out of the set
		for ticket := range tickets {
			var entry entry
			if e, ok := t.entries[ticket]; !ok {
				panic("inconsistent entries structure")
			} else {
				entry = e
			}

			if filter != nil && !filter(entry.userdata) {
				continue
			} else if entry.source == target {
				continue
			} else if t.edges[entry.source][target] {
				// there is already an edge between source and target
				continue
			}
			t.remove(ticket)
			t.addEdge(entry.source, target)
			return &ticket
		}
	}

	return nil
}

func (t *ticketMan) remove(ticket ticket) {
	// assumption: already locked
	entry, ok := t.entries[ticket]
	if !ok {
		return
	}
	keys := entry.handprint.keys()
	// Remove from each index
	for i, index := range t.indexes {
		key := keys[i]
		tickets := index[key]
		delete(tickets, ticket)
		if len(ticket) == 0 {
			delete(index, key)
		}
	}
	// Remove from entries map
	delete(t.entries, ticket)
}

func (t *ticketMan) addEdge(source nodeid, target nodeid) {
	targets, ok := t.edges[source]
	if !ok {
		targets = make(setOfNodeIds)
		t.edges[source] = targets
	}
	targets[target] = true
}

// Struct Handprint contains a set of fingerprints indicating a file.
type Handprint struct {
	fingers [numFingers]cafs.SKey
}

func (h *Handprint) keys() []key {
	var result [numIndexes]key
	keys := result[:0]
	buf := new(bytes.Buffer)
	for skip := -1; skip < numIndexes; skip++ {
		buf.Reset()
		for i, finger := range h.fingers {
			if i == skip {
				continue
			}
			buf.WriteString(finger.String())
		}
		keys = append(keys, key(buf.String()))
	}
	return result[:]
}

func HandprintFromSyncInfo(syncinfo *remotesync.SyncInfo) *Handprint {
	result := Handprint{}
	for i := range result.fingers {
		for j := range result.fingers[i] {
			result.fingers[i][j] = 0xff
		}
	}
	for _, ci := range syncinfo.Chunks {
		result.insert(ci.Key)
	}
	return &result
}

// Inserts a new fingerprint into the Handprint, keeping its ascending order, eliding the highest-valued fingerprint.
func (h *Handprint) insert(finger cafs.SKey) {
	for i, other := range h.fingers {
		cmd := bytes.Compare(other[:], finger[:])
		if cmd < 0 {
			continue
		} else if cmd == 0 {
			return
		}
		finger, h.fingers[i] = other, finger
	}
}
