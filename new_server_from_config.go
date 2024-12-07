package gobase

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path"
	"strings"

	"github.com/go-faster/errors"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type ServerConfig struct {
	Addr string `mapstructure:"addr"`
	Port string `mapstructure:"port"`
}

type ServerTLS struct {
	Dir          string `mapstructure:"dir"`
	SelfSignedCA string `mapstructure:"self-signed-ca"`
}

type ServerTLSMutual struct {
	Dir          string `mapstructure:"dir"`
	SelfSignedCA string `mapstructure:"self-signed-ca"`
	Clients      string `mapstructure:"clients"`
}

func (s ServerConfig) GetBindAddress() string {
	return fmt.Sprintf("%s:%s", s.Addr, s.Port)
}

// NewServerWithConfig creates server and listener from environmental config (or custom config source)
//
//
// Config is read using key=value; pairs in key GRPC_SERVER_serviceName" value:
// A string must begin with first argument with no value (single string)
// Key-value pairs are separated with ';'

// Example:
//
//	"test;key1=value;key2=other value;"
//
// serverName: locates config values by key "GRPC_SERVER_serviceName
// globalConfig: config map, from where to read the string. If nil, environment variables are used
// opts: other gRPC server options to use
//
// panic: if read/parse fails, or key not found in globalConfig
// may return net.Listener bind error.
func NewServerFromConfig(serverName string, globalConfig map[string]string, opts ...grpc.ServerOption) (*grpc.Server, net.Listener, error) {
	config, securityOption, err := LoadServerConfig(serverName, globalConfig)
	if err != nil {
		return nil, nil, err
	}

	server := grpc.NewServer(append(opts, securityOption)...)

	listener, err := net.Listen("tcp", config.GetBindAddress())

	return server, listener, err
}

func LoadServerConfig(serverName string, globalConfig map[string]string) (ServerConfig, grpc.ServerOption, error) {
	key := fmt.Sprintf("GRPC_SERVER_%s", serverName)

	var optsbase ServerConfig

	mode, opts, err := ParseConfstr(key, globalConfig)
	if err != nil {
		return optsbase, nil, err
	}

	if err := mapstructure.Decode(opts, &optsbase); err != nil {
		return optsbase, nil, errors.New(fmt.Sprintf("Invalid config for client security, key: '%s' (failed to parse options struct)", key))
	}

	switch mode {

	case "insecure":

		return optsbase, grpc.Creds(insecure.NewCredentials()), nil

	case "tls":

		var optsv ServerTLS
		if err := mapstructure.Decode(opts, &optsv); err != nil {
			return optsbase, nil, errors.New(fmt.Sprintf("Invalid config for client security, key: '%s' (failed to parse options struct for mode '%s')", key, mode))
		}

		v, err := LoadServerSecurityTLS(optsv)
		if err != nil {
			return optsbase, nil, errors.Wrap(err, "LoadServerSecurityTLS")
		}

		return optsbase, v, nil

	case "tls-mutual":

		var optsv ServerTLSMutual
		if err := mapstructure.Decode(opts, &optsv); err != nil {
			return optsbase, nil, errors.New(fmt.Sprintf("Invalid config for client security, key: '%s' (failed to parse options struct)", key))
		}

		v, err := LoadServerSecurityTLSMutual(optsv)
		if err != nil {
			return optsbase, nil, errors.Wrap(err, "LoadServerSecurityTLSMutual")
		}

		return optsbase, v, nil

	default:
		return optsbase, nil, errors.New(fmt.Sprintf("Invalid config for client security, key: '%s' (invalid mode '%s')", key, mode))
	}
}

func LoadServerSecurityTLS(config ServerTLS) (grpc.ServerOption, error) {
	if config.Dir == "" {
		return nil, errors.New("No 'dir' specified to load certificates and keys from")
	}

	var (
		CACertFile     = path.Join(config.Dir, "ca-cert.pem")
		serverCertFile = path.Join(config.Dir, "server-cert.pem")
		serverKeyFile  = path.Join(config.Dir, "server-key.pem")
	)

	// Load certificate of the CA who signed client's certificate
	pemRootCA, err := os.ReadFile(CACertFile)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemRootCA) {
		return nil, errors.New("failed to add client CA's certificate")
	}

	// Load server's certificate and private key
	serverCert, err := tls.LoadX509KeyPair(serverCertFile, serverKeyFile)
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.NoClientCert,
		ClientCAs:    certPool,
	}

	tlsCredentials := credentials.NewTLS(tlsConfig)

	return grpc.Creds(tlsCredentials), nil
}

func LoadServerSecurityTLSMutual(config ServerTLSMutual) (grpc.ServerOption, error) {
	if config.Dir == "" {
		return nil, errors.New("No 'dir' specified to load certificates and keys from")
	}

	var (
		CACertFile     = path.Join(config.Dir, "ca-cert.pem")
		serverCertFile = path.Join(config.Dir, "server-cert.pem")
		serverKeyFile  = path.Join(config.Dir, "server-key.pem")
	)

	// Load certificate of the CA who signed client's certificate
	pemClientCA, err := os.ReadFile(CACertFile)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemClientCA) {
		return nil, errors.New("failed to add root CA's certificate")
	}

	for _, client := range strings.Split(config.Clients, ",") {
		var (
			clientCertFile = path.Join(config.Dir, fmt.Sprintf("client-%s-cert.pem", client))
			clientKeyFile  = path.Join(config.Dir, fmt.Sprintf("client-%s-key.pem", client))
		)

		if !certPool.AppendCertsFromPEM(pemClientCA) {
			return nil, errors.Errorf("failed to add client CA's certificate, client: '%s', files: %s,%s", client, clientCertFile, clientKeyFile)
		}
	}

	// Load server's certificate and private key
	serverCert, err := tls.LoadX509KeyPair(serverCertFile, serverKeyFile)
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
	}

	return grpc.Creds(credentials.NewTLS(tlsConfig)), nil
}
