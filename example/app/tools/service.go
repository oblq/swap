package tools

import (
	"fmt"
	"net"
	"net/url"
	"path"

	"github.com/oblq/swap"
)

var PublicIP string

func init() {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	if err := conn.Close(); err != nil {
		fmt.Println(err.Error())
	}
	PublicIP = localAddr.IP.String()
}

// ExtendedURL ---------------------------------------------------------------------------------------------------------

type ExtendedURL struct {
	url.URL
}

func (eu *ExtendedURL) WithPath(urlPath string) *ExtendedURL {
	eu.URL.Path = path.Join(eu.URL.Path, urlPath)
	return eu
}

func (eu *ExtendedURL) WithQueryParams(queryParams *map[string]string) *ExtendedURL {
	if queryParams != nil {
		q := eu.URL.Query()
		for k, v := range *queryParams {
			q.Add(k, v)
		}
		eu.URL.RawQuery = q.Encode()
	}
	return eu
}

// Service -------------------------------------------------------------------------------------------------------------

// Service is an abstraction of a service (or microservice) or the monolith itself.
// it holds some basic information useful for service discovery.
type Service struct {
	// Name is the service name.
	Name string `yaml:"Name,omitempty" json:"Name,omitempty" toml:"Name,omitempty"`

	// Version of the service.
	Version string `swap:"default=1" yaml:"Version,omitempty" json:"Version,omitempty" toml:"Version,omitempty"`

	// Data is optional, set custom data here.
	Data map[string]interface{} `yaml:"Data,omitempty" json:"Data,omitempty" toml:"Data,omitempty"`

	// PrivIP is the private IP of the machine running this service
	// use <tasks.serviceName> in Docker swarm or simply <serviceName> with Docker.
	PrivIP string `yaml:"PrivIP,omitempty" json:"PrivIP,omitempty" toml:"PrivIP,omitempty"`

	// PubIP is the public IP of the machine running this service
	// use <tasks.serviceName> in Docker swarm or simply <serviceName> with Docker.
	PubIP string `yaml:"PubIP,omitempty" json:"PubIP,omitempty" toml:"PubIP,omitempty"`

	// Hosts contains the host names pointing to this service.
	// The first one will be used to build the service URL,
	// others may be useful for CORS config or whatever you need
	// and they can be retrieved using URLs().
	Hosts []string `yaml:"Hosts,omitempty" json:"Hosts,omitempty" toml:"Hosts,omitempty"`

	// OverrideHost will automatically use
	// the machine public ip as the primary host when true.
	// That will make possible to connect to it
	// through your local network while in local development.
	OverrideHost bool `yaml:"OverrideHost,omitempty" json:"OverrideHost,omitempty" toml:"OverrideHost,omitempty"`

	// Port 443 automatically set https scheme when you get the service url.
	// Port 80 and all the others automatically set http scheme when you get the service url.
	Port int `swap:"default=80" yaml:"Port,omitempty" json:"Port,omitempty" toml:"Port,omitempty"`

	// Basepath is optional, it will be parsed by
	// the template package, so you can use placeholders here
	// (eg.: "{{.Name}}/v{{.Version}}" -> 'api/v1')
	Basepath string `yaml:"Basepath,omitempty" json:"Basepath,omitempty" toml:"Basepath,omitempty"`
}

// Configure is the swap 'configurable' interface.
func (s *Service) Configure(configFiles ...string) (err error) {
	err = swap.Parse(s, configFiles...)
	if s.OverrideHost {
		localIP := []string{PublicIP}
		s.Hosts = append(localIP, s.Hosts...)
	}
	return err
}

// scheme returns the service scheme (http or https), based on service port.
func (s *Service) scheme() string {
	if s.Port == 443 {
		return "https"
	}
	return "http"
}

// portForURL returns the service port for URL.
// If service.Port == 443 or 80 returns an empty string,
// otherwise, ":<Port>" is returned.
func (s *Service) portForURL() (port string) {
	if s.Port != 443 && s.Port != 80 {
		port = fmt.Sprintf(":%d", s.Port)
	}
	return
}

// hostName returns the first host listed in the config file
// not including the service port.
// If len(s.Hosts) == 0 or OverrideHostWPublicIP == true
// returns the machine public IP.
func (s *Service) hostName() string {
	if len(s.Hosts) > 0 {
		return s.Hosts[0]
	}
	return PublicIP
}

// host returns the first host listed in the config file
// including the port when needed (!= 80 && != 443).
func (s *Service) host() string {
	return s.hostName() + s.portForURL()
}

// URL returns the service URL using the first provided host
// and by eventually adding the provided path and query parameters.
func (s *Service) URL() *ExtendedURL {
	extURL := &ExtendedURL{url.URL{}}
	extURL.URL.Scheme = s.scheme()
	extURL.URL.Host = s.host()
	extURL.URL.Path = s.Basepath
	return extURL
}

// URLs return the service alternative URLs
// permuting any provided host.
func (s *Service) URLs() ([]url.URL, []string) {
	urls := make([]url.URL, 0)
	urlsString := make([]string, 0)

	for _, host := range s.Hosts {
		nURL := url.URL{}
		nURL.Scheme = s.scheme()
		nURL.Host = host + s.portForURL()
		nURL.Path = s.Basepath

		urls = append(urls, nURL)
		urlsString = append(urlsString, nURL.String())
	}

	return urls, urlsString
}
