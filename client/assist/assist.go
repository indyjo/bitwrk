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
type edge struct{ from, to nodeid }

// Suggested name of HTTP header containing a ticket
const HeaderName = "X-AssistiveDownloadTicket"

type GrantTicketFunc = func(string)

// Interface TicketStore handles tickets necessary for managing an assistive transfer network.
type TicketStore interface {
	// Function InitNode tells the TicketStore that a node has begin its cycle.
	// The new node is interested in receiving assistive download tickets for files
	// matching the given handprint.
	// It may also offer new tickets and establish new connections.
	// Function `grantTicket` is called asynchronously.
	InitNode(node nodeid, handprint *Handprint, grantTicket GrantTicketFunc)

	// Function ExitNode tells the TicketStore that a node has finished its cycle.
	// It is no longer ready to establish connections. All unspent tickets are removed.
	// All previous incoming and outgoing connections are cleared.
	ExitNode(node nodeid)

	// Function NewTicket tells the TicketStore that a node offers an assistive download ticket
	// for the file whose Handprint was given in function InitNode.
	// The TicketStore tries to grant tickets to interested nodes. Tickets that can't be
	// granted right away are pooled for later use by appearing nodes. Unspent tickets are removed
	// when their offering node exits.
	NewTicket(ticket ticket, source nodeid)

	// Dumps a human-readable state representation to the given stream
	Dump(w io.Writer) error
}

const numFingers = 4
const numIndexes = numFingers + 1

func NewTicketStore() TicketStore {
	result := ticketMan{
		nodes:     make(map[nodeid]*nodeInfo),
		tickets:   make(map[ticket]*ticketInfo),
		edges:     make(map[edge]bool),
		pastEdges: make(map[edge]int),
	}
	return &result
}

type ticketMan struct {
	m         sync.Mutex
	nodes     map[nodeid]*nodeInfo   // Known nodes
	tickets   map[ticket]*ticketInfo // Unspent (pooled) tickets
	edges     map[edge]bool          // Which nodes are connected to each other currently
	pastEdges map[edge]int           // How many times nodes have been connected historically
}

type nodeInfo struct {
	nodeid    nodeid
	handprint *Handprint
	grantFunc GrantTicketFunc
}

type ticketInfo struct {
	ticket    ticket
	source    nodeid
	handprint *Handprint
}

func (t *ticketMan) InitNode(node nodeid, handprint *Handprint, grantTicket GrantTicketFunc) {
	t.m.Lock()
	defer t.m.Unlock()
	t.resetNode(node)
	nodeInfo := nodeInfo{
		nodeid:    node,
		handprint: handprint,
		grantFunc: grantTicket,
	}
	t.nodes[node] = &nodeInfo
	// Try to match the new node with all available tickets in the pool
	for _, ti := range t.tickets {
		t.match(&nodeInfo, ti)
	}
}

func (t *ticketMan) ExitNode(node nodeid) {
	t.m.Lock()
	defer t.m.Unlock()
	t.resetNode(node)
}

func (t *ticketMan) NewTicket(ticket ticket, source nodeid) {
	t.m.Lock()
	defer t.m.Unlock()

	if _, ok := t.tickets[ticket]; ok {
		return // Ticket already stored? NOP
	}

	var handprint *Handprint
	if n, ok := t.nodes[source]; !ok {
		return // Source not registered? NOP
	} else {
		handprint = n.handprint
	}

	ticketInfo := ticketInfo{
		ticket:    ticket,
		source:    source,
		handprint: handprint,
	}
	t.tickets[ticket] = &ticketInfo

	// Try to match the new ticket against available nodes
	for _, node := range t.nodes {
		if t.match(node, &ticketInfo) {
			return
		}
	}
}

func (t *ticketMan) resetNode(n nodeid) {
	// Delete node frm list of nodes
	delete(t.nodes, n)
	// Delete all edges involving node
	for e := range t.edges {
		if e.from == n || e.to == n {
			delete(t.edges, e)
		}
	}
	// Delete all unspent tickets of node
	for _, ti := range t.tickets {
		if ti.source == n {
			delete(t.tickets, ti.ticket)
		}
	}
}

func (t *ticketMan) Dump(w io.Writer) (err error) {
	t.m.Lock()
	defer t.m.Unlock()

	_, err = fmt.Fprintf(w, "Tickets\ndigraph tickets {\n")
	if err != nil {
		return
	}
	for _, ti := range t.tickets {
		_, err = fmt.Fprintf(w, " \"%v\" -> \"%v\";\n", ti.source, ti.handprint)
		if err != nil {
			return
		}
		_, err = fmt.Fprintf(w, "  \"%v\" -> \"%v\";\n", ti.handprint, ti.ticket)
		if err != nil {
			return
		}
	}
	_, err = fmt.Fprintf(w, "}\n\nConnections\ndigraph edges {\n")
	if err != nil {
		return
	}
	for edge := range t.edges {
		_, err = fmt.Fprintf(w, "  \"%v\" -> \"%v\";\n", edge.from, edge.to)
		if err != nil {
			return
		}
	}
	_, err = fmt.Fprintf(w, "}\n\nPast Connections\ndigraph past_edges {\n")
	if err != nil {
		return
	}
	for edge, n := range t.pastEdges {
		_, err = fmt.Fprintf(w, "  \"%v\" -> \"%v\" [label=\"%v\"];\n", edge.from, edge.to, n)
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

func (t *ticketMan) match(node *nodeInfo, ticket *ticketInfo) bool {
	if node.nodeid == ticket.source {
		return false // Never grant ticket to its source
	}
	edge := edge{ticket.source, node.nodeid}
	if t.edges[edge] {
		return false // There is already an edge connecting the ticket source and this node
	}
	matches := false
	ticketKeys := ticket.handprint.keys()
	for i, k := range node.handprint.keys() {
		if k == ticketKeys[i] {
			matches = true
			break
		}
	}
	if !matches {
		return false // Ticket doesn't match what the node is interested in
	}

	// Remove ticket from pool
	delete(t.tickets, ticket.ticket)

	// Add edge
	t.edges[edge] = true
	t.pastEdges[edge]++

	// Notify the world
	node.grantFunc(ticket.ticket)

	return true
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
