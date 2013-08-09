//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013  Jonas Eschenburg <jonas@bitwrk.net>
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
	"bitwrk/bitcoin"
	"bitwrk/money"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"time"
)

type (
	Origin     int8
	Thash      [32]byte
	Tsignature [65]byte
	Treceipt   struct {
		Hash          Thash
		HashSignature Tsignature
	}
	Tkey [32]byte
)

func (hash *Thash) String() string {
	return hex.EncodeToString(hash[:])
}

func (hash *Thash) MarshalJSON() ([]byte, error) {
	return []byte("\"" + hash.String() + "\""), nil
}

func unmarshalJSONHex(out, in []byte) error {
	s := ""
	if err := json.Unmarshal(in, &s); err != nil {
		return err
	}
	if b, err := hex.DecodeString(s); err != nil {
		return err
	} else if len(b) != len(out) {
		return fmt.Errorf("Length mismatch: %v != %v", len(b), len(out))
	} else {
		copy(out, b)
	}

	return nil
}

func (hash *Thash) UnmarshalJSON(b []byte) error {
	return unmarshalJSONHex(hash[:], b)
}

func (sig *Tsignature) String() string {
	return base64.StdEncoding.EncodeToString(sig[:])
}

func (sig *Tsignature) MarshalJSON() ([]byte, error) {
	return []byte("\"" + sig.String() + "\""), nil
}

func (sig *Tsignature) UnmarshalJSON(b []byte) error {
	return unmarshalJSONHex(sig[:], b)
}

const (
	FromBuyer Origin = iota
	FromSeller
	FromUnknown
)

func (o Origin) String() string {
	switch o {
	case FromBuyer:
		return "Buyer"
	case FromSeller:
		return "Seller"
	case FromUnknown:
		return "Unknown"
	}
	return "From Invalid"
}

// Tmessage objects are associated 1:N with transactions
type Tmessage struct {
	Received            time.Time
	Document            string
	Signature           string
	From                Origin
	Accepted            bool
	RejectMessage       string
	PrePhase, PostPhase TxPhase
}

type TxPhase int8

const (
	PhaseEstablishing TxPhase = iota
	PhaseBuyerEstablished
	PhaseSellerEstablished
	PhaseTransmitting
	PhaseWorking
	PhaseUnverified
	PhaseFinished
	PhaseWorkDisputed
	PhaseResultDisputed
)

type TxState int8

const (
	StateActive TxState = iota
	StateRetired
)

func (phase TxPhase) String() string {
	switch phase {
	case PhaseEstablishing:
		return "ESTABLISHING"
	case PhaseBuyerEstablished:
		return "BUYER_ESTABLISHED"
	case PhaseSellerEstablished:
		return "SELLER_ESTABLISHED"
	case PhaseTransmitting:
		return "TRANSMITTING"
	case PhaseWorking:
		return "WORKING"
	case PhaseUnverified:
		return "UNVERIFIED"
	case PhaseFinished:
		return "FINISHED"
	case PhaseWorkDisputed:
		return "WORK_DISPUTED"
	case PhaseResultDisputed:
		return "RESULT_DISPUTED"
	}
	return fmt.Sprintf("<Unknown TxPhase %v>", int8(phase))
}

func (phase *TxPhase) Parse(s string) error {
	switch s {
	case "ESTABLISHING":
		*phase = PhaseEstablishing
	case "BUYER_ESTABLISHED":
		*phase = PhaseBuyerEstablished
	case "SELLER_ESTABLISHED":
		*phase = PhaseSellerEstablished
	case "TRANSMITTING":
		*phase = PhaseTransmitting
	case "WORKING":
		*phase = PhaseWorking
	case "UNVERIFIED":
		*phase = PhaseUnverified
	case "FINISHED":
		*phase = PhaseFinished
	case "WORK_DISPUTED":
		*phase = PhaseWorkDisputed
	case "RESULT_DISPUTED":
		*phase = PhaseResultDisputed
	default:
		return fmt.Errorf("Invalid phase %#v", s)
	}

	return nil
}

func (phase TxPhase) MarshalJSON() ([]byte, error) {
	return []byte("\"" + phase.String() + "\""), nil
}

