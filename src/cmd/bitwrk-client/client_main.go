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
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"runtime/pprof"
	"strings"
	"time"
)

var ExternalAddress string
var ExternalPort int
var InternalPort int
var BitcoinIdentity *bitcoin.KeyPair
var ResourceDir string
var BitwrkUrl string

func main() {
	client.BitwrkUserAgent = "BitWrkGoClient/" + ClientVersion
	flags := flag.NewFlagSet("bitwrk-client", flag.ExitOnError)
	flags.StringVar(&ExternalAddress, "extaddr", "auto",
		"IP address or name this host can be reached under from the internet")
	flags.IntVar(&ExternalPort, "extport", -1,
		"Port that can be reached from the Internet (-1 disables incoming connections)")
	flags.IntVar(&InternalPort, "intport", 8081, "Maintenance port for admin interface")
	flags.StringVar(&ResourceDir, "resourcedir",
		"auto",
		"Directory where the bitwrk client loads resources from")
	flags.StringVar(&BitwrkUrl, "bitwrkurl", "http://bitwrk.appspot.com/",
		"URL to contact the bitwrk service at")
	flags.BoolVar(&cafs.LoggingEnabled, "log-cafs", cafs.LoggingEnabled,
		"Enable logging for content-addressable file storage")
	flags.IntVar(&client.NumUnmatchedBids, "num-unmatched-bids", client.NumUnmatchedBids,
		"Mamimum number of unmatched bids for an article on server")
	err := flags.Parse(os.Args[1:])
	if err == flag.ErrHelp {
		flags.Usage()
	} else if err != nil {
		log.Fatalf("Error parsing command line: %v", err)
		os.Exit(1)
	}

	if ResourceDir == "auto" {
		if dir, err := AutoFindResourceDir("bitwrk-client", "0.0.1"); err != nil {
			log.Fatalf("Error finding resource directory: %v", err)
		} else {
			ResourceDir = dir
		}
	} else {
		if err := TestResourceDir(ResourceDir, "bitwrk-client", "0.0.1"); err != nil {
			log.Fatalf("Directory [%v] is not a valid resource directory: %v", err)
		}
	}

	BitcoinIdentity = LoadOrCreateIdentity("bitwrk-client", bitcoin.AddrVersionBitcoin)

	if !strings.HasSuffix(BitwrkUrl, "/") {
		BitwrkUrl = BitwrkUrl + "/"
	}

	log.Printf("Bitwrk URL: %v", BitwrkUrl)
	client.BitwrkUrl = BitwrkUrl

	log.Printf("Resource directory: %v\n", ResourceDir)
	initTemplates()

	receiveManager := startReceiveManager()

	log.Printf("Internal port: %v\n", InternalPort)
	log.Printf("Bitcoin address: %v\n", BitcoinIdentity.GetAddress())

	workerManager := client.NewWorkerManager(client.GetActivityManager(), receiveManager)

	exit := make(chan error)
	if InternalPort > 0 {
		go serveInternal(workerManager, exit)
	}

	if ExternalPort > 0 {
		go serveExternal(receiveManager, exit)
	}

	err = <-exit
	if err != nil {
		log.Fatalf("Exiting because of: %v", err)
		os.Exit(1)
	}
}

func getReceiveManagerPrefix(addr string) (prefix string) {
	if strings.Contains(addr, ":") {
		addr = "[" + addr + "]"
	}
	prefix = fmt.Sprintf("http://%v:%v/", addr, ExternalPort)
	return
}

func startReceiveManager() (receiveManager *client.ReceiveManager) {
	receiveManager = client.NewReceiveManager("")
	if ExternalPort <= 0 {
		log.Printf("External port is %v. No connections will be accepted from other hosts.", ExternalPort)
		log.Printf("Only buys can be performed.")
		return
	}

	actualExternalAddress := ""
	if ExternalAddress == "auto" {
		if addr, err := client.DetermineIpAddress(); err != nil {
			log.Fatalf("Error auto-determining IP address: %v", err)
			os.Exit(1)
		} else {
			actualExternalAddress = addr
		}

		// periodically update the address from now on
		ticker := time.NewTicker(15 * time.Minute)
		go func() {
			for {
				<-ticker.C
				if addr, err := client.DetermineIpAddress(); err != nil {
					log.Printf("Ignoring failure to determine IP address: %v", err)
				} else if addr == actualExternalAddress {
					log.Printf("Client IP address still %v", addr)
				} else {
					log.Printf("New IP address: %v", addr)
					receiveManager.SetUrlPrefix(getReceiveManagerPrefix(addr))
					actualExternalAddress = addr
				}
			}
		}()
	} else {
		actualExternalAddress = ExternalAddress
	}

	receiveManager.SetUrlPrefix(getReceiveManagerPrefix(actualExternalAddress))
	log.Printf("External address: %v\n", actualExternalAddress)
	log.Printf("External port: %v\n", ExternalPort)

	return
}

