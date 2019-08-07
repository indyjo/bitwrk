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

package client

import (
	"fmt"
	"github.com/indyjo/bitwrk-common/bitcoin"
	"github.com/indyjo/bitwrk-common/bitwrk"
	"io"
	"net/http"
	"sync"
	"time"
)

type WorkerManager struct {
	mutex           sync.Mutex
	workers         map[string]*WorkerState
	activityManager *ActivityManager
	receiveManager  *ReceiveManager
	localOnly       bool // Whether workers are restricted to local jobs
}

type WorkerState struct {
	m            *WorkerManager
	cond         *sync.Cond
	LastError    string // set after each call to DoWork
	Info         WorkerInfo
	Idle         bool // set to false when a job is started, true when worker reports back
	Unregistered bool
	Blockers     int              // count of currently blocking circumstances
	identity     *bitcoin.KeyPair // BitWrk identity this worker is associated with
}

type WorkerInfo struct {
	Id      string
	Article bitwrk.ArticleId
	Method  string
	PushURL string
}

// The interface given to ActivityManager.NewSell() for controlling a worker without knowing
// About the exact cient<->worker protocol.
type Worker interface {
	// Returns the current state of a worker
	GetWorkerState() WorkerState
	// Makes the worker perform some work. Returns an io.ReadCloser containing the result,
	// or an error if anything went wrong. The caller is responsible for closing the result.
	DoWork(workReader io.Reader, client *http.Client) (io.ReadCloser, error)
}

func NewWorkerManager(a *ActivityManager, r *ReceiveManager, localOnly bool) *WorkerManager {
	m := new(WorkerManager)
	m.workers = make(map[string]*WorkerState)
	m.activityManager = a
	m.receiveManager = r
	m.localOnly = localOnly
	return m
}

func (m *WorkerManager) ListWorkers() (result []WorkerState) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	result = make([]WorkerState, 0, len(m.workers))
	for _, workerState := range m.workers {
		result = append(result, *workerState)
	}
	return
}

func (m *WorkerManager) RegisterWorker(info WorkerInfo, identity *bitcoin.KeyPair) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	log := bitwrk.Root().Newf("Worker %#v", info.Id)
	if s, ok := m.workers[info.Id]; ok {
		log.Printf("Reported idle: %v", info)
		s.setIdle(true)
	} else {
		log.Printf("Registered: %v", info)
		s = &WorkerState{
			m:        m,
			cond:     sync.NewCond(new(sync.Mutex)),
			Info:     info,
			Idle:     true,
			identity: identity,
		}
		m.workers[info.Id] = s
		go s.offer(log, m.localOnly)
	}
}

func (m *WorkerManager) UnregisterWorker(id string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if s, ok := m.workers[id]; ok {
		delete(m.workers, id)
		s.cond.L.Lock()
		defer s.cond.L.Unlock()
		s.Unregistered = true
		s.cond.Broadcast()
		bitwrk.Root().Newf("Worker %#v", id).Printf("Unregistered: %v", s.Info)
	}
}

func (s *WorkerState) offer(log bitwrk.Logger, localOnly bool) {
	defer log.Printf("Stopped offering")
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	interrupt := make(chan bool, 1)
	for {
		// Interrupt if unregistered, stop iterating
		if s.Unregistered {
			log.Printf("No longer registered")
			interrupt <- true
			break
		}
		if s.Blockers == 0 {
			s.LastError = ""
			if sell, err := s.m.activityManager.NewSell(s, s.identity, localOnly); err != nil {
				s.LastError = fmt.Sprintf("Error creating sell: %v", err)
				log.Println(s.LastError)
				s.blockFor(20 * time.Second)
			} else {
				s.Blockers++
				go s.executeSell(log, sell, interrupt)
			}
		}
		s.cond.Wait()
	}
}

func (s *WorkerState) executeSell(log bitwrk.Logger, sell *SellActivity, interrupt <-chan bool) {
	defer func() {
		s.cond.L.Lock()
		s.Blockers--
		s.cond.Broadcast()
		s.cond.L.Unlock()
	}()
	defer sell.Dispose()
	if err := sell.PerformSell(log.Newf("Sell #%v", sell.GetKey()), s.m.receiveManager, interrupt); err != nil {
		s.LastError = fmt.Sprintf("Error performing sell (delaying next sell by 20s): %v", err)
		log.Println(s.LastError)
		s.cond.L.Lock()
		s.blockFor(20 * time.Second)
		s.cond.L.Unlock()
	} else {
		log.Printf("Returned from PerformSell successfully")
	}
}

// Increases blockers count and starts a timer that decreases it again after the specified duration.
// Assumes that the mutex is held at the time of the call.
func (s *WorkerState) blockFor(d time.Duration) {
	s.Blockers++ // Unlocked after N seconds
	s.cond.Broadcast()
	go func() {
		time.Sleep(d)
		s.cond.L.Lock()
		defer s.cond.L.Unlock()
		s.Blockers--
		s.cond.Broadcast()
	}()
}

// As long as a worker is marked as busy (not idle), no attempt is made to sell with it.
func (s *WorkerState) setIdle(idle bool) {
	s.cond.L.Lock()
	s.cond.L.Unlock()
	if s.Idle != idle {
		s.Idle = idle
		if idle {
			s.Blockers--
		} else {
			s.Blockers++
		}
		s.cond.Broadcast()
	}
}

func (s *WorkerState) GetWorkerState() WorkerState {
	return *s
}

func (s *WorkerState) DoWork(workReader io.Reader, client *http.Client) (io.ReadCloser, error) {
	// Mark worker as busy until it reports back.
	s.setIdle(false)

	// Do ectual HTTP request
	resp, err := client.Post(s.Info.PushURL, "application/octet-stream", workReader)
	if err != nil {
		// There is no guarantee that the worker will report back, so we need to assume it is idle
		s.setIdle(true)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		// TODO: Need a way for the worker to signal it will report back after an error
		s.setIdle(true)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("Worker returned status: %v", resp.Status)
	}

	return resp.Body, nil
}
