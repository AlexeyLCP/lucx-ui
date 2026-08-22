// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/awg/vpnuri"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// SubAwgService builds AmneziaWG client subscription bodies (.conf / vpn://)
// for a given subId, reusing BuildAwgClientConf.
type SubAwgService struct {
	SubService *SubService
}

func NewSubAwgService(sub *SubService) *SubAwgService {
	return &SubAwgService{SubService: sub}
}

// GetAwg returns the subscription body and Subscription-Userinfo header.
// format: "" or "conf" → plain .conf (multi-inbound separated by "# ---");
// "vpn" → vpn:// lines (one per conf). inboundId > 0 keeps only that inbound.
func (s *SubAwgService) GetAwg(subId, host, format string, inboundId int) (body, header string, err error) {
	subReq := s.SubService.ForRequest(host)
	subReq.subscriptionBody = true
	inbounds, err := subReq.getInboundsBySubId(subId)
	if err != nil {
		return "", "", err
	}

	var confs []string
	seenEmails := make(map[string]struct{})
	var lastBuildErr error
	for _, inbound := range inbounds {
		if inbound.Protocol != model.AWG || !awgInboundWanted(inbound.Id, inboundId) {
			continue
		}
		clients := subReq.matchingClients(inbound, subId)
		if len(clients) == 0 {
			continue
		}
		endpointHost := subReq.resolveInboundAddress(inbound)
		if endpointHost == "" {
			endpointHost = host
		}
		for i := range clients {
			client := clients[i]
			if !client.Enable {
				continue
			}
			seenEmails[client.Email] = struct{}{}
			conf, confErr := service.BuildAwgClientConf(inbound, &client, endpointHost)
			if confErr != nil {
				lastBuildErr = confErr
				continue
			}
			if strings.TrimSpace(conf) == "" {
				continue
			}
			// Label multi-inbound confs so operators can split them.
			if inbound.Remark != "" || inbound.Tag != "" {
				label := inbound.Remark
				if label == "" {
					label = inbound.Tag
				}
				conf = "# " + label + "\n" + conf
			}
			confs = append(confs, conf)
		}
	}

	if len(confs) == 0 {
		if lastBuildErr != nil {
			return "", "", lastBuildErr
		}
		return "", "", nil
	}

	emails := make([]string, 0, len(seenEmails))
	for e := range seenEmails {
		emails = append(emails, e)
	}
	traffic, _ := subReq.AggregateTrafficByEmails(emails)
	header = fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d",
		traffic.Up, traffic.Down, traffic.Total, traffic.ExpiryTime/1000)

	format = strings.ToLower(strings.TrimSpace(format))
	if format == "vpn" {
		var lines []string
		for _, conf := range confs {
			uri, uriErr := vpnuri.EncodeConf(conf)
			if uriErr != nil {
				// Fall back to conf block if encode fails for one entry.
				lines = append(lines, conf)
				continue
			}
			lines = append(lines, uri)
		}
		return strings.Join(lines, "\n"), header, nil
	}

	return strings.Join(confs, "\n\n# ---\n\n"), header, nil
}

func awgInboundWanted(id, filter int) bool {
	return filter <= 0 || id == filter
}