func serveInternal(workerManager *client.WorkerManager, exit chan<- error) {
	mux := http.NewServeMux()
	s := &http.Server{
		Addr:    fmt.Sprintf("localhost:%v", InternalPort),
		Handler: mux,
		// No timeouts!
	}
	relay := &HttpRelay{"/", client.BitwrkUrl, client.NewClient(&http.Transport{})}
	mux.Handle("/account/", relay)
	mux.Handle("/bid", relay)
	mux.Handle("/bid/", relay)
	mux.Handle("/tx/", relay)
	mux.Handle("/motd", relay)

	resource := http.FileServer(http.Dir(path.Join(ResourceDir, "htroot")))
	mux.Handle("/js/", resource)
	mux.Handle("/css/", resource)
	mux.Handle("/img/", resource)

	mux.HandleFunc("/buy/", handleBuy)
	mux.HandleFunc("/file/", handleFile)
	mux.HandleFunc("/", handleHome)
	mux.HandleFunc("/activities", handleActivities)
	if ExternalPort > 0 {
		mux.HandleFunc("/registerworker", func(w http.ResponseWriter, r *http.Request) {
			handleRegisterWorker(workerManager, w, r)
		})
	} else {
		mux.HandleFunc("/registerworker", http.NotFound)
	}
	mux.HandleFunc("/unregisterworker", func(w http.ResponseWriter, r *http.Request) {
		handleUnregisterWorker(workerManager, w, r)
	})
	mux.HandleFunc("/workers", func(w http.ResponseWriter, r *http.Request) {
		handleWorkers(workerManager, w, r)
	})
	mux.HandleFunc("/mandates", func(w http.ResponseWriter, r *http.Request) {
		handleMandates(client.GetActivityManager(), w, r)
	})
	mux.HandleFunc("/revokemandate", func(w http.ResponseWriter, r *http.Request) {
		if err := handleRevokeMandate(r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/id", handleId)
	mux.HandleFunc("/version", handleVersion)
	mux.HandleFunc("/cafsdebug", func(w http.ResponseWriter, r *http.Request) {
		client.GetActivityManager().GetStorage().DumpStatistics(cafs.NewWriterPrinter(w))
	})
	mux.HandleFunc("/stackdump", func(w http.ResponseWriter, r *http.Request) {
		name := r.FormValue("name")
		if len(name) == 0 {
			name = "goroutine"
		}
		profile := pprof.Lookup(name)
		if profile == nil {
			w.Write([]byte("No such profile"))
			return
		}
		err := profile.WriteTo(w, 1)
		if err != nil {
			log.Printf("Error in profile.WriteTo: %v\n", err)
		}
	})
	exit <- s.ListenAndServe()
}

func serveExternal(receiveManager *client.ReceiveManager, exit chan<- error) {
	mux := http.NewServeMux()
	s := &http.Server{
		Addr:         fmt.Sprintf(":%v", ExternalPort),
		Handler:      mux,
		ReadTimeout:  900 * time.Second,
		WriteTimeout: 900 * time.Second,
	}

	mux.Handle("/", receiveManager)

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

	log.Printf("Handling buy for %#v from %v", article, r.RemoteAddr)

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var buy *client.BuyActivity
	if _buy, err := client.GetActivityManager().NewBuy(bitwrk.ArticleId(article)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error creating buy activity: %v", err)
		return
	} else {
		buy = _buy
	}
	defer buy.Dispose()

	log := bitwrk.Root().Newf("Buy #%v", buy.GetKey())

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
					break
				} else {
					log.Printf("Skipping form part %v", part)
				}
			}
		}
	}

	workTemp := client.GetActivityManager().GetStorage().Create(fmt.Sprintf("buy #%v: work", buy.GetKey()))
	defer workTemp.Dispose()
	if _, err := io.Copy(workTemp, reader); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error receiving work data from client: %v", err)
		return
	} else {
		if err := workTemp.Close(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("Error writing work data to storage: %v", err)
			return
		}
	}

	// Listen for close notfications
	interrupt := w.(http.CloseNotifier).CloseNotify()

	workFile := workTemp.File()
	defer workFile.Dispose()
	var result cafs.File
	if res, err := buy.PerformBuy(log, interrupt, workFile); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error receiving result from BitWrk network: %v", err)
		return
	} else {
		result = res
	}

	http.Redirect(w, r, "/file/"+result.Key().String(), http.StatusSeeOther)
}

var registerWorkerTemplate = template.Must(template.New("registerWorker").Parse(`
<!doctype html>
<html>
<head>
<title>Register Worker</title>
<body>
<form method="POST">
<input type="text" name="id" value="{{if .Id}}{{.Id}}{{else}}worker-1{{end}}" /> Worker's ID<br/>
<input type="text" name="article" value="{{if .Article}}{{.Article}}{{else}}foobar{{end}}" /> Worker's article<br/>
<input type="text" name="pushurl" value="{{if .PushURL}}{{.PushURL}}{{else}}http://localhost:1234/{{end}}" /> URL the worker accepts work on<br/>
<input type="submit" />
</form>
</body>
</html>
`))

func handleRegisterWorker(workerManager *client.WorkerManager, w http.ResponseWriter, r *http.Request) {
	info := client.WorkerInfo{
		Id:      r.FormValue("id"),
		Article: bitwrk.ArticleId(r.FormValue("article")),
		Method:  "http-push",
		PushURL: r.FormValue("pushurl"),
	}

	if r.Method != "POST" || info.Id == "" || info.PushURL == "" {
		registerWorkerTemplate.Execute(w, info)
	}

	workerManager.RegisterWorker(info)
}

func handleUnregisterWorker(workerManager *client.WorkerManager, w http.ResponseWriter, r *http.Request) {
	workerManager.UnregisterWorker(r.FormValue("id"))
}

func handleId(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("BitWrk Go Client"))
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(ClientVersion))
}
