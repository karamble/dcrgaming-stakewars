// Package bridgeconn configures StakeWars' authenticated SDK bridge connection.
package bridgeconn

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const MaxPEM = 32768

type Config struct {
	Network    string `json:"network"`
	Host       string `json:"host"`
	Port       string `json:"port"`
	ClientCert string `json:"client_certificate"`
	ClientKey  string `json:"client_private_key"`
	BridgeCert string `json:"bridge_certificate"`
}

func Defaults() Config { return Config{Network: Network, Host: "127.0.0.1", Port: "8443"} }
func (c Config) Address() (string, error) {
	host := strings.Trim(strings.TrimSpace(c.Host), "[]")
	if net.ParseIP(host) == nil {
		return "", errors.New("Enter a valid IPv4 or IPv6 address.")
	}
	port, err := strconv.Atoi(strings.TrimSpace(c.Port))
	if err != nil || port < 1 || port > 65535 {
		return "", errors.New("Port must be between 1 and 65535.")
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}
func (c Config) Validate() error {
	switch c.Network {
	case "mainnet", "testnet3", "simnet":
	default:
		return errors.New("Choose mainnet, testnet3, or simnet for the bridge network.")
	}

	if _, err := c.Address(); err != nil {
		return err
	}
	for _, p := range []string{c.ClientCert, c.ClientKey, c.BridgeCert} {
		if len(p) == 0 || len(p) > MaxPEM {
			return errors.New("Paste all three PEM credentials (maximum 32 KiB each).")
		}
	}
	if _, err := tls.X509KeyPair([]byte(c.ClientCert), []byte(c.ClientKey)); err != nil {
		return errors.New("The client certificate and private key are invalid or do not match.")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(c.BridgeCert)) {
		return errors.New("The bridge certificate is not valid PEM.")
	}
	return nil
}
func Load(path string) (Config, error) {
	c := Defaults()
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, errors.New("Could not read saved bridge settings.")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 4*MaxPEM+1))
	if err != nil || len(data) > 4*MaxPEM {
		return c, errors.New("Saved bridge settings are unreadable or too large.")
	}
	if json.Unmarshal(data, &c) != nil {
		return Defaults(), errors.New("Saved bridge settings are invalid.")
	}
	return c, nil
}

// Save writes the copied private key with owner-only permissions. No credential
// is passed to a shell, logger, renderer or command-line flag.
func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.New("Could not create settings directory.")
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return errors.New("Could not encode settings.")
	}
	f, err := os.CreateTemp(dir, ".bridge-")
	if err != nil {
		return errors.New("Could not save bridge settings.")
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		return errors.New("Could not save bridge settings.")
	}
	return nil
}