func (phase *TxPhase) UnmarshalJSON(b []byte) error {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return fmt.Errorf("Innvalid phase JSON: %#v", b)
	}
	return phase.Parse(string(b[1 : len(b)-1]))
}

type Transaction struct {
	// Revision number used for caching
	Revision            int
	BuyerBid, SellerBid string
	Buyer, Seller       string
	Article             ArticleId
	Price, Fee          money.Money
	Matched             time.Time
	State               TxState
	Phase               TxPhase
	Timeout             time.Time

	// --> ESTABLISHING phase

	// URL the worker (usually the seller) wishes to reveive data over (via POST, together with
	// BuyerSecret)
	WorkerURL *string

	// Hash of work data, sent by Buyer
	WorkHash *Thash
	// Hash of (WorkHash|BuyerNonce)
	WorkSecretHash *Thash

	// --> TRANSMITTING phase

	// Secret random number (same size as hashes)
	//  - Initially generated by buyer.
	//  - Sent to seller together with (but after) work data.
	//  - Published by seller as proof of having received work and to
	//    signal that the work package is accepted and is being worked on.
	//  - Everyone can verify WorkSecretHash once BuyerSecret is known.
	BuyerSecret *Thash
	// --> WORKING phase

	// Alternatively, the seller can also decide to reject the work
	// --> WORK_DISPUTED

	// The seller starts working and transmits the result back to the buyer.
	// The result, however, is sent encrypted and the buyer must sign a receipt.
	// for having received the encrypted result.
	// By publishing the receipt, alongside with the key necessary for decryption,
	// the seller proves completion in time and it is for the buyer to decide
	// whether to accept or reject the result.
	EncryptedResultReceipt *Treceipt
	ResultDecryptionKey    *Tkey

	// --> UNVERIFIED phase

	// This phase is terminated by the buyer either accepting the result
	//   --> FINISHED
	// or rejecting it
	//   --> RESULT_DISPUTED
}

type messageHandlerFunc func(*Transaction, map[string]string) error

func handleMessageDefault(_ *Transaction, _ map[string]string) error {
	return nil
}

type messageType struct {
	from      Origin
	handler   messageHandlerFunc
	arguments []string
}

func (t messageType) with(handler messageHandlerFunc) messageType {
	t.handler = handler
	return t
}

func makeMessageType(from Origin, arguments ...string) messageType {
	return messageType{from, handleMessageDefault, arguments}
}

type phaseTransition struct {
	prePhase, postPhase TxPhase
}

type phaseTransitionRule struct {
	messageType      messageType
	phaseTransitions []phaseTransition
}

var phaseTransitionRules = []phaseTransitionRule{
	{makeMessageType(FromBuyer, "workhash", "worksecrethash").with(handleWorkHashes),
		[]phaseTransition{
			{PhaseEstablishing, PhaseBuyerEstablished},
			{PhaseSellerEstablished, PhaseTransmitting}}},
	{makeMessageType(FromSeller, "workerurl").with(handleWorkerUrl),
		[]phaseTransition{
			{PhaseEstablishing, PhaseSellerEstablished},
			{PhaseBuyerEstablished, PhaseTransmitting}}},
	{makeMessageType(FromSeller, "buyersecret").with(handleBuyerSecret),
		[]phaseTransition{
			{PhaseBuyerEstablished, PhaseWorking},
			{PhaseTransmitting, PhaseWorking}}},
	{makeMessageType(FromSeller, "rejectwork"),
		[]phaseTransition{
			{PhaseSellerEstablished, PhaseWorkDisputed},
			{PhaseTransmitting, PhaseWorkDisputed}}},
	{makeMessageType(FromSeller, "encresulthash", "encresulthashsig", "encresultkey").with(handleTransmitFinished),
		[]phaseTransition{
			{PhaseWorking, PhaseUnverified}}},
	{makeMessageType(FromBuyer, "rejectresult"),
		[]phaseTransition{
			{PhaseUnverified, PhaseWorkDisputed}}},
	{makeMessageType(FromBuyer, "acceptresult"),
		[]phaseTransition{
			{PhaseEstablishing, PhaseFinished},
			{PhaseBuyerEstablished, PhaseFinished},
			{PhaseSellerEstablished, PhaseFinished},
			{PhaseTransmitting, PhaseFinished},
			{PhaseWorking, PhaseFinished},
			{PhaseUnverified, PhaseFinished}}},
}

