package main

import (
	"compress/gzip"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/pktline"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
)

func RunHTTP(dir, addr string) error {
	log.Printf("Starting HTTP server for dir '%s' on addr '%s'\n", dir, addr)

	http.HandleFunc("/info/refs", httpInfoRefs(dir))
	http.HandleFunc("/git-upload-pack", httpGitUploadPack(dir))

	err := http.ListenAndServe(addr, nil)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("Error during ListenAndServe: %v\n", err)
			log.Printf("HTTP server failed to start on addr '%s'\n", addr)
		return err
	}
	log.Println("HTTP server stopped")
	return nil
}

func httpInfoRefs(dir string) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("service") != "git-upload-pack" {
			http.Error(rw, "only smart git", http.StatusForbidden)
			log.Printf("Request to /info/refs with invalid service: %s\n", r.URL.Query().Get("service"))
			return
		}

		rw.Header().Set("content-type", "application/x-git-upload-pack-advertisement")

		ep, err := transport.NewEndpoint("/")
		if err != nil {
			log.Printf("Error creating endpoint: %v\n", err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		bfs := osfs.New(dir)
		ld := server.NewFilesystemLoader(bfs)
		svr := server.NewServer(ld)
		sess, err := svr.NewUploadPackSession(ep, nil)
		if err != nil {
			log.Printf("Error creating upload pack session: %v\n", err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		ar, err := sess.AdvertisedReferencesContext(r.Context())
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			log.Printf("Error getting advertised references: %v\n", err)
			return
		}

		ar.Prefix = [][]byte{
			[]byte("# service=git-upload-pack"),
			pktline.Flush,
		}
		err = ar.Encode(rw)
		if err != nil {
			log.Printf("Error encoding advertised references: %v\n", err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func httpGitUploadPack(dir string) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("content-type", "application/x-git-upload-pack-result")

		var bodyReader io.Reader = r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			gzipReader, err := gzip.NewReader(r.Body)
			if err != nil {
				log.Printf("Error creating gzip reader: %v\n", err)
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
			defer gzipReader.Close()
			bodyReader = gzipReader
		}

		upr := packp.NewUploadPackRequest()
		err := upr.Decode(bodyReader)
		if err != nil {
			log.Printf("Error decoding upload pack request: %v\n", err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		ep, err := transport.NewEndpoint("/")
		if err != nil {
			log.Printf("Error creating endpoint: %v\n", err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		bfs := osfs.New(dir)
		ld := server.NewFilesystemLoader(bfs)
		svr := server.NewServer(ld)
		sess, err := svr.NewUploadPackSession(ep, nil)
		if err != nil {
			log.Printf("Error creating upload pack session: %v\n", err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		res, err := sess.UploadPack(r.Context(), upr)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			log.Printf("Error during upload pack: %v\n", err)
			return
		}

		err = res.Encode(rw)
		if err != nil {
			log.Printf("Error encoding upload pack result: %v\n", err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
