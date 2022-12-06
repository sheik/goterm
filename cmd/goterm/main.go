package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

var (
	debug      = flag.Bool("debug", false, "turn debug logging on")
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	sshClient  = flag.Bool("ssh", false, "enable ssh client")
	user       = flag.String("u", "", "ssh user")
	host       = flag.String("h", "", "ssh host:port")
)

func (s *SSH) Read(p []byte) (n int, err error) {
	return s.session.stdout.Read(p)
}

func (s *SSH) Write(p []byte) (n int, err error) {
	return s.term.stdin.Write(p)
}

type SSH struct {
	session *Session
	term    *Term
}

type Session struct {
	stdin  *io.PipeWriter
	stdout io.Reader
}

type Term struct {
	stdin  *io.PipeWriter
	stdout io.Reader
}

func main() {
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	width := 120
	height := 34
	//	var gui = &XGBGui{}
	var gui = &GioGUI{}
	var s *SSH

	if *sshClient {
		fmt.Print("Enter Password: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		config := &ssh.ClientConfig{
			User: *user,
			Auth: []ssh.AuthMethod{
				ssh.Password(string(bytePassword)),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
		client, err := ssh.Dial("tcp", *host, config)
		if err != nil {
			log.Fatal("Failed to dial: ", err)
		}
		defer client.Close()

		// Each ClientConn can support multiple interactive sessions,
		// represented by a Session.
		session, err := client.NewSession()
		if err != nil {
			log.Fatal("Failed to create session: ", err)
		}
		defer session.Close()

		// Once a Session is created, you can execute a single command on
		// the remote side using the Run method.
		reader, writer := io.Pipe()
		r2, w2 := io.Pipe()
		s = &SSH{
			session: &Session{
				stdin:  writer,
				stdout: reader,
			},
			term: &Term{
				stdin:  w2,
				stdout: r2,
			},
		}

		session.Stdout = s.session.stdin
		session.Stdin = s.term.stdout

		modes := ssh.TerminalModes{
			ssh.ECHO:  1,
			ssh.IGNCR: 0,
		}

		err = session.RequestPty("xterm-256color", height, width, modes)
		if err != nil {
			log.Fatalf("failed to start pty: %s", err)
		}

		// Start remote shell
		if err := session.Shell(); err != nil {
			log.Fatalf("failed to start shell: %s", err)
		}
		time.Sleep(1 * time.Second)
		//		session.Run("/bin/bash")

		_, err = NewTerminal(s, gui, width, height)
		if err != nil {
			log.Fatal("failed to start terminal:", err)
		}

	} else {
		c := exec.Command("/bin/bash")

		localPty, err := pty.Start(c)
		if err != nil {
			return
		}

		pty.Setsize(localPty, &pty.Winsize{
			Rows: uint16(height),
			Cols: uint16(width),
			X:    0,
			Y:    0,
		})

		_, err = NewTerminal(localPty, gui, width, height)
		if err != nil {
			log.Fatal(err)
		}

	}

	os.Setenv("TERM", "xterm-256color")

	gui.Main()
}
