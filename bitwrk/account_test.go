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

package bitwrk

import (
	"net/url"
	"testing"
)

func Test_DepositAddressMessage(t *testing.T) {
	for _, m := range []DepositAddressMessage{
		{
			Nonce:          "123456",
			DepositAddress: "TheDepositAddress",
			Participant:    "TheParticipant",
			Reference:      "Some random stuff",
			Signer:         "Mr. Faker",
		},
		{
			Nonce:          "123456",
			DepositAddress: "TheDepositAddress",
			Participant:    "TheParticipant",
			Reference:      "This is still fake, the signature doesn't match",
			Signer:         "1MLzeGTE3LoWPsaCwp6fuYFwjsQgR4hVPm",
			Signature:      "IKtBgTh4nCJbCs/i52J2+lI/IHRWoVExprwDWAscYzNeY2hT5sbP1Ppp/bmnrvAjXE9hhft74R+iT9aD4i9YBzM=",
		},
	} {
		expect(m, t, false)
	}

	for _, m := range []DepositAddressMessage{
		{
			Nonce:          "123456",
			DepositAddress: "TheDepositAddress",
			Participant:    "TheParticipant",
			Reference:      "This is a correct example",
			Signer:         "16pe6ppHN5oeho1jECjaULS74y1zoD3kmM",
			Signature:      "G/iK3V34QDpcwK9o70dCZu0bH4nzrwNWc/jA4HAujiIB8t2KE3Ex8DcgmpoPy7/uEURPqJpA9W5oRg6I5wF5yAk=",
		},
	} {
		expect(m, t, true)
	}
}

func expect(m DepositAddressMessage, t *testing.T, expectSuccess bool) {
	values := url.Values{}
	m.ToValues(values)
	t.Log(values.Encode())
	m2 := DepositAddressMessage{}
	m2.FromValues(values)

	if m2 != m {
		t.Fatal("Serialization + deserialization does not produce equal result.")
	} else {
		t.Logf("Serialization + deserialization produces equal objects.")
	}

	if err := m.VerifyWith(m.Signer); err != nil {
		if expectSuccess {
			t.Fatalf("Could not verify correct message: %v", err)
		} else {
			t.Logf("Verifying correctly complained: %v", err)
		}
	} else {
		if expectSuccess {
			t.Log("Correctly verified message.")
		} else {
			t.Fatalf("Verifying a fake deposit address message did not complain!")
		}
	}

}
