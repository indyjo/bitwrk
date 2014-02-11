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
	Info         WorkerInfo
	Idle         bool   // set to true when registering, false when a job is started
	LastError    string // set after each call to DoWork
	Unregistered bool
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
		log.Printf("Free again: %v", info)
		s.Info = info
		s.Idle = true
		go s.offer(log)
	} else {
		log.Printf("Registered: %v", info)
		s = &WorkerState{
			m:    m,
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
		s.Unregistered = true
		bitwrk.Root().Newf("Worker %#v", id).Printf("Unregistered: %v", s.Info)
	}
}

func (s *WorkerState) offer(log bitwrk.Logger) {
	for s.isIdle() {
		if lastError := s.getLastError(); lastError != "" {
			log.Printf("Delaying sell by 20s because of last error: %v", lastError)
			time.Sleep(20 * time.Second)
		}
		if !s.isRegistered() {
			break
		}
		log.Printf("Creating sell")
		s.setLastError(nil)
		if sell, err := s.m.activityManager.NewSell(s); err != nil {
			log.Printf("Error creating sell: %v", err)
		} else {
			if err = sell.PerformSell(log.Newf("Sell #%v", sell.GetKey()), s.m.receiveManager); err != nil {
				log.Printf("Error performing sell: %v", err)
				s.setLastError(err)
			}
		}
	}
	if !s.isRegistered() {
		log.Printf("No longer registered")
	}
	if !s.isIdle() {
		log.Printf("Waiting for worker to report back")
	}
}

func (s *WorkerState) isRegistered() bool {
	s.m.mutex.Lock()
	defer s.m.mutex.Unlock()
	return !s.Unregistered
}

func (s *WorkerState) isIdle() bool {
	s.m.mutex.Lock()
	defer s.m.mutex.Unlock()
	return s.Idle
}

func (s *WorkerState) setIdle(idle bool) {
	s.m.mutex.Lock()
	defer s.m.mutex.Unlock()
	s.Idle = idle
}

func (s *WorkerState) setLastError(err error) {
	s.m.mutex.Lock()
	defer s.m.mutex.Unlock()
	if err == nil {
		s.LastError = ""
	} else {
		s.LastError = err.Error()
	}
}

func (s *WorkerState) getLastError() string {
	s.m.mutex.Lock()
	defer s.m.mutex.Unlock()
	return s.LastError
}

func (s *WorkerState) GetWorkerState() WorkerState {
	return *s
}

func (s *WorkerState) DoWork(workReader io.Reader, closerChan chan<- io.Closer) (io.ReadCloser, error) {
	// Mark worker as busy
	s.setIdle(false)

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
