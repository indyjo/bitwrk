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

package client

import (
	"crypto/rand"
	"fmt"
	"github.com/indyjo/bitwrk-common/bitcoin"
	"github.com/indyjo/bitwrk-common/bitwrk"
	. "github.com/indyjo/bitwrk-common/protocol"
	"github.com/indyjo/cafs"
	"io"
	"sync"
	"time"
)

type SellActivity struct {
	Trade

	worker Worker
}

func (m *ActivityManager) NewSell(worker Worker) (*SellActivity, error) {
	now := time.Now()

	result := &SellActivity{
		Trade: Trade{
			condition:    sync.NewCond(new(sync.Mutex)),
			manager:      m,
			key:          m.NewKey(),
			started:      now,
			lastUpdate:   now,
			bidType:      bitwrk.Sell,
			article:      worker.GetWorkerState().Info.Article,
			encResultKey: new(bitwrk.Tkey),
			alive:        true,
		},
		worker: worker,
	}

	// Get a random key for encrypting the result
	if _, err := rand.Reader.Read(result.encResultKey[:]); err != nil {
		return nil, err
	}

	m.register(result.key, result)
	return result, nil
}

// Manages the complete lifecycle of a sell
func (a *SellActivity) PerformSell(log bitwrk.Logger, receiveManager *ReceiveManager, interrupt <-chan bool) error {
	defer log.Println("Sell finished")
	err := a.doPerformSell(log, receiveManager, interrupt)
	if err != nil {
		a.lastError = err
	}
	a.alive = false
	return err
}

func (a *SellActivity) doPerformSell(log bitwrk.Logger, receiveManager *ReceiveManager, interrupt <-chan bool) error {
	if err := a.beginTrade(log, interrupt); err != nil {
		return err
	}

	// Start polling for state changes in background
	abortPolling := make(chan bool)
	defer func() {
		// Stop polling when sell has ended
		abortPolling <- true
	}()
	go func() {
		a.pollTransaction(log, abortPolling)
	}()

	info := fmt.Sprintf("Sell #%v", a.GetKey())
	receiver := NewWorkReceiver(log, info, receiveManager, a.manager.GetStorage(), *a.encResultKey, a)
	defer receiver.Dispose()

	// Announce receive URL
	if err := SendTxMessageEstablishSeller(a.txId, a.identity, receiver.URL()); err != nil {
		return err
	}

	// Wait through the whole transaction lifecycle
	a.waitForTransactionPhase(log, bitwrk.PhaseFinished,
		bitwrk.PhaseEstablishing,
		bitwrk.PhaseBuyerEstablished,
		bitwrk.PhaseSellerEstablished,
		bitwrk.PhaseTransmitting,
		bitwrk.PhaseWorking,
		bitwrk.PhaseUnverified)
	return nil
}

func (a *SellActivity) HandleWork(log bitwrk.Logger, workFile cafs.File, buyerSecret bitwrk.Thash) (io.ReadCloser, error) {
	// Wait for buyer to establish
	active := true
	var workHash, workSecretHash *bitwrk.Thash
	log.Printf("Watching transaction state...")
	a.waitWhile(func() bool {
		active = a.tx.State == bitwrk.StateActive
		workHash, workSecretHash = a.tx.WorkHash, a.tx.WorkSecretHash
		log.Printf("  state: %v    phase: %v", a.tx.State, a.tx.Phase)
		return active && a.tx.Phase != bitwrk.PhaseTransmitting
	})

	if !active {
		return nil, fmt.Errorf("Transaction timed out waiting for buyer to establish")
	}

	// Verify work hash
	if *workHash != bitwrk.Thash(workFile.Key()) {
		return nil, fmt.Errorf("WorkHash and received data do not match")
	}

	if err := verifyBuyerSecret(workHash, workSecretHash, &buyerSecret); err != nil {
		return nil, err
	}

	log.Println("Got valid work data. Publishing buyer's secret.")
	if err := SendTxMessagePublishBuyerSecret(a.txId, a.identity, &buyerSecret); err != nil {
		return nil, fmt.Errorf("Error publishing buyer's secret: %v", err)
	}

	log.Println("Starting to work...")
	r, err := a.dispatchWork(log, workFile)
	if err != nil {
		log.Printf("Rejecting work because of error '%v'", err)
		if err := SendTxMessageRejectWork(a.txId, a.identity); err != nil {
			log.Printf("Rejecting work failed: %v", err)
		}
	}
	return r, err
}

func (a *SellActivity) HandleReceipt(log bitwrk.Logger, encResultHash, encResultHashSig string) error {
	if err := bitcoin.VerifySignatureBase64(encResultHash, a.tx.Buyer, encResultHashSig); err != nil {
		return err
	}
	if err := SendTxMessageTransmitFinished(a.txId, a.identity, encResultHash, encResultHashSig, a.encResultKey.String()); err != nil {
		return fmt.Errorf("Signaling working finished failed: %v", err)
	}
	return nil
}

func (a *SellActivity) dispatchWork(log bitwrk.Logger, workFile cafs.File) (io.ReadCloser, error) {
	// Watch transaction state and close connection to worker when transaction expires
	connChan := make(chan io.Closer)
	exitChan := make(chan bool)
	go a.watchdog(log, exitChan, connChan, func() bool { return a.tx.State == bitwrk.StateActive })
	defer func() {
		exitChan <- true
	}()

	reader := workFile.Open()
	defer reader.Close()

	st := NewScopedTransport()
	connChan <- st
	defer st.Close()
	r, err := a.worker.DoWork(reader, NewClient(&st.Transport))
	if err == nil {
		// Defuse connection closing mechanism
		st.DisownConnections()
	}
	return r, err
}
