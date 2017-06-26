//  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
//  Copyright (C) 2013-2015  Jonas Eschenburg <jonas@bitwrk.net>
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
	"fmt"
	"github.com/indyjo/bitwrk-common/bitcoin"
	"github.com/indyjo/bitwrk-common/bitwrk"
	. "github.com/indyjo/bitwrk-common/protocol"
	"github.com/indyjo/cafs"
	"io"
)

type SellActivity struct {
	Trade

	worker Worker
}

// Manages the complete lifecycle of a sell
func (a *SellActivity) PerformSell(log bitwrk.Logger, receiveManager *ReceiveManager, interrupt <-chan bool) error {
	log.Printf("Sell started")
	defer log.Println("Sell finished")
	err := a.doPerformSell(log, receiveManager, interrupt)
	if err != nil {
		a.execSync(func() { a.lastError = err })
	}
	a.execSync(func() { a.alive = false })
	return err
}

// Waits for clearance and then performs either a local or a remote sell, depending on the decision taken.
func (a *SellActivity) doPerformSell(log bitwrk.Logger, receiveManager *ReceiveManager, interrupt <-chan bool) error {
	if err := a.awaitClearance(log, interrupt); err != nil {
		return err
	}

	if a.localMatch != nil {
		return a.doLocalSell(log, interrupt)
	} else {
		return a.doRemoteSell(log, receiveManager, interrupt)
	}
}

// Performs a local sell.
func (a *SellActivity) doLocalSell(log bitwrk.Logger, interrupt <-chan bool) error {
	// Directly get the work file from the local buy
	buy := a.localMatch
	var workFile cafs.File
	buy.interruptibleWaitWhile(interrupt, func() bool {
		// Initially, the work file is not set
		if buy.workFile == nil {
			return buy.alive
		} else {
			workFile = buy.workFile.Duplicate()
			return false
		}
	})

	if workFile == nil {
		return fmt.Errorf("Buy was no longer alive on start of local sell")
	}

	a.execSync(func() {
		a.workFile = workFile
	})

	reader := workFile.Open()
	defer reader.Close()

	st := NewScopedTransport()
	defer st.Close()
	if r, err := a.worker.DoWork(reader, NewClient(&st.Transport)); err != nil {
		return fmt.Errorf("Worker finished with error: %v", err)
	} else {
		info := fmt.Sprintf("Sell #%v", a.GetKey())
		temp := a.manager.GetStorage().Create(info)
		defer temp.Dispose()
		if _, err := io.Copy(temp, r); err != nil {
			temp.Close()
			return fmt.Errorf("Error reading work result: %v", err)
		}
		if err := temp.Close(); err != nil {
			return fmt.Errorf("Error closing temporary of result data: %v", err)
		}

		// Put result file into sell so that the buy can see it
		a.execSync(func() { a.resultFile = temp.File() })

		// Wait for the buy to accept it
		buy.interruptibleWaitWhile(interrupt, func() bool {
			return buy.alive && buy.resultFile == nil
		})

		return nil
	}
}

// Performs a remote sell once it has been cleared.
func (a *SellActivity) doRemoteSell(log bitwrk.Logger, receiveManager *ReceiveManager, interrupt <-chan bool) error {
	if err := a.beginRemoteTrade(log, interrupt); err != nil {
		return err
	}

	// Start polling for state changes in background
	abortPolling := make(chan bool)
	// Stop polling when sell has ended
	defer close(abortPolling)

	info := fmt.Sprintf("Sell #%v", a.GetKey())
	receiver := NewWorkReceiver(log, info, receiveManager, a.manager.GetStorage(), *a.encResultKey, a)
	go func() {
		a.pollTransaction(log, abortPolling)
		receiver.Dispose()
	}()

	// Announce receive URL
	if err := SendTxMessageEstablishSeller(a.txId, a.identity, receiver.URL()); err != nil {
		return err
	}

	// Wait while transaction is active and work is not finished
	var txState bitwrk.TxState
	var txPhase bitwrk.TxPhase
	a.waitWhile(func() bool {
		txPhase = a.tx.Phase
		txState = a.tx.State
		return txState == bitwrk.StateActive &&
			txPhase != bitwrk.PhaseUnverified
	})

	// Return an error if receiver had one
	var receiverErr error
	if disposed, err := receiver.IsDisposed(); disposed && err != nil {
		receiverErr = err
	}

	// Return an error if transaction ended unexpectedly
	var stateErr error
	if txState != bitwrk.StateActive && txPhase != bitwrk.PhaseUnverified && txPhase != bitwrk.PhaseFinished {
		stateErr = fmt.Errorf("Sell ended unexpectedly in phase %v", txPhase)
	}

	// Depending on failure mode, report the corresponding error
	if stateErr == nil && receiverErr == nil {
		return nil
	} else if receiverErr == nil {
		return stateErr
	} else if stateErr == nil {
		return receiverErr
	} else {
		return fmt.Errorf("%v. Additionally: %v", stateErr, receiverErr)
	}
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
