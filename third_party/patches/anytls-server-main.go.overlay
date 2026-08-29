// LucX overlay on anytls-go v0.0.13 cmd/server/main.go.
// Adds -cert/-key so the panel can pass its ACME pair instead of the
// process-local self-signed certificate GenerateKeyPair always minted.

package main

import (
	"anytls/proxy/padding"
	"anytls/util"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"flag"
	"io"
	"net"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

var passwordSha256 []byte

func main() {
	listen := flag.String("l", "0.0.0.0:8443", "server listen port")
	password := flag.String("p", "", "password")
	paddingScheme := flag.String("padding-scheme", "", "padding-scheme")
	certFile := flag.String("cert", "", "tls certificate pem")
	keyFile := flag.String("key", "", "tls private key pem")
	flag.Parse()

	if *password == "" {
		logrus.Fatalln("please set password")
	}
	if *paddingScheme != "" {
		if f, err := os.Open(*paddingScheme); err == nil {
			b, err := io.ReadAll(f)
			if err != nil {
				logrus.Fatalln(err)
			}
			if padding.UpdatePaddingScheme(b) {
				logrus.Infoln("loaded padding scheme file:", *paddingScheme)
			} else {
				logrus.Errorln("wrong format padding scheme file:", *paddingScheme)
			}
			f.Close()
		} else {
			logrus.Fatalln(err)
		}
	}

	logLevel, err := logrus.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	logrus.SetLevel(logLevel)

	var sum = sha256.Sum256([]byte(*password))
	passwordSha256 = sum[:]

	logrus.Infoln("[Server]", util.ProgramVersionName)
	logrus.Infoln("[Server] Listening TCP", *listen)

	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		logrus.Fatalln("listen server tcp:", err)
	}

	var tlsCert *tls.Certificate
	if *certFile != "" || *keyFile != "" {
		if *certFile == "" || *keyFile == "" {
			logrus.Fatalln("both -cert and -key are required")
		}
		c, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			logrus.Fatalln("load certificate:", err)
		}
		tlsCert = &c
		logrus.Infoln("[Server] TLS certificate", *certFile)
	} else {
		tlsCert, _ = util.GenerateKeyPair(time.Now, "")
		logrus.Infoln("[Server] TLS using ephemeral self-signed certificate")
	}

	tlsConfig := &tls.Config{
		GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return tlsCert, nil
		},
	}

	ctx := context.Background()
	server := NewMyServer(tlsConfig)

	for {
		c, err := listener.Accept()
		if err != nil {
			logrus.Fatalln("accept:", err)
		}
		go handleTcpConnection(ctx, c, server)
	}
}
