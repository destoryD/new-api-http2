package service

import (
	"context"
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

var (
	httpClient       *http.Client
	http2Client      *http.Client
	proxyClientLock  sync.Mutex
	proxyClients     = make(map[string]*http.Client)
	http2ProxyClients = make(map[string]*http.Client)
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
	if err := http2.ConfigureTransport(transport); err != nil {
		common.SysLog(fmt.Sprintf("Failed to configure HTTP/2 transport: %v, falling back to HTTP/1.1", err))
		// 如果配置失败，降级到 HTTP/1.1
		http2Client = httpClient
		return
	}

	if common.RelayTimeout == 0 {
		http2Client = &http.Client{
			Transport:     transport,
			CheckRedirect: checkRedirect,
		}
	} else {
		http2Client = &http.Client{
			Transport:     transport,
			Timeout:       time.Duration(common.RelayTimeout) * time.Second,
			CheckRedirect: checkRedirect,
		}
	}
}

func GetHttpClient() *http.Client {
	return httpClient
}

// GetHttp2Client 获取 HTTP/2 客户端
func GetHttp2Client() *http.Client {
	if http2Client == nil {
		// 如果未初始化，返回 HTTP/1.1 客户端作为备选
		return httpClient
	}
	return http2Client
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
		return GetHttp2Client(), nil
	}
	return NewHttp2ProxyHttpClient(proxyURL)
}

// ResetProxyClientCache 清空代理客户端缓存，确保下次使用时重新初始化
func ResetProxyClientCache() {
	proxyClientLock.Lock()
	defer proxyClientLock.Unlock()
	
	// 清理 HTTP/1.1 代理客户端
	for _, client := range proxyClients {
		if transport, ok := client.Transport.(*http.Transport); ok && transport != nil {
			transport.CloseIdleConnections()
		}
	}
	proxyClients = make(map[string]*http.Client)
	
	// 清理 HTTP/2 代理客户端
	for _, client := range http2ProxyClients {
		if transport, ok := client.Transport.(*http.Transport); ok && transport != nil {
			transport.CloseIdleConnections()
		}
	}
	http2ProxyClients = make(map[string]*http.Client)
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

// NewHttp2ProxyHttpClient 创建支持代理的 HTTP/2 客户端
func NewHttp2ProxyHttpClient(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		if client := GetHttp2Client(); client != nil {
			return client, nil
		}
		return http.DefaultClient, nil
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

		// 配置 HTTP/2
		if err := http2.ConfigureTransport(transport); err != nil {
			common.SysLog(fmt.Sprintf("Failed to configure HTTP/2 transport for proxy %s: %v", proxyURL, err))
			// 配置失败时，回退到 HTTP/1.1
			client := &http.Client{
				Transport:     transport,
				CheckRedirect: checkRedirect,
			}
			client.Timeout = time.Duration(common.RelayTimeout) * time.Second
			proxyClientLock.Lock()
			http2ProxyClients[proxyURL] = client
			proxyClientLock.Unlock()
			return client, nil
		}

		client := &http.Client{
			Transport:     transport,
			CheckRedirect: checkRedirect,
		}
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
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

		// 配置 HTTP/2
		if err := http2.ConfigureTransport(transport); err != nil {
			common.SysLog(fmt.Sprintf("Failed to configure HTTP/2 transport for SOCKS5 proxy %s: %v", proxyURL, err))
			// 配置失败时，回退到 HTTP/1.1
			client := &http.Client{Transport: transport, CheckRedirect: checkRedirect}
			client.Timeout = time.Duration(common.RelayTimeout) * time.Second
			proxyClientLock.Lock()
			http2ProxyClients[proxyURL] = client
			proxyClientLock.Unlock()
			return client, nil
		}

		client := &http.Client{Transport: transport, CheckRedirect: checkRedirect}
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
		proxyClientLock.Lock()
		http2ProxyClients[proxyURL] = client
		proxyClientLock.Unlock()
		return client, nil

	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s, must be http, https, socks5 or socks5h", parsedURL.Scheme)
	}
}
