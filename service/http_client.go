package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"golang.org/x/net/http2"
	"golang.org/x/net/proxy"
)

type strictHTTP2Transport struct {
	transport *http.Transport
}

func (t *strictHTTP2Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.ProtoMajor == 2 {
		return resp, nil
	}
	if resp.Body != nil {
		_ = resp.Body.Close()
	}
	return nil, fmt.Errorf("HTTP/2 is required but upstream negotiated %s", resp.Proto)
}

func (t *strictHTTP2Transport) CloseIdleConnections() {
	t.transport.CloseIdleConnections()
}

func configureStrictHTTP2Transport(transport *http.Transport) error {
	if err := http2.ConfigureTransport(transport); err != nil {
		return err
	}
	transport.ForceAttemptHTTP2 = true
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	}
	transport.TLSClientConfig.NextProtos = []string{"h2"}
	return nil
}

func newStrictHTTP2Client(transport *http.Transport) *http.Client {
	client := &http.Client{
		Transport:     &strictHTTP2Transport{transport: transport},
		CheckRedirect: checkRedirect,
	}
	if common.RelayTimeout != 0 {
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
	}
	return client
}

var (
	httpClient           *http.Client
	http2Client          *http.Client
	httpOnlyClient       *http.Client
	proxyClientLock      sync.Mutex
	proxyClients         = make(map[string]*http.Client)
	http2ProxyClients    = make(map[string]*http.Client)
	httpOnlyProxyClients = make(map[string]*http.Client)
	http2ClientLock      sync.Mutex
	httpOnlyClientLock   sync.Mutex
)

func checkRedirect(req *http.Request, via []*http.Request) error {
	fetchSetting := system_setting.GetFetchSetting()
	urlStr := req.URL.String()
	if err := common.ValidateURLWithFetchSetting(urlStr, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return fmt.Errorf("redirect to %s blocked: %v", urlStr, err)
	}
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	return nil
}

func InitHttpClient() {
	transport := &http.Transport{
		MaxIdleConns:        common.RelayMaxIdleConns,
		MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
		IdleConnTimeout:     time.Duration(common.RelayIdleConnTimeout) * time.Second,
		ForceAttemptHTTP2:   true,
		Proxy:               http.ProxyFromEnvironment, // Support HTTP_PROXY, HTTPS_PROXY, NO_PROXY env vars
	}
	if common.TLSInsecureSkipVerify {
		transport.TLSClientConfig = common.InsecureTLSConfig
	}

	if common.RelayTimeout == 0 {
		httpClient = &http.Client{
			Transport:     transport,
			CheckRedirect: checkRedirect,
		}
	} else {
		httpClient = &http.Client{
			Transport:     transport,
			Timeout:       time.Duration(common.RelayTimeout) * time.Second,
			CheckRedirect: checkRedirect,
		}
	}
}

func configureHTTPOnlyTransport(transport *http.Transport) {
	transport.ForceAttemptHTTP2 = false
	transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	}
	transport.TLSClientConfig.NextProtos = []string{"http/1.1"}
}

func newHTTPOnlyClient(transport *http.Transport) *http.Client {
	client := &http.Client{
		Transport:     transport,
		CheckRedirect: checkRedirect,
	}
	if common.RelayTimeout != 0 {
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
	}
	return client
}

func InitHttpOnlyClient() {
	transport := &http.Transport{
		MaxIdleConns:        common.RelayMaxIdleConns,
		MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
		Proxy:               http.ProxyFromEnvironment,
	}
	if common.TLSInsecureSkipVerify {
		transport.TLSClientConfig = common.InsecureTLSConfig
	}
	configureHTTPOnlyTransport(transport)
	httpOnlyClient = newHTTPOnlyClient(transport)
}

// InitHttp2Client 初始化 HTTP/2 客户端
func InitHttp2Client() {
	transport := &http.Transport{
		MaxIdleConns:        common.RelayMaxIdleConns,
		MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
		ForceAttemptHTTP2:   true,
		Proxy:               http.ProxyFromEnvironment,
	}
	if common.TLSInsecureSkipVerify {
		transport.TLSClientConfig = common.InsecureTLSConfig
	}

	// 配置 HTTP/2 参数
	if err := configureStrictHTTP2Transport(transport); err != nil {
		common.SysError(fmt.Sprintf("Failed to configure strict HTTP/2 transport: %v", err))
		http2Client = nil
		return
	}

	http2Client = newStrictHTTP2Client(transport)
}

func GetHttpClient() *http.Client {
	return httpClient
}

