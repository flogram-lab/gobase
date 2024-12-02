package gobase

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"path"

	"github.com/go-faster/errors"
	"google.golang.org/grpc/credentials"
)

func loadTLSCredentialsServer() ([]grpc.ServerOption, error) {

	TLS_AUTHORITY := GetenvStr("TLS_AUTHORITY", "", false)

	var (
		serverCertFile   = path.Join(TLS_AUTHORITY, "server-cert.pem")
		serverKeyFile    = path.Join(TLS_AUTHORITY, "server-key.pem")
		clientCACertFile = path.Join(TLS_AUTHORITY, "ca-cert.pem")
	)

	// Load certificate of the CA who signed client's certificate
	pemClientCA, err := ioutil.ReadFile(clientCACertFile)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemClientCA) {
		return nil, errors.New("failed to add client CA's certificate")
	}

	// Load server's certificate and private key
	serverCert, err := tls.LoadX509KeyPair(serverCertFile, serverKeyFile)
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
	}

	tlsCredentials := credentials.NewTLS(config)

	return []grpc.ServerOption{grpc.Creds(tlsCredentials)}, nil
}