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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	client "github.com/indyjo/bitwrk-client"
	"github.com/indyjo/bitwrk-common/bitcoin"
	"github.com/indyjo/bitwrk-common/bitwrk"
	"github.com/indyjo/bitwrk-common/money"
	"github.com/indyjo/bitwrk-common/protocol"
	"github.com/indyjo/cafs"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"time"
)

var ExternalAddress string
var ExternalPort int
var InternalPort int
var InternalIface string
var BitcoinIdentity *bitcoin.KeyPair
var ResourceDir string
var BitwrkUrl string
var TrustedAccount string

func main() {
	protocol.BitwrkUserAgent = "BitWrkGoClient/" + ClientVersion
	flags := flag.NewFlagSet("bitwrk-client", flag.ExitOnError)
	flags.StringVar(&ExternalAddress, "extaddr", "auto",
		"IP address or name this host can be reached under from the internet")
	flags.IntVar(&ExternalPort, "extport", -1,
		"Port that can be reached from the Internet (-1 disables incoming connections)")
	flags.IntVar(&InternalPort, "intport", 8081, "Network port on which to listen for internal connections (UI and workers)")
	flags.StringVar(&InternalIface, "intiface", "127.0.0.1", "Network interface on which to listen for internal connections (UI and workers)")
	flags.StringVar(&ResourceDir, "resourcedir",
		"auto",
		"Directory where the bitwrk client loads resources from")
	flags.StringVar(&BitwrkUrl, "bitwrkurl", "http://bitwrk.appspot.com/",
		"URL to contact the bitwrk service at")
	flags.BoolVar(&cafs.LoggingEnabled, "log-cafs", cafs.LoggingEnabled,
		"Enable logging for content-addressable file storage")
	flags.IntVar(&client.NumUnmatchedBids, "num-unmatched-bids", client.NumUnmatchedBids,
		"Mamimum number of unmatched bids for an article on server")
	flags.StringVar(&TrustedAccount, "trusted-account", "1TrsjuCvBch1D9h6nRkadGKakv9KyaiP6",
		"Account to trust when verifying deposit information.")
	err := flags.Parse(os.Args[1:])
	if err == flag.ErrHelp {
		flags.Usage()
	} else if err != nil {
		log.Fatalf("Error parsing command line: %v", err)
	}

	if ResourceDir == "auto" {
		if dir, err := AutoFindResourceDir("bitwrk-client", ClientVersion); err != nil {
			log.Fatalf("Error finding resource directory: %v", err)
		} else {
			ResourceDir = dir
		}
	} else {
		if err := TestResourceDir(ResourceDir, "bitwrk-client", ClientVersion); err != nil {
			log.Fatalf("Directory [%v] is not a valid resource directory: %v", ResourceDir, err)
		}
	}

	BitcoinIdentity = LoadOrCreateIdentity("bitwrk-client", bitcoin.AddrVersionBitcoin)

	if !strings.HasSuffix(BitwrkUrl, "/") {
		BitwrkUrl = BitwrkUrl + "/"
	}

	log.Printf("Bitwrk URL: %v", BitwrkUrl)
	protocol.BitwrkUrl = BitwrkUrl

	log.Printf("Resource directory: %v\n", ResourceDir)
	initTemplates()

	receiveManager := startReceiveManager()

	log.Printf("Internal network interface for UI and workers: %v\n", InternalIface)
	log.Printf("Internal network port for UI and workers: %v\n", InternalPort)
	log.Printf("Own BitWrk account: %v\n", BitcoinIdentity.GetAddress())
	log.Printf("Trusted account: %v", TrustedAccount)

	// Create local-only worker manager if no external port has been specified
	workerManager := client.NewWorkerManager(client.GetActivityManager(), receiveManager, ExternalPort <= 0)

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
	}
}

func getReceiveManagerPrefix(addr string) (prefix string) {
	if strings.Contains(addr, ":") {
		addr = "[" + addr + "]"
	}
	prefix = fmt.Sprintf("http://%v:%v/", addr, ExternalPort)
	return
}

