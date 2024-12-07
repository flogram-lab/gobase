package gobase

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path"

	"github.com/go-faster/errors"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type ClientConfig struct {
	Addr string `mapstructure:"addr"`
	Port string `mapstructure:"port"`
}

type ClientTLS struct {
	Dir          string `mapstructure:"dir"`
	SelfSignedCA string `mapstructure:"self-signed-ca"`
}

type ClientTLSMutual struct {
	Dir          string `mapstructure:"dir"`
	SelfSignedCA string `mapstructure:"self-signed-ca"`
	Client       string `mapstructure:"client"`
}

func (s ClientConfig) GetDialAddress() string {
	return fmt.Sprintf("%s:%s", s.Addr, s.Port)
}

// NewClientFromConfig creates client connection from environmental config (or custom config source)
//
// Config is read using key=value; pairs in key GRPC_SERVER_serviceName" value:
// A string must begin with first argument with no value (single string)
// Key-value pairs are separated with ';'
//
// Example:
//
//	"test;key1=value;key2=other value;"
//
// serviceName: locates config values by key "GRPC_SERVICE_serviceName
// globalConfig: config map, from where to read the string. If nil, environment variables are used
// opts: other gRPC server options to use
//
// panic: if read/parse fails, or key not found in globalConfig
// may return net.Listener bind error.
func NewClientFromConfig(serviceName string, globalConfig map[string]string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	config, securityOption, err := LoadClientConfig(serviceName, globalConfig)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(config.GetDialAddress(), append(opts, securityOption)...)

	return conn, err
}

func LoadClientConfig(serviceName string, globalConfig map[string]string) (ClientConfig, grpc.DialOption, error) {
	key := fmt.Sprintf("GRPC_CONNECT_%s", serviceName)

	var optsbase ClientConfig

	mode, opts, err := ParseConfstr(key, globalConfig)
	if err != nil {
		return optsbase, nil, err
	}

	if err := mapstructure.Decode(opts, &optsbase); err != nil {
		return optsbase, nil, errors.New(fmt.Sprintf("Invalid config for client security, key: '%s' (failed to parse options struct)", key))
	}

	switch mode {

	case "insecure":

		return optsbase, grpc.WithTransportCredentials(insecure.NewCredentials()), nil

	case "tls":

		var optsv ClientTLS
		if err := mapstructure.Decode(opts, &optsv); err != nil {
			return optsbase, nil, errors.New(fmt.Sprintf("Invalid config for client security, key: '%s' (failed to parse options struct)", key))
		}

		v, err := LoadClientSecurityTLS(optsv)
		if err != nil {
			return optsbase, nil, errors.Wrap(err, "LoadClientSecurityTLS")
		}

		return optsbase, v, nil

	case "tls-mutual":

		var optsv ClientTLSMutual
		if err := mapstructure.Decode(opts, &optsv); err != nil {
			return optsbase, nil, errors.New(fmt.Sprintf("Invalid config for client security, key: '%s' (failed to parse options struct)", key))
		}

		v, err := LoadClientSecurityTLSMutual(optsv)
		if err != nil {
			return optsbase, nil, errors.Wrap(err, "LoadClientSecurityTLSMutual")
		}

		return optsbase, v, nil

	default:
		return optsbase, nil, errors.New(fmt.Sprintf("Invalid config for client security, key: '%s' (invalid mode '%s')", key, mode))
	}
}

func LoadClientSecurityTLS(opts ClientTLS) (grpc.DialOption, error) {
	if opts.Dir == "" {
		return nil, errors.New("No 'dir' specified to load certificates and keys from")
	}

	var (
		CACertFile = path.Join(opts.Dir, "ca-cert.pem")
	)

	// Create the credentials and return it
	var config *tls.Config = nil

	if opts.SelfSignedCA == "1" {

		// Load certificate of the CA who signed client's certificate
		pemRootCA, err := os.ReadFile(CACertFile)
		if err != nil {
			return nil, err
		}

		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(pemRootCA) {
			return nil, errors.New("failed to add client CA's certificate")
		}

		config = &tls.Config{
			RootCAs: certPool,
		}
	}

	return grpc.WithTransportCredentials(credentials.NewTLS(config)), nil
}

func LoadClientSecurityTLSMutual(opts ClientTLSMutual) (grpc.DialOption, error) {
	if opts.Dir == "" {
		return nil, errors.New("No 'dir' specified to load certificates and keys from")
	}

	var (
		CACertFile     = path.Join(opts.Dir, "ca-cert.pem")
		clientCertFile = path.Join(opts.Dir, fmt.Sprintf("client-%s-cert.pem", opts.Client))
		clientKeyFile  = path.Join(opts.Dir, fmt.Sprintf("client-%s-key.pem", opts.Client))
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

	clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	config := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		RootCAs:      certPool,
	}

	return grpc.WithTransportCredentials(credentials.NewTLS(config)), nil
}
