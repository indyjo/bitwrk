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

// Suggested name of HTTP header containing a ticket
const HeaderName = "X-AssistiveDownloadTicket"

type FilterFunc = func(interface{}) bool

// Interface TicketStore handles tickets necessary for managing an assistive transfer network.
type TicketStore interface {
	// Function AddTicket associates an assistive download ticket with a handprint.
	// When an upload connection to a peer has been established, and that peer has indicated
	// a ticket for other peers to establish an assistive download, this function stores the
	// ticket.
	AddTicket(ticket string, handprint *Handprint, userdata interface{})

	// Function GetTicket retrieves a ticket to assist a transfer matching `handprint`.
	// The ticket is consumed. This function is used when esdtablishing a new upload connection.
	TakeTicket(handprint *Handprint, filter FilterFunc) *string
}

const numFingers = 4
const numIndexes = numFingers + 1

func NewTicketStore() TicketStore {
	result := ticketMan{
		entries: make(map[string]entry),
	}
	for i := range result.indexes {
		result.indexes[i] = make(map[string]string)
	}
	return &result
}

type ticketMan struct {
	m       sync.Mutex
	entries map[string]entry
	indexes [numIndexes]map[string]string
}

type entry struct {
	userdata  interface{}
	handprint *Handprint
}

func (t *ticketMan) AddTicket(ticket string, handprint *Handprint, userdata interface{}) {
	keys := handprint.keys()
	t.m.Lock()
	defer t.m.Unlock()
	if _, ok := t.entries[ticket]; ok {
		return // Ticket already stored? NOP
	}
	t.entries[ticket] = entry{
		userdata:  userdata,
		handprint: handprint,
	}
	for i, k := range keys {
		t.indexes[i][k] = ticket
	}
}

func (t *ticketMan) TakeTicket(handprint *Handprint, filter FilterFunc) *string {
	keys := handprint.keys()
	t.m.Lock()
	defer t.m.Unlock()
	for i, k := range keys {
		if ticket, ok := t.indexes[i][k]; !ok {
			continue
		} else if entry, ok := t.entries[ticket]; !ok {
			panic("inconsistent entries map")
		} else if filter != nil && !filter(entry.userdata) {
			continue
		} else {
			t.remove(ticket, keys)
			return &ticket
		}
	}
	return nil
}

func (t *ticketMan) remove(ticket string, keys []string) {
	// assumption: already locked
	// Remove from each index
	for i, index := range t.indexes {
		delete(index, keys[i])
	}
	// Remove from entries map
	delete(t.entries, ticket)
}

// Struct Handprint contains a set of fingerprints indicating a file.
type Handprint struct {
	fingers [numFingers]cafs.SKey
}

func (hand *Handprint) keys() []string {
	var result [numIndexes]string
	keys := result[:0]
	buf := new(bytes.Buffer)
	for skip := -1; skip < numIndexes; skip++ {
		buf.Reset()
		for i, finger := range hand.fingers {
			if i == skip {
				continue
			}
			buf.WriteString(finger.String())
		}
		keys = append(keys, buf.String())
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
