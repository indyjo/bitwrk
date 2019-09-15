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

package assist

import (
	"github.com/indyjo/cafs"
	"testing"
)

func TestTicketMan(t *testing.T) {
	ts := NewTicketStore()

	h1234 := Handprint{fingers: [4]cafs.SKey{{1}, {2}, {3}, {4}}}
	ts1 := make([]ticket, 0)
	ts.InitNode("n1", &h1234, func(t string) { ts1 = append(ts1, t) })

	// Add matching ticket from same node, expect no effect
	ts.NewTicket("t1", "n1")
	if len(ts1) != 0 {
		t.Fatalf("Ticket assigned to originating node")
	}

	// Add matching node. Expect to get the ticket.
	ts2 := make([]ticket, 0)
	ts.InitNode("n2", &h1234, func(t string) { ts2 = append(ts2, t) })
	if len(ts2) != 1 {
		t.Fatalf("Expected to receive a ticket when node was added")
	}

	// Add ticket by node 2. Expect node 1 to get the ticket.
	ts.NewTicket("t3", "n2")
	if len(ts1) != 1 {
		t.Fatalf("Expected to receive ticket upon creation")
	}

	// Add another ticket by node 1. Expect noone to get the ticket.
	ts.NewTicket("t4", "n1")
	if len(ts1) != 1 {
		t.Fatalf("Didn't expect node 1 to get this ticketn")
	}
	if len(ts2) != 1 {
		t.Fatalf("Didn't expect node 2 to get this ticketn")
	}

	// Add a non-matching node. Expect it to get no ticket.
	h3456 := Handprint{fingers: [4]cafs.SKey{{3}, {4}, {5}, {6}}}
	ts3 := make([]ticket, 0)
	ts.InitNode("n3", &h3456, func(t string) { ts3 = append(ts3, t) })
	if len(ts3) != 0 {
		t.Fatalf("Didn't expect a ticket for non-matching node")
	}

	// Add a semi-matching node. Expect the ticket.
	h2345 := Handprint{fingers: [4]cafs.SKey{{2}, {3}, {4}, {5}}}
	ts4 := make([]ticket, 0)
	ts.InitNode("n4", &h2345, func(t string) { ts4 = append(ts4, t) })
	if len(ts4) != 1 {
		t.Fatalf("Expected a ticket for semi-matching node")
	}
}
