// SwapStudio desktop launcher: serves the bundled web app on localhost and opens it in the default browser.
// The program exits by itself a short while after the browser tab is closed.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync/atomic"
	"time"
)

//go:embed web
var webFS embed.FS

const heartbeatJS = `<script>setInterval(()=>fetch('/__ping').catch(()=>{}),3000);fetch('/__ping').catch(()=>{});</script>`

func openBrowser(url string) {
	switch runtime.GOOS {
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		exec.Command("open", url).Start()
	default:
		exec.Command("xdg-open", url).Start()
	}
}

func main() {
	sub, _ := fs.Sub(webFS, "web")
	var last atomic.Int64
	last.Store(time.Now().Unix())

	mux := http.NewServeMux()
	mux.HandleFunc("/__ping", func(w http.ResponseWriter, r *http.Request) {
		last.Store(time.Now().Unix())
		w.WriteHeader(204)
	})
	files := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			b, err := fs.ReadFile(sub, "index.html")
			if err != nil {
				http.Error(w, "missing index.html", 500)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(b)
			w.Write([]byte(heartbeatJS))
			return
		}
		files.ServeHTTP(w, r)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	url := fmt.Sprintf("http://%s/", ln.Addr().String())
	fmt.Println("SwapStudio is running at", url)
	go func() {
		time.Sleep(300 * time.Millisecond)
		openBrowser(url)
	}()
	// Quit when the page has stopped pinging for 20 seconds (tab or browser closed).
	go func() {
		for {
			time.Sleep(5 * time.Second)
			if time.Now().Unix()-last.Load() > 20 {
				ln.Close()
				return
			}
		}
	}()
	http.Serve(ln, mux)
}
