package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"path"
)

func main() {
	port := *flag.Int("p", 80, "fileserver -p 80")
	workdir := *flag.String("d", "./", "fileserver -d ./")
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Path[1:]
		if r.URL.Path == "/" {
			file = "plugins.xml"
		}
		http.ServeFile(w, r, path.Join(workdir, file))
	})
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Printf("Start error: %s\n", err)
		return
	}
	log.Println("Stopped.")
}
