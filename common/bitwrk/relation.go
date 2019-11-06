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

package bitwrk

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/indyjo/bitwrk-common/bitcoin"
)

// Type RelationType specifies how to interpret a Relation.
type RelationType int

const (
	RELATION_TYPE_TRUSTS   RelationType = 1
	RELATION_TYPE_WORKSFOR RelationType = 2
)

// Type relation describes a relationship between two participants.
// Must always be signed by Source.
type Relation struct {
	Source, Target      string       // Participant IDs corresponding to both sides of the Relation
	Type                RelationType // What kind of releation this Relation models
	Enabled             bool         // Whether this relation is true or false
	Document, Signature string       // For verifying authenticity
	LastModified        time.Time    // When the relation was created or last modified
}

// Sentinel error returned on invalid relation types
var errNoSuchRelationType = errors.New("No such relation type")

func ParseRelationType(str string) (RelationType, error) {
	if str == "trusts" {
		return RELATION_TYPE_TRUSTS, nil
	} else if str == "worksfor" {
		return RELATION_TYPE_WORKSFOR, nil
	}
	return 0, errNoSuchRelationType
}

func (t RelationType) String() string {
	if t == RELATION_TYPE_TRUSTS {
		return "trusts"
	} else if t == RELATION_TYPE_WORKSFOR {
		return "worksfor"
	}
	return fmt.Sprintf("<invalid relation type: %d>", int(t))
}

func (t RelationType) MarshalJSON() ([]byte, error) {
	return []byte("\"" + t.String() + "\""), nil
}

func (t *RelationType) UnmarshalJSON(b []byte) error {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return fmt.Errorf("Innvalid relation type JSON: %#v", b)
	}
	if t2, err := ParseRelationType(string(b[1 : len(b)-1])); err != nil {
		return err
	} else {
		*t = t2
		return nil
	}
}

var errNoReflexive = errors.New("source and target may not be equal")
var errNoBoolean = errors.New("enabled must be true or false")

func ParseRelation(enabled, nonce, source, target, reltype, signature string) (*Relation, error) {
	if enabled != "true" && enabled != "false" {
		return nil, errNoBoolean
	}
	rtype, err := ParseRelationType(reltype)
	if err != nil {
		return nil, err
	}

	if source == target {
		return nil, errNoReflexive
	}

	// Construct the document which is checked against the signature.
	// It is built like a URL query, with parameters ordered strictly
	// alphabetically.
	document := fmt.Sprintf(
		"enabled=%s&nonce=%s&source=%s&target=%s&type=%s",
		normalize(enabled),
		normalize(nonce),
		normalize(source),
		normalize(target),
		normalize(reltype))

	result := Relation{
		Source:       source,
		Target:       target,
		Type:         rtype,
		Enabled:      enabled == "true",
		Document:     document,
		Signature:    signature,
		LastModified: time.Now(),
	}

	return &result, nil
}

func (r *Relation) Verify() error {
	if err := bitcoin.VerifySignatureBase64(r.Document, r.Source, r.Signature); err != nil {
		return fmt.Errorf("Could not validate signature: %v", err)
	}
	return nil
}

func (r *Relation) String() string {
	return fmt.Sprintf("%v -[%v:%v]-> %v", r.Source, r.Type, r.Enabled, r.Target)
}

// Signs the relation using the given key and random number sources.
func (r *Relation) SignWith(key *bitcoin.KeyPair, rand io.Reader, nonce string) error {
	doc := fmt.Sprintf(
		"enabled=%v&nonce=%s&source=%s&target=%s&type=%v",
		r.Enabled,
		normalize(nonce),
		normalize(r.Source),
		normalize(r.Target),
		r.Type)
	if sig, err := key.SignMessage(doc, rand); err != nil {
		return err
	} else {
		r.Document = doc
		r.Signature = sig
		return nil
	}
}
