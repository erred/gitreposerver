package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"log"
	"net"

	"github.com/anmitsu/go-shlex"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"golang.org/x/crypto/ssh"
)

func runSSH(dir, addr string) error {
	config := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	_, edSigner, _ := ed25519.GenerateKey(rand.Reader)
	sshSigner, _ := ssh.NewSignerFromSigner(edSigner)
	config.AddHostKey(sshSigner)

	log.Println("starting ssh server on", addr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer lis.Close()
	for {
		conn, err := lis.Accept()
		if err != nil {
			return err
		}

		go func(conn net.Conn) {
			defer conn.Close()

			sshConn, chanc, reqc, err := ssh.NewServerConn(conn, config)
			if err != nil {
				log.Println(err)
				return
			}
			defer sshConn.Close()
			go ssh.DiscardRequests(reqc)
			for chanr := range chanc {
				switch chanr.ChannelType() {
				case "session":
					ch, reqc, err := chanr.Accept()
					if err != nil {
						log.Println(err)
						return
					}
					handleSSHSession(dir, ch, reqc)
				}
			}
		}(conn)
	}
}

func handleSSHSession(dir string, ch ssh.Channel, reqc <-chan *ssh.Request) {
	defer ch.Close()

	var exitCode uint32
	defer func() {
		b := ssh.Marshal(struct{ Value uint32 }{exitCode})
		ch.SendRequest("exit-status", false, b)
	}()

	envs := make(map[string]string)
	for req := range reqc {
		switch req.Type {
		case "env":
			payload := struct{ Key, Value string }{}
			ssh.Unmarshal(req.Payload, &payload)
			envs[payload.Key] = payload.Value
			req.Reply(true, nil)

		case "exec":
			payload := struct{ Value string }{}
			ssh.Unmarshal(req.Payload, &payload)
			args, err := shlex.Split(payload.Value, true)
			if err != nil {
				log.Println("lex args", err)
				exitCode = 1
				return
			}

			cmd := args[0]
			switch cmd {
			case "git-upload-pack": // read
				if gp := envs["GIT_PROTOCOL"]; gp != "version=2" {
					log.Println("unhandled GIT_PROTOCOL", gp)
					exitCode = 1
					return
				}

				// TODO: get directory from args[1]

				err := handleUploadPack(dir, ch)
				if err != nil {
					log.Println(err)
					exitCode = 1
					return
				}

				req.Reply(true, nil)
				return

			default:
				req.Reply(false, nil)
				exitCode = 1
				return
			}

		default:
			req.Reply(false, nil)
			exitCode = 1
			return
		}
	}
}

func handleUploadPack(dir string, ch ssh.Channel) error {
	ctx := context.Background()

	ep, err := transport.NewEndpoint("/")
	if err != nil {
		return fmt.Errorf("create transport endpoint: %w", err)
	}
	bfs := osfs.New(dir)
	ld := server.NewFilesystemLoader(bfs)
	svr := server.NewServer(ld)
	sess, err := svr.NewUploadPackSession(ep, nil)
	if err != nil {
		return fmt.Errorf("create upload-pack session: %w", err)
	}

	ar, err := sess.AdvertisedReferencesContext(ctx)
	if err != nil {
		return fmt.Errorf("get advertised references: %w", err)
	}
	err = ar.Encode(ch)
	if err != nil {
		return fmt.Errorf("encode advertised references: %w", err)
	}

	upr := packp.NewUploadPackRequest()
	err = upr.Decode(ch)
	if err != nil {
		return fmt.Errorf("decode upload-pack request: %w", err)
	}

	res, err := sess.UploadPack(ctx, upr)
	if err != nil {
		return fmt.Errorf("create upload-pack response: %w", err)
	}
	err = res.Encode(ch)
	if err != nil {
		return fmt.Errorf("encode upload-pack response: %w", err)
	}

	return nil
}
