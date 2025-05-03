package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"io"

	"github.com/pkg/sftp"
	"github.com/spf13/afero"
	"golang.org/x/crypto/ssh"
)

type DummyFS struct {}

func (DummyFS) Fileread(req *sftp.Request) (io.ReaderAt, error) {
    if req == nil || req.Filepath == "" {
        return nil, fmt.Errorf("invalid request or path")
    }

    file, err := os.Open(req.Filepath)
    if err != nil {
        return nil, fmt.Errorf("failed to open file: %w", err)
    }

    // os.File implements io.ReaderAt, so we can return it directly
    return file, nil
}

func (DummyFS) Filewrite(req *sftp.Request)  (io.WriterAt, error) {
	return nil, fmt.Errorf("write not implemented")
}

func (DummyFS) Filecmd(*sftp.Request) error {
	return fmt.Errorf("cmd not implemented")
}

func (DummyFS) Filelist(*sftp.Request) (sftp.ListerAt, error) {
	fmt.Printf("got a listing request")
	return nil, fmt.Errorf("listing not implemented")
}

// func ListAt([]os.FileInfo, int64) (int, error) {
// 	return fmt.Errorf("listAt not implemented")
// }


// Simple in-memory file system using Afero
var fs = afero.NewMemMapFs()

// Setup a basic SSH server configuration
func sshConfig() *ssh.ServerConfig {
	privateBytes, err := os.ReadFile("sftp_host_key")
	if err != nil {
		log.Fatal("Failed to load private key: ", err)
	}
	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key: ", err)
	}

	config := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	config.AddHostKey(private)

	return config
}

// Start the SSH + SFTP server
func startSFTPServer() {
	listener, err := net.Listen("tcp", "127.0.0.1:2022")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	log.Println("Listening on 127.0.0.1:2022...")

	config := sshConfig()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Failed to accept connection:", err)
			continue
		}

		go handleConn(conn, config)
	}
}

func handleConn(conn net.Conn, config *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		log.Printf("Failed to handshake: %v", err)
		return
	}
	defer sshConn.Close()

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("Could not accept channel: %v", err)
			continue
		}

		go func(in <-chan *ssh.Request) {
			for req := range in {
				if req.Type == "subsystem" && string(req.Payload[4:]) == "sftp" {
					req.Reply(true, nil)
					dummyFS := DummyFS{}
					server := sftp.NewRequestServer(channel, sftp.Handlers{
						// FileGet:  sftp.FileReader(fs),
						FileGet: dummyFS,
						FilePut: dummyFS,
						FileCmd:  dummyFS,
						FileList: dummyFS,
					})
					if err := server.Serve(); err == io.EOF {
						server.Close()
						log.Println("SFTP client disconnected.")
					} else if err != nil {
						log.Println("SFTP server error:", err)
					}
					return
				}
				req.Reply(false, nil)
			}
		}(requests)
	}
}

func main() {
	// Optional: populate the virtual FS with a sample file
	afero.WriteFile(fs, "/example.txt", []byte("Hello from Afero FS!"), 0644)

	startSFTPServer()
}
