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

package main

import (
	"bitwrk"
	"bitwrk/bitcoin"
	"bitwrk/cafs"
	"bitwrk/client"
	"bitwrk/money"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

var ExternalAddress string
var ExternalPort int
var InternalPort int
var BitcoinPrivateKeyEncoded string
var BitcoinIdentity *bitcoin.KeyPair

func main() {
	flags := flag.NewFlagSet("", flag.ExitOnError)
	flags.StringVar(&ExternalAddress, "extaddr", "auto",
		"IP address or name this host can be reached under from the internet")
	flags.IntVar(&ExternalPort, "extport", -1, "Port that can be reached from the Internet")
	flags.IntVar(&InternalPort, "intport", 8081, "Maintenance port for admin interface")
	flags.StringVar(&BitcoinPrivateKeyEncoded, "bitcoinprivkey",
		"random",
		"The private key of the Bitcoin address to use for authentication")
	err := flags.Parse(os.Args[1:])
	if err == flag.ErrHelp {
		flags.Usage()
	} else if err != nil {
		log.Fatalf("Error parsing command line: %v", err)
		os.Exit(1)
	}

	if BitcoinPrivateKeyEncoded == "random" {
		data := make([]byte, 32)
		if _, err := rand.Reader.Read(data); err != nil {
			log.Fatalf("Error generating random key: %v", err)
			os.Exit(1)
		}
		if key, err := bitcoin.FromPrivateKeyRaw(data, true, bitcoin.AddrVersionBitcoin); err != nil {
			log.Fatalf("Error creating key: %v", err)
			os.Exit(1)
		} else {
			BitcoinIdentity = key
		}
	} else {
		if key, err := bitcoin.FromPrivateKeyWIF(BitcoinPrivateKeyEncoded, bitcoin.AddrVersionBitcoin); err != nil {
			log.Fatalf("Error creating key: %v", err)
			os.Exit(1)
		} else {
			BitcoinIdentity = key
		}
	}

	if ExternalAddress == "auto" {
		ExternalAddress, err = client.DetermineIpAddress()
		if err != nil {
			log.Fatalf("Error auto-determining IP address: %v", err)
			os.Exit(1)
		}
	}

	log.Printf("External address: %v\n", ExternalAddress)
	log.Printf("External port: %v\n", ExternalPort)
	log.Printf("Internal port: %v\n", InternalPort)
	log.Printf("Bitcoin address: %v\n", BitcoinIdentity.GetAddress())

	exit := make(chan error)
	if InternalPort > 0 {
		go serveInternal(exit)
	}

	if ExternalPort > 0 {
		go serveExternal(exit)
	}

	//client.GetActivityManager().NewBuy("foobar")
	//TestPlaceBid()
	//os.Exit(0)

	err = <-exit
	if err != nil {
		log.Fatalf("Exiting because of: %v", err)
		os.Exit(1)
	}
}

func serveInternal(exit chan<- error) {
	mux := http.NewServeMux()
	s := &http.Server{
		Addr:         fmt.Sprintf("localhost:%v", InternalPort),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	mux.HandleFunc("/buy/", handleBuy)
	mux.HandleFunc("/file/", handleFile)
	exit <- s.ListenAndServe()
}

func serveExternal(exit chan<- error) {
	mux := http.NewServeMux()
	s := &http.Server{
		Addr:         fmt.Sprintf(":%v", ExternalPort),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	exit <- s.ListenAndServe()
}

func handleFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var key *cafs.SKey
	if k, err := cafs.ParseKey(r.URL.Path[6:]); err != nil {
		http.NotFound(w, r)
		log.Printf("Error parsing key from URL %v: %v", r.URL, err)
		return
	} else {
		key = k
	}

	var reader io.ReadCloser
	if f, err := client.GetActivityManager().GetStorage().Get(key); err != nil {
		http.NotFound(w, r)
		log.Printf("Error retrieving key %v: %v", key, err)
		return
	} else {
		reader = f.Open()
	}
	defer func() {
		if err := reader.Close(); err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}()

	if _, err := io.Copy(w, reader); err != nil {
		log.Printf("Error sending file contents to client: %v", err)
	}
}

func handleBuy(w http.ResponseWriter, r *http.Request) {
	article := r.URL.Path[5:]

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var buy *client.BuyActivity
	if _buy, err := client.GetActivityManager().NewBuy(article); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error creating buy activity: %v", err)
		return
	} else {
		buy = _buy
	}
	defer buy.End()

	workWriter := buy.WorkWriter()

	var reader io.Reader
	if multipart, err := r.MultipartReader(); err != nil {
	    // read directly from body
	    reader = r.Body
	} else {
	    // Iterate through parts of multipart body, find the one called "data"
	    for {
	        if part, err := multipart.NextPart(); err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                log.Printf("Error iterating through multipart content: %v", err)
                return
            } else {
                if part.FormName() == "data" {
                    reader = part
                    break;
                }
            }
        }
    }
    
	if _, err := io.Copy(workWriter, reader); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error receiving work data from client: %v", err)
		return
	} else {
		workWriter.Close()
	}

	var result cafs.File
	if res, err := buy.GetResult(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error receiving result from BitWrk network: %v", err)
		return
	} else {
		result = res
	}

	http.Redirect(w, r, "/file/"+result.Key().String(), http.StatusSeeOther)
}

func TestPlaceBid() {
	rawBid := &bitwrk.RawBid{
		Type:    bitwrk.Buy,
		Price:   money.MustParse("BTC 0.001"),
		Article: "foobar",
	}
	randombyte := make([]byte, 1)
	rand.Read(randombyte)
	if randombyte[0] > 127 {
		rawBid.Type = bitwrk.Sell
	}
	bidId, err := client.PlaceBid(rawBid, BitcoinIdentity)
	if err != nil {
		log.Fatalf("Place Bid failed: %v", err)
		return
	}
	log.Printf("bidId: %v", bidId)

	etag := ""
	for i := 0; i < 30; i++ {
		resp, err := client.GetJsonFromServer("bid/"+bidId, etag)
		if err != nil {
			panic(err)
		}
		log.Printf("Response: %v", resp)
		if resp.StatusCode == http.StatusOK {
			etag = resp.Header.Get("ETag")
		} else if resp.StatusCode == http.StatusNotModified {
			log.Printf("  Cache hit")
		} else {
			log.Fatalf("UNEXPECTED STATUS CODE %v", resp.StatusCode)
		}
		time.Sleep(5 * time.Second)
	}
}