// Depending on whether an external port has been configured, starts listening for
// incoming connections on it.
func startReceiveManager() (receiveManager *client.ReceiveManager) {
	receiveManager = client.NewReceiveManager("")
	if ExternalPort <= 0 {
		log.Printf("External port is %v.", ExternalPort)
		log.Println("  -> No connections will be accepted from other hosts.")
		log.Println("  -> Workers can only accept local jobs.")
		return
	}

	actualExternalAddress := ""
	if ExternalAddress == "auto" {
		if addr, err := protocol.DetermineIpAddress(); err != nil {
			log.Fatalf("Error auto-determining IP address: %v", err)
		} else {
			actualExternalAddress = addr
		}

		// periodically update the address from now on
		ticker := time.NewTicker(15 * time.Minute)
		go func() {
			for {
				<-ticker.C
				if addr, err := protocol.DetermineIpAddress(); err != nil {
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

// A wrapper around http.Handler that denies access from non-loopback sources
type protector struct{ h http.Handler }

// Denies requests from prohibited addresses
func (p protector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if allowed(r.RemoteAddr) {
		p.h.ServeHTTP(w, r)
	} else {
		http.Error(w, "Access only allowed from loopback interface, not from "+r.RemoteAddr, http.StatusForbidden)
	}
}

// Deny access to everybody except localhost
func allowed(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

// Sets up handlers for requests on BitWrk's internal port
func serveInternal(workerManager *client.WorkerManager, exit chan<- error) {
	mux := http.NewServeMux()
	s := &http.Server{
		Addr:    fmt.Sprintf("%v:%v", InternalIface, InternalPort),
		Handler: mux,
		// No timeouts!
	}
	relay := NewHttpRelay("/", protocol.BitwrkUrl, protocol.NewClient(&http.Transport{}))

	// Some shortcuts for API declaration
	public := func(pattern string, handler http.Handler) {
		mux.Handle(pattern, handler)
	}
	protected := func(pattern string, handler http.Handler) {
		mux.Handle(pattern, protector{handler})
	}
	publicFunc := func(pattern string, handler func(http.ResponseWriter, *http.Request)) {
		public(pattern, http.HandlerFunc(handler))
	}
	protectedFunc := func(pattern string, handler func(http.ResponseWriter, *http.Request)) {
		protected(pattern, protector{http.HandlerFunc(handler)})
	}

	public("/account/", relay)
	public("/bid/", relay)
	public("/deposit/", relay)
	public("/tx/", relay)
	public("/motd", relay)

	accountFilter := func(data []byte) ([]byte, error) {
		var account bitwrk.ParticipantAccount
		if err := json.Unmarshal(data, json.Unmarshaler(&account)); err != nil {
			return data, nil
		} else {
			// Pass data about validation results into the template so it doesn't
			// have to be done in JavaScript.
			type result struct {
				Account             *bitwrk.ParticipantAccount
				Updated             time.Time
				TrustedAccount      string
				DepositAddress      string
				DepositAddressValid bool
			}
			r := result{&account, time.Now(), TrustedAccount, "", false}
			if v, err := url.ParseQuery(account.DepositInfo); err == nil {
				m := bitwrk.DepositAddressMessage{}
				m.FromValues(v)
				if m.VerifyWith(TrustedAccount) == nil {
					r.DepositAddress = m.DepositAddress
					r.DepositAddressValid = true
				}
			}
			return json.Marshal(r)
		}
	}
	myAccountUrl := fmt.Sprintf("%saccount/%s", protocol.BitwrkUrl, BitcoinIdentity.GetAddress())
	myAccountRelay := NewHttpRelay("/myaccount", myAccountUrl, relay.client).WithFilterFunc(accountFilter)
	public("/myaccount", myAccountRelay)

	resource := http.FileServer(http.Dir(filepath.Join(ResourceDir, "htroot")))
	public("/js/", resource)
	public("/css/", resource)
	public("/img/", resource)

	protectedFunc("/buy/", handleBuy)
	publicFunc("/file/", handleFile)
	protectedFunc("/", handleHome)
	protectedFunc("/ui/", handleHome)
	protectedFunc("/activities", handleActivities)
	publicFunc("/registerworker", func(w http.ResponseWriter, r *http.Request) {
		handleRegisterWorker(workerManager, w, r)
	})
	publicFunc("/unregisterworker", func(w http.ResponseWriter, r *http.Request) {
		handleUnregisterWorker(workerManager, w, r)
	})
	protectedFunc("/workers", func(w http.ResponseWriter, r *http.Request) {
		handleWorkers(workerManager, w, r)
	})
	protectedFunc("/mandates", func(w http.ResponseWriter, r *http.Request) {
		handleMandates(client.GetActivityManager(), w, r)
	})
	protectedFunc("/revokemandate", func(w http.ResponseWriter, r *http.Request) {
		if err := handleRevokeMandate(r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	protectedFunc("/requestdepositaddress", func(w http.ResponseWriter, r *http.Request) {
		if err := handleRequestDepositAddress(r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			// Account information is now stale
			myAccountRelay.InvalidateCache()
		}
	})
	publicFunc("/id", handleId)
	publicFunc("/version", handleVersion)
	publicFunc("/myip", handleMyIp)
	protectedFunc("/cafsdebug", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		client.GetActivityManager().GetStorage().DumpStatistics(cafs.NewWriterPrinter(w))
	})
	protectedFunc("/stackdump", func(w http.ResponseWriter, r *http.Request) {
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
		f.Dispose()
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
	article := bitwrk.ArticleId(r.URL.Path[5:])

	log.Printf("Handling buy for %#v from %v", article, r.RemoteAddr)

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var price *money.Money
	if priceStr := r.URL.Query().Get("price"); priceStr == "" {
		// No price given, ok
	} else if m, err := money.Parse(priceStr); err != nil {
		// Parsing failed
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		// Good case
		price = &m
	}

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

	var buy *client.BuyActivity
	if _buy, err := client.GetActivityManager().NewBuy(article, BitcoinIdentity, price); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error creating buy activity: %v", err)
		return
	} else {
		buy = _buy
	}
	defer buy.Dispose()

	log := bitwrk.Root().Newf("Buy #%v", buy.GetKey())

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

	workerManager.RegisterWorker(info, BitcoinIdentity)
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

func handleMyIp(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	addr := r.RemoteAddr
	w.Write([]byte(addr))
}
