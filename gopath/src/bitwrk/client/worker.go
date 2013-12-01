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
	"log"
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
	stopOffering func()
}

type WorkerInfo struct {
	Id      string
	Article bitwrk.ArticleId
	Method  string
	PushURL string
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
	if s, ok := m.workers[info.Id]; ok {
		log.Printf("Changed worker %#v: %v -> %v", info.Id, s.Info, info)
		s.Info = info
	} else {
		log.Printf("Registered worker: %v", info)
		chStop := make(chan int, 1)
		s = &WorkerState{
			m:    m,
			Info: info,
			stopOffering: func() {
				// The value has no meaning
				chStop <- 0
			},
		}
		m.workers[info.Id] = s
		go s.keepOffering(chStop)
	}
}

func (m *WorkerManager) UnregisterWorker(id string) {
	m.mutex.Lock()
	s, ok := m.workers[id]
	delete(m.workers, id)
	m.mutex.Unlock()

	if ok {
		s.stopOffering()
	}
}

func (s *WorkerState) keepOffering(chStop <-chan int) {
	log.Printf("Start offering worker %#v", s.Info.Id)
	defer log.Printf("Stopped offering worker %#v", s.Info.Id)
	for {
		select {
		case _ = <-chStop:
			return
		default:
			s.offer()
		}
	}
}

func (s *WorkerState) offer() {
	log.Printf("Offering worker %#v", s.Info.Id)
	if sell, err := s.m.activityManager.NewSell(&s.Info); err != nil {
		log.Printf("Error creating sell: %v", err)
	} else {
		log.Printf("Performing sell")
		if err = sell.Perform(s.m.receiveManager); err != nil {
			log.Printf("Error performing sell: %v", err)
			time.Sleep(20 * time.Second)
		}
	}
}
