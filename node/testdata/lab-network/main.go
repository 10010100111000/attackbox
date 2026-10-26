// Standalone, offline-built network acceptance fixture; never a production API.
package main

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "check":
			c, err := net.DialTimeout("tcp4", "127.0.0.1:8080", time.Second)
			if err != nil {
				os.Exit(1)
			}
			c.Close()
			return
		case "write":
			if len(os.Args) != 4 {
				os.Exit(2)
			}
			if err := os.WriteFile(os.Args[2], []byte(os.Args[3]), 0644); err != nil {
				os.Exit(1)
			}
			return
		case "read":
			if len(os.Args) != 3 {
				os.Exit(2)
			}
			b, err := os.ReadFile(os.Args[2])
			if err != nil {
				os.Exit(1)
			}
			os.Stdout.Write(b)
			return
		}
	}
	udp, err := net.ListenPacket("udp4", ":8081")
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		b := make([]byte, 64)
		for {
			n, a, err := udp.ReadFrom(b)
			if err != nil {
				return
			}
			udp.WriteTo(b[:n], a)
		}
	}()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "lab-fixture") })
	http.HandleFunc("/whoami", func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		json.NewEncoder(w).Encode(map[string]string{"source": ip})
	})
	http.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		network := r.URL.Query().Get("network")
		if network != "tcp" && network != "udp" {
			http.Error(w, "invalid protocol", 400)
			return
		}
		c, err := net.DialTimeout(network, r.URL.Query().Get("address"), time.Second)
		ok := err == nil
		source := ""
		if c != nil {
			defer c.Close()
			c.SetDeadline(time.Now().Add(time.Second))
			if network == "udp" {
				_, err = c.Write([]byte("ping"))
				if err == nil {
					b := make([]byte, 4)
					_, err = io.ReadFull(c, b)
				}
				ok = err == nil
			}
			if network == "tcp" && r.URL.Query().Get("source") == "1" {
				source, err = bufio.NewReader(c).ReadString('\n')
				ok = err == nil
				source = strings.TrimSpace(source)
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": ok, "source": source})
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