func GetHttpOnlyClient() *http.Client {
	if httpOnlyClient == nil {
		httpOnlyClientLock.Lock()
		defer httpOnlyClientLock.Unlock()
		if httpOnlyClient == nil {
			InitHttpOnlyClient()
		}
	}
	if httpOnlyClient == nil {
		return GetHttpClient()
	}
	return httpOnlyClient
}

// GetHttp2Client 获取 HTTP/2 客户端
func GetHttp2Client() (*http.Client, error) {
	if http2Client == nil {
		http2ClientLock.Lock()
		defer http2ClientLock.Unlock()
		if http2Client == nil {
			InitHttp2Client()
		}
	}
	if http2Client == nil {
		return nil, fmt.Errorf("strict HTTP/2 client is not initialized")
	}
	return http2Client, nil
}

// GetHttpClientWithProxy returns the default client or a proxy-enabled one when proxyURL is provided.
func GetHttpClientWithProxy(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		return GetHttpClient(), nil
	}
	return NewProxyHttpClient(proxyURL)
}

// GetHttp2ClientWithProxy 获取支持代理的 HTTP/2 客户端
func GetHttp2ClientWithProxy(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		return GetHttp2Client()
	}
	return NewHttp2ProxyHttpClient(proxyURL)
}

func GetHttpOnlyClientWithProxy(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		return GetHttpOnlyClient(), nil
	}
	return NewHTTPOnlyProxyHttpClient(proxyURL)
}

// ResetProxyClientCache 清空代理客户端缓存，确保下次使用时重新初始化
func ResetProxyClientCache() {
	proxyClientLock.Lock()
	defer proxyClientLock.Unlock()

	// 清理 HTTP/1.1 代理客户端
	for _, client := range proxyClients {
		client.CloseIdleConnections()
	}
	proxyClients = make(map[string]*http.Client)

	// 清理 HTTP/2 代理客户端
	for _, client := range http2ProxyClients {
		client.CloseIdleConnections()
	}
	http2ProxyClients = make(map[string]*http.Client)

	for _, client := range httpOnlyProxyClients {
		client.CloseIdleConnections()
	}
	httpOnlyProxyClients = make(map[string]*http.Client)
}

// NewProxyHttpClient 创建支持代理的 HTTP 客户端
func NewProxyHttpClient(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		if client := GetHttpClient(); client != nil {
			return client, nil
		}
		return http.DefaultClient, nil
	}

	proxyClientLock.Lock()
	if client, ok := proxyClients[proxyURL]; ok {
		proxyClientLock.Unlock()
		return client, nil
	}
	proxyClientLock.Unlock()

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	switch parsedURL.Scheme {
	case "http", "https":
		transport := &http.Transport{
			MaxIdleConns:        common.RelayMaxIdleConns,
			MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
			IdleConnTimeout:     time.Duration(common.RelayIdleConnTimeout) * time.Second,
			ForceAttemptHTTP2:   true,
			Proxy:               http.ProxyURL(parsedURL),
		}
		if common.TLSInsecureSkipVerify {
			transport.TLSClientConfig = common.InsecureTLSConfig
		}
		client := &http.Client{
			Transport:     transport,
			CheckRedirect: checkRedirect,
		}
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
		proxyClientLock.Lock()
		proxyClients[proxyURL] = client
		proxyClientLock.Unlock()
		return client, nil

	case "socks5", "socks5h":
		// 获取认证信息
		var auth *proxy.Auth
		if parsedURL.User != nil {
			auth = &proxy.Auth{
				User:     parsedURL.User.Username(),
				Password: "",
			}
			if password, ok := parsedURL.User.Password(); ok {
				auth.Password = password
			}
		}

		// 创建 SOCKS5 代理拨号器
		// proxy.SOCKS5 使用 tcp 参数，所有 TCP 连接包括 DNS 查询都将通过代理进行。行为与 socks5h 相同
		dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, proxy.Direct)
		if err != nil {
			return nil, err
		}

		transport := &http.Transport{
			MaxIdleConns:        common.RelayMaxIdleConns,
			MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
			IdleConnTimeout:     time.Duration(common.RelayIdleConnTimeout) * time.Second,
			ForceAttemptHTTP2:   true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		}
		if common.TLSInsecureSkipVerify {
			transport.TLSClientConfig = common.InsecureTLSConfig
		}

		client := &http.Client{Transport: transport, CheckRedirect: checkRedirect}
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
		proxyClientLock.Lock()
		proxyClients[proxyURL] = client
		proxyClientLock.Unlock()
		return client, nil

	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s, must be http, https, socks5 or socks5h", parsedURL.Scheme)
	}
}

