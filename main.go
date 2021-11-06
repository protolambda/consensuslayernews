package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// 1 MiB max page size
const maxSize = 1000*1000


func main() {
	port := flag.Int("port", 5000, "port")
	self := flag.String("self", "http://localhost:5000", "self domain")
	domain := flag.String("domain", "localhost", "self domain")
	flag.Parse()

	cl := &http.Client{
		Timeout:       15 * time.Second,
	}

	newsPath := fmt.Sprintf("%s/news/", *self)


	ensureConsensusLayer := func(v string) string {
		v = strings.ReplaceAll(v, "window.domain = 'hackmd.io'", fmt.Sprintf("window.domain = '%s'", *domain))
		v = strings.ReplaceAll(v, "window.urlpath = ''", fmt.Sprintf("window.urlpath = 'forward'"))
		v = strings.ReplaceAll(v, "eth2.news", "consensuslayer.news")
		v = strings.ReplaceAll(v, "Ethereum 2.0", "Ethereum Consensus Layer")
		v = strings.ReplaceAll(v, "Eth2", "Ethereum Consensus Layer")
		v = strings.ReplaceAll(v, "ETH2", "Eth CL")
		v = strings.ReplaceAll(v, "eth2", "eth CL")
		v = strings.ReplaceAll(v, "eth 2", "eth CL")
		v = strings.ReplaceAll(v, "Eth 2", "Eth CL")

		v = strings.ReplaceAll(v, "Ethereum 1.0", "Ethereum Execution Layer")
		v = strings.ReplaceAll(v, "Eth1", "Ethereum Execution Layer")
		v = strings.ReplaceAll(v, "ETH1", "Eth EL")
		v = strings.ReplaceAll(v, "eth1", "eth EL")
		v = strings.ReplaceAll(v, "eth 1", "eth EL")
		v = strings.ReplaceAll(v, "Eth 1", "Eth EL")

		v = strings.ReplaceAll(v, "https://hackmd.io/@benjaminion/wnie2_", newsPath)
		// fix broken links (hidden anyway)
		v = strings.ReplaceAll(v, "newineth CL", "newineth2")
		v = strings.ReplaceAll(v, "/eth CL_news/", "/eth2news/")
		return v
	}

	forwardResource := func (w http.ResponseWriter, r *http.Request) {
		if len(r.RequestURI) > 1000 {
			log.Println("URL too long")
			w.WriteHeader(400)
			return
		}
		log.Printf("Forwarding: %q", r.RequestURI)
		path := strings.TrimPrefix(r.URL.Path, "/forward/")
		resp, err := cl.Get(fmt.Sprintf("https://hackmd.io/%s", path))

		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("Failed to get news page"))
			log.Println(err)
			return
		}
		io.Copy(w, resp.Body)
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			log.Printf("unexpected status code: %d", resp.StatusCode)
			return
		}
	}

	newsPage := func (w http.ResponseWriter, r *http.Request) {
		if len(r.RequestURI) > 1000 {
			log.Println("URL too long")
			w.WriteHeader(400)
			return
		}
		log.Printf("Remapping: %q", r.RequestURI)

		vars := mux.Vars(r)
		var resp *http.Response
		var err error
		inner := false
		if newsID, ok := vars["newsid"]; ok {
			if len(newsID) > 100 {
				w.WriteHeader(500)
				return
			}
			resp, err = cl.Get(fmt.Sprintf("https://hackmd.io/@benjaminion/wnie2_%s", newsID))
			inner = true
		} else {
			resp, err = cl.Get("https://hackmd.io/@benjaminion/eth2_news")
		}

		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("Failed to get news page"))
			log.Println(err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			log.Printf("unexpected status code: %d", resp.StatusCode)
			return
		}


		dr := bufio.NewReader(io.LimitReader(resp.Body, maxSize))
		bodyBytes, err := ioutil.ReadAll(dr)
		if err != nil {
			log.Fatal(err)
		}
		bodyString := string(bodyBytes)
		bodyString = ensureConsensusLayer(bodyString)
		if inner {
			bodyString = strings.ReplaceAll(bodyString, "<body style=\"display:none;\">", `<body style="display:none;">
<div style="position:fixed;z-index:1000;bottom:10px;right:10px;padding:5px;background:white;">
<i><strong>Disclaimer</strong>: <a href="https://github.com/protolambda/consensuslayernews" target="_blank">this</a>
is a parody by <a href="https://twitter.com/protolambda" target="_blank">@protolambda</a>
of <a href="https://eth2.news" target="_blank">eth2.news</a>,<br/>
renaming "eth2" to "consensus-layer" and "eth1" to "execution-layer"</i></div>`)
		}

		w.WriteHeader(200)
		w.Write([]byte(bodyString))
	}

	router := mux.NewRouter()
	router.HandleFunc("/", newsPage).Methods("GET")
	router.PathPrefix("/forward/").HandlerFunc(forwardResource).Methods("GET")
	router.HandleFunc(`/news/{newsid}`, newsPage).Methods("GET")

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: router,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	log.Print("Server Started")

	<-done
	log.Print("Server Stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		// extra handling here
		cancel()
	}()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
	log.Print("Server Exited")
}
