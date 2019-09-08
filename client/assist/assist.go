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
	"fmt"
	"github.com/indyjo/cafs"
	"github.com/indyjo/cafs/remotesync"
	"io"
	"sync"
)

// Tickets is a global instance of TicketStore
var Tickets = NewTicketStore()

type nodeid = string
type ticket = string
type key string
type setOfTickets map[ticket]bool
type index map[key]setOfTickets
type edge struct{ from, to nodeid }

// Suggested name of HTTP header containing a ticket
const HeaderName = "X-AssistiveDownloadTicket"

type FilterFunc = func(interface{}) bool

// Interface TicketStore handles tickets necessary for managing an assistive transfer network.
type TicketStore interface {
	// Function ResetNode tells the TicketStore that a node has reset its cycle.
	// All incoming and outgoing connections are cleared.
	// The node is now ready to offer new tickets and establish new connections.
	ResetNode(node nodeid)

	// Function AddTicket associates an assistive download ticket for a file identified by a
	// handprint with a node offering the download.
	// When an upload connection to a peer has been established, and that peer has indicated
	// a ticket for other peers to establish an assistive download, this function stores the
	// ticket.
	AddTicket(ticket ticket, handprint *Handprint, source nodeid, userdata interface{})

	// Function GetTicket retrieves a ticket to assist a transfer matching `handprint`.
	// The ticket is consumed. This function is used when establishing a new upload connection.
	// It is ensured that no more than one connection exists at eny time between a source node
	// and a target node.
	TakeTicket(handprint *Handprint, target nodeid, filter FilterFunc) *ticket

	// Dumps a human-readable state representation to the given stream
	Dump(w io.Writer) error
}

const numFingers = 4
const numIndexes = numFingers + 1

func NewTicketStore() TicketStore {
	result := ticketMan{
		entries:   make(map[ticket]entry),
		bySource:  make(map[nodeid]setOfTickets),
		edges:     make(map[edge]bool),
		pastEdges: make(map[edge]int),
	}
	for i := range result.indexes {
		result.indexes[i] = make(index)
	}
	return &result
}

type ticketMan struct {
	m         sync.Mutex
	entries   map[ticket]entry
	indexes   [numIndexes]index
	bySource  map[nodeid]setOfTickets
	edges     map[edge]bool
	pastEdges map[edge]int
}

type entry struct {
	ticket    ticket
	userdata  interface{}
	handprint *Handprint
	source    nodeid
}

func (t *ticketMan) ResetNode(node nodeid) {
	t.m.Lock()
	defer t.m.Unlock()
	for ticket := range t.bySource[node] {
		t.remove(ticket)
	}
	delete(t.bySource, node)
	for e := range t.edges {
		if e.from == node || e.to == node {
			delete(t.edges, e)
			t.pastEdges[e] = t.pastEdges[e] + 1
		}
	}
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
			} else if t.edges[edge{entry.source, target}] {
				// there is already an edge between source and target
				continue
			}
			t.remove(ticket)
			t.edges[edge{entry.source, target}] = true
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
	// Remove ticket from source
	delete(t.bySource[entry.source], entry.ticket)
	if len(t.bySource[entry.source]) == 0 {
		// Remove source altogether if no tickets left.
		delete(t.bySource, entry.source)
	}
	// Remove from entries map
	delete(t.entries, ticket)
}

func (t *ticketMan) Dump(w io.Writer) (err error) {
	t.m.Lock()
	defer t.m.Unlock()

	_, err = fmt.Fprintf(w, "Tickets by source\n")
	if err != nil {
		return
	}
	for source, tickets := range t.bySource {
		_, err = fmt.Fprintf(w, " %v\n", source)
		if err != nil {
			return
		}
		for ticket := range tickets {
			entry := t.entries[ticket]
			_, err = fmt.Fprintf(w, "  %v => %v\n", entry.handprint, entry.ticket)
			if err != nil {
				return
			}
		}
	}
	_, err = fmt.Fprintf(w, "\nConnections\ndigraph edges{\n")
	if err != nil {
		return
	}
	for edge := range t.edges {
		_, err = fmt.Fprintf(w, "  _%v -> _%v ;\n", edge.from, edge.to)
		if err != nil {
			return
		}
	}
	_, err = fmt.Fprintf(w, "}\n")
	_, err = fmt.Fprintf(w, "\nPast Connections\ndigraph pastedges{\n")
	if err != nil {
		return
	}
	for edge, n := range t.pastEdges {
		_, err = fmt.Fprintf(w, "  _%v -> _%v [label=\"%v\"];\n", edge.from, edge.to, n)
		if err != nil {
			return
		}
	}
	_, err = fmt.Fprintf(w, "}\n")
	if err != nil {
		return
	}
	return
}

// Struct Handprint contains a set of fingerprints indicating a file.
type Handprint struct {
	fingers [numFingers]cafs.SKey
}

func (h *Handprint) String() string {
	result := ""
	for i, finger := range h.fingers {
		if i > 0 {
			result = result + "."
		}
		result = result + fmt.Sprintf("%x", finger[len(finger)-2:])
	}
	return result
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
