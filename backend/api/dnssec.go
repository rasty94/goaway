package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/miekg/dns"
)

func (api *API) dnssecDiagnose(c *gin.Context) {
	type dnssecDiagnoseInput struct {
		Domain string `json:"domain"`
		Type   string `json:"type"`
	}

	var input dnssecDiagnoseInput
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	domain := strings.TrimSpace(input.Domain)
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain is required"})
		return
	}

	qtype := dns.TypeA
	if input.Type != "" {
		normalized := strings.ToUpper(strings.TrimSpace(input.Type))
		if t, ok := dns.StringToType[normalized]; ok {
			qtype = t
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported DNS type"})
			return
		}
	}

	diagnostic, err := api.DNSServer.DiagnoseDNSSEC(domain, qtype)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, diagnostic)
}
