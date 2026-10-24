package utils

import (
	"github.com/reactor2k3/ddns-updater/internal/models"
	"github.com/reactor2k3/ddns-updater/pkg/publicip/ipversion"
)

func ToString(domain, owner string, provider models.Provider, ipVersion ipversion.IPVersion) string {
	return "[domain: " + domain + " | owner: " + owner + " | provider: " +
		string(provider) + " | ip: " + ipVersion.String() + "]"
}
