package proxy

import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/portainer/portainer/api/http/proxy/provider/docker"

	"github.com/portainer/portainer/api/http/proxy/provider/azure"

	"github.com/portainer/portainer/api"
	"github.com/portainer/portainer/api/crypto"
)

const azureAPIBaseURL = "https://management.azure.com"

// proxyFactory is a factory to create reverse proxies to Docker endpoints
type proxyFactory struct {
	ResourceControlService portainer.ResourceControlService
	UserService            portainer.UserService
	TeamMembershipService  portainer.TeamMembershipService
	SettingsService        portainer.SettingsService
	RegistryService        portainer.RegistryService
	DockerHubService       portainer.DockerHubService
	SignatureService       portainer.DigitalSignatureService
	ReverseTunnelService   portainer.ReverseTunnelService
	ExtensionService       portainer.ExtensionService
}

func (factory *proxyFactory) newHTTPProxy(u *url.URL) http.Handler {
	u.Scheme = "http"
	return httputil.NewSingleHostReverseProxy(u)
}

func newAzureProxy(credentials *portainer.AzureCredentials) (http.Handler, error) {
	remoteURL, err := url.Parse(azureAPIBaseURL)
	if err != nil {
		return nil, err
	}

	proxy := newSingleHostReverseProxyWithHostHeader(remoteURL)
	proxy.Transport = azure.NewTransport(credentials)

	return proxy, nil
}

func (factory *proxyFactory) newDockerHTTPSProxy(u *url.URL, tlsConfig *portainer.TLSConfiguration, endpoint *portainer.Endpoint) (http.Handler, error) {
	u.Scheme = "https"

	proxy := factory.createDockerReverseProxy(u, endpoint)
	config, err := crypto.CreateTLSConfigurationFromDisk(tlsConfig.TLSCACertPath, tlsConfig.TLSCertPath, tlsConfig.TLSKeyPath, tlsConfig.TLSSkipVerify)
	if err != nil {
		return nil, err
	}

	proxy.Transport.(*docker.Transport).HTTPTransport.TLSClientConfig = config
	return proxy, nil
}

func (factory *proxyFactory) newDockerHTTPProxy(u *url.URL, endpoint *portainer.Endpoint) http.Handler {
	u.Scheme = "http"
	return factory.createDockerReverseProxy(u, endpoint)
}

func (factory *proxyFactory) createDockerReverseProxy(u *url.URL, endpoint *portainer.Endpoint) *httputil.ReverseProxy {
	transportParameters := &docker.TransportParameters{
		EnableSignature:        false,
		EndpointIdentifier:     endpoint.ID,
		EndpointType:           endpoint.Type,
		ResourceControlService: factory.ResourceControlService,
		UserService:            factory.UserService,
		TeamMembershipService:  factory.TeamMembershipService,
		RegistryService:        factory.RegistryService,
		DockerHubService:       factory.DockerHubService,
		SettingsService:        factory.SettingsService,
		ReverseTunnelService:   factory.ReverseTunnelService,
		ExtensionService:       factory.ExtensionService,
		SignatureService:       nil,
	}

	if endpoint.Type == portainer.AgentOnDockerEnvironment {
		transportParameters.EnableSignature = true
		transportParameters.SignatureService = factory.SignatureService
	}

	proxy := newSingleHostReverseProxyWithHostHeader(u)
	proxy.Transport = docker.NewTransport(transportParameters, &http.Transport{})
	return proxy
}

func newSocketTransport(socketPath string) *http.Transport {
	return &http.Transport{
		Dial: func(proto, addr string) (conn net.Conn, err error) {
			return net.Dial("unix", socketPath)
		},
	}
}