func NewHTTPOnlyProxyHttpClient(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		if client := GetHttpOnlyClient(); client != nil {
			return client, nil
		}
		return http.DefaultClient, nil
	}

	proxyClientLock.Lock()
	if client, ok := httpOnlyProxyClients[proxyURL]; ok {
		proxyClientLock.Unlock()
		return client, nil
	}
	proxyClientLock.Unlock()

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	switch parsedURL.Scheme {
	case "http", "https":
		transport := &http.Transport{
			MaxIdleConns:        common.RelayMaxIdleConns,
			MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
			Proxy:               http.ProxyURL(parsedURL),
		}
		if common.TLSInsecureSkipVerify {
			transport.TLSClientConfig = common.InsecureTLSConfig
		}
		configureHTTPOnlyTransport(transport)
		client := newHTTPOnlyClient(transport)
		proxyClientLock.Lock()
		httpOnlyProxyClients[proxyURL] = client
		proxyClientLock.Unlock()
		return client, nil

	case "socks5", "socks5h":
		var auth *proxy.Auth
		if parsedURL.User != nil {
			auth = &proxy.Auth{
				User:     parsedURL.User.Username(),
				Password: "",
			}
			if password, ok := parsedURL.User.Password(); ok {
				auth.Password = password
			}
		}

		dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, proxy.Direct)
		if err != nil {
			return nil, err
		}

		transport := &http.Transport{
			MaxIdleConns:        common.RelayMaxIdleConns,
			MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		}
		if common.TLSInsecureSkipVerify {
			transport.TLSClientConfig = common.InsecureTLSConfig
		}
		configureHTTPOnlyTransport(transport)
		client := newHTTPOnlyClient(transport)
		proxyClientLock.Lock()
		httpOnlyProxyClients[proxyURL] = client
		proxyClientLock.Unlock()
		return client, nil

	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s, must be http, https, socks5 or socks5h", parsedURL.Scheme)
	}
}

// NewHttp2ProxyHttpClient 创建支持代理的 HTTP/2 客户端
func NewHttp2ProxyHttpClient(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		return GetHttp2Client()
	}

	proxyClientLock.Lock()
	if client, ok := http2ProxyClients[proxyURL]; ok {
		proxyClientLock.Unlock()
		return client, nil
	}
	proxyClientLock.Unlock()

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	switch parsedURL.Scheme {
	case "http", "https":
		transport := &http.Transport{
			MaxIdleConns:        common.RelayMaxIdleConns,
			MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
			ForceAttemptHTTP2:   true,
			Proxy:               http.ProxyURL(parsedURL),
		}
		if common.TLSInsecureSkipVerify {
			transport.TLSClientConfig = common.InsecureTLSConfig
		}

		if err := configureStrictHTTP2Transport(transport); err != nil {
			common.SysError(fmt.Sprintf("Failed to configure strict HTTP/2 transport for proxy %s: %v", proxyURL, err))
			return nil, err
		}

		client := newStrictHTTP2Client(transport)
		proxyClientLock.Lock()
		http2ProxyClients[proxyURL] = client
		proxyClientLock.Unlock()
		return client, nil

	case "socks5", "socks5h":
		// 获取认证信息
		var auth *proxy.Auth
		if parsedURL.User != nil {
			auth = &proxy.Auth{
				User:     parsedURL.User.Username(),
				Password: "",
			}
			if password, ok := parsedURL.User.Password(); ok {
				auth.Password = password
			}
		}

		// 创建 SOCKS5 代理拨号器
		dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, proxy.Direct)
		if err != nil {
			return nil, err
		}

		transport := &http.Transport{
			MaxIdleConns:        common.RelayMaxIdleConns,
			MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
			ForceAttemptHTTP2:   true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		}
		if common.TLSInsecureSkipVerify {
			transport.TLSClientConfig = common.InsecureTLSConfig
		}

		if err := configureStrictHTTP2Transport(transport); err != nil {
			common.SysError(fmt.Sprintf("Failed to configure strict HTTP/2 transport for SOCKS5 proxy %s: %v", proxyURL, err))
			return nil, err
		}

		client := newStrictHTTP2Client(transport)
		proxyClientLock.Lock()
		http2ProxyClients[proxyURL] = client
		proxyClientLock.Unlock()
		return client, nil

	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s, must be http, https, socks5 or socks5h", parsedURL.Scheme)
	}
}
