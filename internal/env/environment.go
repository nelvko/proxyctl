package env

// ProxyEnv contains the proxy variables emitted for a shell.
type ProxyEnv struct{ HTTP, HTTPS, All, NO string }

const noProxyValue = "localhost,127.0.0.1,::1,.local"

func ForProxy(httpURL, socksURL string) ProxyEnv {
	return ProxyEnv{HTTP: httpURL, HTTPS: httpURL, All: socksURL, NO: noProxyValue}
}