func (tx *Transaction) findMatchingRule(address string, arguments map[string]string) *phaseTransitionRule {
rules:
	for _, rule := range phaseTransitionRules {
		if rule.messageType.from == FromBuyer && address != tx.Buyer {
			continue
		}
		if rule.messageType.from == FromSeller && address != tx.Seller {
			continue
		}
		if len(arguments) != len(rule.messageType.arguments) {
			continue
		}
		for _, argname := range rule.messageType.arguments {
			if _, ok := arguments[argname]; !ok {
				continue rules
			}
		}
		return &rule
	}
	return nil
}

func (tx *Transaction) findMatchingPhaseTransition(rule *phaseTransitionRule) *phaseTransition {
	for _, transition := range rule.phaseTransitions {
		if transition.prePhase == tx.Phase {
			return &transition
		}
	}
	return nil
}

func (tx *Transaction) Identify(address string) (from Origin) {
	switch address {
	case tx.Buyer:
		from = FromBuyer
	case tx.Seller:
		from = FromSeller
	default:
		from = FromUnknown
	}
	return
}

// Sends a message to the transaction and modifies it accordingly.
// Returns nil in case of success, an error otherwise.
// If an error is returned, the state of tx is undefined.
func (tx *Transaction) SendMessage(address string, arguments map[string]string) (result *Tmessage) {
	result = new(Tmessage)
	result.Accepted = false
	result.PrePhase = tx.Phase
	result.PostPhase = tx.Phase

	rule := tx.findMatchingRule(address, arguments)
	if rule == nil {
		result.RejectMessage = "Invalid message type"
		result.From = tx.Identify(address)
		return
	}

	result.From = rule.messageType.from

	transition := tx.findMatchingPhaseTransition(rule)
	if transition == nil {
		result.RejectMessage = "Invalid transaction phase"
		return
	}

	result.PostPhase = transition.postPhase
	if err := safeCall(rule.messageType.handler, tx, arguments); err != nil {
		result.RejectMessage = err.Error()
		return
	}

	result.Accepted = true
	tx.Phase = transition.postPhase
	tx.Revision += 1

	return
}

// Convert panic() into error
func safeCall(handler messageHandlerFunc, tx *Transaction, arguments map[string]string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Aborted: %v", r)
		}
	}()
	err = handler(tx, arguments)
	return
}

func mustMatch(r *regexp.Regexp, s string) {
	if !r.MatchString(s) {
		panic(fmt.Sprintf("String %#v doesn't match pattern %v", s, r))
	}
}

var workerUrlPattern = regexp.MustCompile(`^http://.*$`)

func mustParseWorkerUrl(s string) *string {
	mustMatch(workerUrlPattern, s)
	if _, err := url.ParseRequestURI(s); err != nil {
		panic(err)
	}

	return &s
}

var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func mustParseHash(s string) *Thash {
	mustMatch(hashPattern, s)
	result := new(Thash)
	hex.Decode(result[:], []byte(s))
	return result
}

func mustParseKey(s string) *Tkey {
	return (*Tkey)(mustParseHash(s))
}

func mustParseSignature(s string) *Tsignature {
	signature, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(fmt.Sprintf("Could not decode signature: %v", err))
	}
	if len(signature) != 64 {
		panic(fmt.Sprintf("Signature must be 64 bytes long. Received: %v bytes", len(signature)))
	}

	var result Tsignature
	copy(result[:], signature)

	return &result
}

func mustVerifyReceipt(receipt *Treceipt, address string) {
	err := bitcoin.VerifySignatureBase64(receipt.Hash.String(), address, receipt.HashSignature.String())
	if err != nil {
		panic(fmt.Errorf("Could not verify receipt: %v", err))
	}
}

func handleWorkerUrl(tx *Transaction, arguments map[string]string) error {
	tx.WorkerURL = mustParseWorkerUrl(arguments["workerurl"])
	return nil
}

