package tunnel

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ClientConfig holds the configuration for the WSTunnelClient
type ClientConfig struct {
	Token          string
	Password       string
	Tunnel         string
	Server         string
	Insecure       bool
	Regexp         string
	Timeout        int
	PidFile        string
	LogFile        string
	StatusFile     string
	Proxy          string
	ClientPorts    string
	CertFile       string
	ReconnectDelay int
	MaxRetries     int
}

// ParseClientConfig parses command line arguments into a ClientConfig
func ParseClientConfig(args []string) (*ClientConfig, error) {
	config := &ClientConfig{}

	cliFlag := flag.NewFlagSet("client", flag.ContinueOnError)
	cliFlag.SetOutput(io.Discard)
	cliFlag.StringVar(&config.Token, "token", "",
		"rendez-vous token identifying this server (format: token or token:password)")
	cliFlag.StringVar(&config.Tunnel, "tunnel", "",
		"websocket server ws[s]://user:pass@hostname:port to connect to")
	cliFlag.StringVar(&config.Server, "server", "",
		"http server http[s]://hostname:port to send received requests to")
	cliFlag.BoolVar(&config.Insecure, "insecure", false,
		"accept self-signed SSL certs from local HTTPS servers")
	cliFlag.StringVar(&config.Regexp, "regexp", "",
		"regexp for local HTTP(S) server to allow sending received requests to")
	cliFlag.IntVar(&config.Timeout, "timeout", 30, "timeout on websocket in seconds")
	cliFlag.StringVar(&config.PidFile, "pidfile", "", "path for pidfile")
	cliFlag.StringVar(&config.LogFile, "logfile", "", "path for log file")
	cliFlag.StringVar(&config.StatusFile, "statusfile", "", "path for status file")
	cliFlag.StringVar(&config.Proxy, "proxy", "",
		"use HTTPS proxy http://user:pass@hostname:port")
	cliFlag.StringVar(&config.ClientPorts, "client-ports", "",
		"comma separated list of client listening ports ex: -client-ports 8000..8100,8300..8400,8500,8505")
	cliFlag.StringVar(&config.CertFile, "certfile", "", "path for trusted certificate in PEM-encoded format")
	cliFlag.IntVar(&config.ReconnectDelay, "reconnect-delay", 5, "delay between reconnection attempts in seconds")
	cliFlag.IntVar(&config.MaxRetries, "max-retries", 0, "maximum number of reconnection attempts (0 for unlimited)")

	if err := cliFlag.Parse(args); err != nil {
		return nil, err
	}

	return config, nil
}

// NewWSTunnelClientFromConfig creates a new WSTunnelClient from a ClientConfig
func NewWSTunnelClientFromConfig(config *ClientConfig) (*WSTunnelClient, error) {
	client := &WSTunnelClient{
		Token:       config.Token,
		Password:    config.Password,
		Server:      config.Server,
		Insecure:    config.Insecure,
		Cert:        config.CertFile,
		Timeout:     time.Duration(config.Timeout) * time.Second,
		Log:         makeLogger("WStuncli", config.LogFile, ""),
		connManager: NewConnectionManager(time.Duration(config.ReconnectDelay)*time.Second, config.MaxRetries),
	}

	// Parse token:password format
	if config.Token != "" {
		parts := strings.SplitN(config.Token, ":", 2)
		client.Token = strings.TrimSpace(parts[0])
		if len(parts) == 2 {
			client.Password = strings.TrimSpace(parts[1])
		}
	}

	// Parse tunnel URL
	if config.Tunnel != "" {
		tunnelURL, err := url.Parse(config.Tunnel)
		if err != nil {
			return nil, fmt.Errorf("invalid tunnel address: %q, %v", config.Tunnel, err)
		}
		if tunnelURL.Scheme != "ws" && tunnelURL.Scheme != "wss" {
			return nil, fmt.Errorf("remote tunnel (-tunnel option) must begin with ws:// or wss://")
		}
		// Strip any custom path, query and fragment since tunnel endpoint is fixed to /_tunnel
		tunnelURL.Path = ""
		tunnelURL.RawPath = ""
		tunnelURL.RawQuery = ""
		tunnelURL.Fragment = ""
		client.Tunnel = tunnelURL
	} else {
		return nil, fmt.Errorf("must specify tunnel server ws://hostname:port using -tunnel option")
	}

	// Parse regexp
	if config.Regexp != "" {
		var err error
		client.Regexp, err = regexp.Compile(config.Regexp)
		if err != nil {
			return nil, fmt.Errorf("can't parse -regexp: %v", err)
		}
	}

	// Parse proxy
	if config.Proxy != "" {
		proxyURL, err := url.Parse(config.Proxy)
		if err != nil || !strings.HasPrefix(proxyURL.Scheme, "http") {
			if proxyURL, err = url.Parse("http://" + config.Proxy); err != nil {
				return nil, fmt.Errorf("invalid proxy address: %q, %v", config.Proxy, err)
			}
		}
		client.Proxy = proxyURL
	}

	// Parse client ports
	if config.ClientPorts != "" {
		ports, err := parseClientPorts(config.ClientPorts)
		if err != nil {
			return nil, err
		}
		client.ClientPorts = ports
	}

	// Create status file
	if config.StatusFile != "" {
		fd, err := os.Create(config.StatusFile)
		if err != nil {
			return nil, fmt.Errorf("can't create statusfile: %v", err)
		}
		client.StatusFd = fd
	}

	// Write PID file
	if config.PidFile != "" {
		if err := writePid(config.PidFile); err != nil {
			return nil, fmt.Errorf("can't write pidfile: %v", err)
		}
	}

	return client, nil
}

// parseClientPorts parses a comma-separated list of ports and port ranges
func parseClientPorts(portsStr string) ([]int, error) {
	portList := strings.Split(portsStr, ",")
	clientPorts := []int{}

	for _, v := range portList {
		if strings.Contains(v, "..") {
			k := strings.Split(v, "..")
			bInt, err := strconv.Atoi(k[0])
			if err != nil {
				return nil, fmt.Errorf("invalid port assignment: %q in range: %q", k[0], v)
			}

			eInt, err := strconv.Atoi(k[1])
			if err != nil {
				return nil, fmt.Errorf("invalid port assignment: %q in range: %q", k[1], v)
			}

			if eInt < bInt {
				return nil, fmt.Errorf("end port %d cannot be less than beginning port %d", eInt, bInt)
			}

			for n := bInt; n <= eInt; n++ {
				clientPorts = append(clientPorts, n)
			}
		} else {
			intPort, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("cannot convert %q to integer", v)
			}
			clientPorts = append(clientPorts, intPort)
		}
	}

	return clientPorts, nil
}
