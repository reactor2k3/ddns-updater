package namecom

import (
	"net/http"

	"github.com/reactor2k3/ddns-updater/internal/provider/headers"
)

func setHeaders(request *http.Request) {
	headers.SetAccept(request, "application/json")
	headers.SetUserAgent(request)
}
