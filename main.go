package main

import (
	"flag"
	"log"
)

func main() {
	gitDir := flag.String("git-dir", "", "path to git directory (.git/ or a bare repo)")
	httpAddr := flag.String("http-addr", ":8080", "http address to serve on")
	sshAddr := flag.String("ssh-addr", ":8081", "ssh address to serve on")
	flag.Parse()

	errc := make(chan error, 2)
	go func() {
		errc <- runSSH(*gitDir, *sshAddr)
	}()
	go func() {
		errc <- runHTTP(*gitDir, *httpAddr)
	}()
	for i := 0; i < cap(errc); i++ {
		err := <-errc
		if err != nil {
			log.Println(err)
		}
	}
}
