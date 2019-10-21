// simple reverse proxy
// Copyright (C) 2017-2019  geosoft1  geosoft1@gmail.com
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var configFile = flag.String("conf", "conf.json", "configuration file")
var httpAddress = flag.String("http", ":8080", "http address")
var httpsAddress = flag.String("https", ":8090", "https address")
var httpsEnabled = flag.Bool("https-enabled", false, "enable https server")
var verbose = flag.Bool("verbose", true, "explain what is being done")

var config map[string]interface{}

// Register ...

func main() {
	flag.Usage = func() {
		fmt.Printf("usage: %s [options]\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// this returns a tmpdir if using `go run ...`
	folder, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatalln(err)
	}

	file, err := os.Open(filepath.Join(folder, *configFile))
	if err != nil {
		log.Printf("%s not found here, trying current working directory.", *configFile)
		// try looking in the current working directory
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalln(err)
		}
		file, err = os.Open(filepath.Join(wd, *configFile))
		if err != nil {
			log.Fatalln(err)
		}
	}

	if err := json.NewDecoder(file).Decode(&config); err != nil {
		log.Fatalln(err)
	}

	// for 'proxies', IN --> OUT
	for host, target := range config["proxies"].(map[string]interface{}) {
		log.Printf("%s ---> %s", host, target)
		if strings.HasPrefix(host, "#") {
			// skip comments
			continue
		}
		targetURL, err := url.Parse(target.(string))
		if err != nil {
			// skip invalid hosts
			log.Println(err)
			continue
		}
		if targetURL.Scheme == "https" && !*httpsEnabled {
			log.Println("https scheme detected but server is not enabled, run with -https-enabled")
			continue
		}

		// type URL struct {
		//     Scheme     string
		//     Opaque     string    // encoded opaque data
		//     User       *Userinfo // username and password information
		//     Host       string    // host or host:port
		//     Path       string    // path (relative paths may omit leading slash)
		//     RawPath    string    // encoded path hint (see EscapedPath method); added in Go 1.5
		//     ForceQuery bool      // append a query ('?') even if RawQuery is empty; added in Go 1.7
		//     RawQuery   string    // encoded query values, without '?'
		//     Fragment   string    // fragment for references, without '#'
		// }

		fmt.Printf("New proxy: %s --> %#v", host, targetURL)
		// log.Printf("New proxy: %s --> %+v", host, targetURL)
		proxy := httputil.NewSingleHostReverseProxy(
			&url.URL{
				Scheme: targetURL.Scheme,
				Host:   targetURL.Host,
			},
		)

		registered := func(p *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
			return func(w http.ResponseWriter, r *http.Request) {
				log.Printf("Request: %s%s", r.RemoteAddr, r.RequestURI)
				fmt.Println(" ")
				fmt.Printf("Request: %#v", r)
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Headers", "X-Requested-With")
				p.ServeHTTP(w, r)
			}
		}(proxy)

		http.HandleFunc(
			"/",
			// Register function
			registered,
		)

		if *httpsEnabled {
			go func() {
				// allow you to use self signed certificates
				http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
				// openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout server.key -out server.crt
				log.Printf("start https server on %s", *httpsAddress)
				if err := http.ListenAndServeTLS(*httpsAddress, filepath.Join(folder, "server.crt"), filepath.Join(folder, "server.key"), nil); err != nil {
					log.Fatalln(err)
				}
			}()
		}

		log.Printf("start http server on %s", *httpAddress)
		// just uses DefaultServeMux
		if err := http.ListenAndServe(*httpAddress, nil); err != nil {
			log.Fatalln(err)
		}
	}
}
