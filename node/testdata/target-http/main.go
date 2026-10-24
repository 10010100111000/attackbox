// A local acceptance fixture, never an application target template.
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "probe" {
		client := &http.Client{Timeout: time.Second}
		resp, err := client.Get("http://127.0.0.1:8080/")
		if err != nil {
			os.Exit(1)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, "attackbox-target-smoke") })
	if err := http.ListenAndServe(":8080", nil); err != nil {
		os.Exit(1)
	}
}
