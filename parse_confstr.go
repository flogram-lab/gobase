package gobase

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-faster/errors"
)

// EnvVarsConfig should be used to indicate config variable containing settings for must be read by ParseConfstr() from os.Environ
var EnvVarsConfig map[string]string = nil

// ParseConfstr splits value of a variable by key=value; pairs.
// A string must begin with first argument with no value (single string)
// Key-value pairs are separated with ';'
// Example:
//
//	"test;key1=value;key2=other value;"
//
// key: config key name to read value from
// globalConfig: config map, from where to read the string. If nil (EnvVarsConfig), environment variables are used
func ParseConfstr(key string, globalConfig map[string]string) (string, map[string]string, error) {
	var (
		configString string
		ok           bool
	)

	if globalConfig != nil {
		configString, ok = globalConfig[key]
	} else {
		configString, ok = os.LookupEnv(key)
	}

	if !ok {
		return "", nil, errors.New(fmt.Sprintf("Config for client security is not used, key: '%s'", key))
	}

	config := strings.Split(configString, ";")
	if len(config) < 1 {
		return "", nil, errors.New(fmt.Sprintf("Invalid config for client security, key: '%s', value: '%s'", key, configString))
	}

	mode := config[0]
	if strings.Contains(mode, "=") {
		return "", nil, errors.New(fmt.Sprintf("Invalid config for client security, key: '%s', value: '%s' (first parameter must not contain separator '=')", key, configString))
	}

	opts := make(map[string]string, len(config)-1)
	for i := 1; i < len(config); i++ {
		pair := strings.SplitN(config[i], "=", 1)
		if len(pair) != 2 {
			return "", nil, errors.New(fmt.Sprintf("Invalid config for client security, key: '%s', value: '%s' (parse key=value failed)", key, configString))
		}

		if pair[1] == "" {
			return "", nil, errors.New(fmt.Sprintf("Invalid config for client security, key: '%s', value: '%s' (key %s has empty val)", key, configString, pair[0]))
		}

		opts[pair[0]] = pair[1]
	}

	return mode, opts, nil
}
