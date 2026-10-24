package publicip

import (
	"net/http"

	"github.com/reactor2k3/ddns-updater/pkg/publicip/dns"
	iphttp "github.com/reactor2k3/ddns-updater/pkg/publicip/http"
)

type settings struct {
	// If both dns and http are enabled it will cycle between both of them.
	dns  DNSSettings
	http HTTPSettings
}

type DNSSettings struct {
	Enabled bool
	Options []dns.Option
}

type HTTPSettings struct {
	Enabled bool
	Client  *http.Client
	Options []iphttp.Option
}
