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

package client

import (
	"bitwrk"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

type WorkerManager struct {
	mutex           sync.Mutex
	workers         map[string]*WorkerState
	activityManager *ActivityManager
	receiveManager  *ReceiveManager
}

type WorkerState struct {
	m            *WorkerManager
	cond         *sync.Cond
	LastError    string // set after each call to DoWork
	Info         WorkerInfo
	Idle         bool // set to false when a job is started, true when worker reports back
	Unregistered bool
	Blockers     int // count of currently blocking circumstances
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
	// By putting closeable objects into closerChan, the Worker implementation can submit
	// any connections or files used to communicate with the worker process to a watchdog
	// supervising the process.
	DoWork(workReader io.Reader, closerChan chan<- io.Closer) (io.ReadCloser, error)
}

func NewWorkerManager(a *ActivityManager, r *ReceiveManager) *WorkerManager {
	m := new(WorkerManager)
	m.workers = make(map[string]*WorkerState)
	m.activityManager = a
	m.receiveManager = r
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

func (m *WorkerManager) RegisterWorker(info WorkerInfo) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	log := bitwrk.Root().Newf("Worker %#v", info.Id)
	if s, ok := m.workers[info.Id]; ok {
		log.Printf("Reported idle: %v", info)
		s.cond.L.Lock()
		defer s.cond.L.Unlock()
		s.Info = info
		if !s.Idle {
			s.Idle = true
			s.Blockers--
		}
		s.cond.Broadcast()
	} else {
		log.Printf("Registered: %v", info)
		s = &WorkerState{
			m:    m,
			cond: sync.NewCond(new(sync.Mutex)),
			Info: info,
			Idle: true,
		}
		m.workers[info.Id] = s
		go s.offer(log)
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

func (s *WorkerState) offer(log bitwrk.Logger) {
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
			if sell, err := s.m.activityManager.NewSell(s); err != nil {
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
	} ()
	if err := sell.PerformSell(log.Newf("Sell #%v", sell.GetKey()), s.m.receiveManager, interrupt); err != nil {
		s.LastError = fmt.Sprintf("Error performing sell (delaying next sell by 20s): %v", err)
		log.Println(s.LastError)
		s.cond.L.Lock()
		s.blockFor(20 * time.Second)
		s.cond.L.Unlock()
	}
	log.Printf("returned from buy")
}

func (s *WorkerState) blockFor(d time.Duration) {
	s.Blockers++ // Unlocked after 20s
	s.cond.Broadcast()
	go func() {
		time.Sleep(20 * time.Second)
		s.cond.L.Lock()
		defer s.cond.L.Unlock()
		s.Blockers--
		s.cond.Broadcast()
	}()
}

func (s *WorkerState) setBusy() {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	if s.Idle {
		s.Idle = false
		s.Blockers++ // Wait for worker reporting back
	}
}

func (s *WorkerState) GetWorkerState() WorkerState {
	return *s
}

func (s *WorkerState) DoWork(workReader io.Reader, closerChan chan<- io.Closer) (io.ReadCloser, error) {
	// Mark worker as busy
	s.setBusy()

	// Customized HTTP client that submits all connections to watchdog
	var controlledClient http.Client = client
	controlledClient.Transport = &http.Transport{
		Dial: func(network, addr string) (conn net.Conn, err error) {
			conn, err = net.DialTimeout(network, addr, 10*time.Second)
			if err == nil {
				closerChan <- conn // Start watching connection
			}
			return
		},
	}

	// Do ectual HTTP request
	resp, err := controlledClient.Post(s.Info.PushURL, "application/octet-stream", workReader)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("Worker returned status: %v", resp.Status)
	}

	return resp.Body, nil
}