func handleWorkHashes(tx *Transaction, arguments map[string]string) error {
	tx.WorkHash = mustParseHash(arguments["workhash"])
	tx.WorkSecretHash = mustParseHash(arguments["worksecrethash"])
	return nil
}

func handleBuyerSecret(tx *Transaction, arguments map[string]string) error {
	bsecret := mustParseHash(arguments["buyersecret"])
	sha := sha256.New()
	sha.Write(tx.WorkHash[:])
	sha.Write(bsecret[:])
	if !bytes.Equal(sha.Sum(make([]byte, 0, 32)), tx.WorkSecretHash[:]) {
		return fmt.Errorf("Published buyer's secret is inconsistent with WorkHash and WorkSecretHash")
	}
	tx.BuyerSecret = bsecret
	return nil
}

func handleTransmitFinished(tx *Transaction, arguments map[string]string) error {
	receipt := &Treceipt{
		Hash:          *mustParseHash(arguments["encresulthash"]),
		HashSignature: *mustParseSignature(arguments["encresulthashsig"]),
	}
	mustVerifyReceipt(receipt, tx.Buyer)
	tx.ResultDecryptionKey = mustParseKey(arguments["encresultkey"])
	tx.EncryptedResultReceipt = receipt
	return nil
}

func NewTransaction(now time.Time, newKey, oldKey string, newBid, oldBid *Bid) *Transaction {
	// sanity checks
	if oldBid.Type == newBid.Type || oldBid.Price.Currency != newBid.Price.Currency {
		panic("Non-matching bids")
	}

	tx := &Transaction{
		Price:   oldBid.Price,
		Article: oldBid.Article,
		Matched: now,
		Timeout: now.Add(300 * time.Second),
		State:   StateActive,
	}

	var buyBid, sellBid *Bid
	var buyKey, sellKey string
	if newBid.Type == Buy {
		buyBid = newBid
		buyKey = newKey
		sellBid = oldBid
		sellKey = oldKey
	} else {
		buyBid = oldBid
		buyKey = oldKey
		sellBid = newBid
		sellKey = newKey
	}

	tx.Price = oldBid.Price
	tx.Fee = money.Min(newBid.Fee, oldBid.Fee)
	tx.BuyerBid = buyKey
	tx.SellerBid = sellKey
	tx.Buyer = buyBid.Participant
	tx.Seller = sellBid.Participant

	newBid.Matched = &now
	newBid.State = Matched
	oldBid.Matched = &now
	oldBid.State = Matched

	return tx
}

func (tx *Transaction) Book(dao CachedAccountingDao, buyerBid *Bid) error {
	bidPrice := buyerBid.Price.Add(buyerBid.Fee)
	txPrice := tx.Price.Add(tx.Fee)
	delta := bidPrice.Sub(txPrice)
	if delta.Amount < 0 {
		return fmt.Errorf("Strange price delta bid->tx: %v", delta)
	} else if delta.Amount == 0 {
		return nil
	}

	return PlaceAccountMovement(dao, tx.Matched, AccountMovementTransaction,
		tx.Buyer, tx.Buyer,
		delta, delta.Neg(),
		money.Money{Currency: tx.Price.Currency, Amount: 0})
}

func (tx *Transaction) Retire(dao AccountingDao, now time.Time) error {
	if tx.State != StateActive {
		panic("Wrong transaction state")
	}
	if tx.Price.Currency != tx.Fee.Currency {
		panic("Inconsistent currencies")
	}

	var err error
	if tx.Phase == PhaseFinished {
		err = PlaceAccountMovement(dao, now, AccountMovementTransactionFinish,
			tx.Seller, tx.Buyer,
			tx.Price, tx.Price.Add(tx.Fee).Neg(),
			tx.Fee)
	} else {
		err = PlaceAccountMovement(dao, now, AccountMovementTransactionFinish,
			tx.Buyer, tx.Buyer,
			tx.Price.Add(tx.Fee), tx.Price.Add(tx.Fee).Neg(),
			money.Money{Currency: tx.Price.Currency, Amount: 0})
	}
	if err != nil {
		return err
	}

	tx.State = StateRetired
	tx.Revision++
	return nil
}
