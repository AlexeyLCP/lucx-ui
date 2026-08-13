package service

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel" // LUCX-HOOK: Naive service-credential share link
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

type SubLinkProvider interface {
	SubLinksForSubId(host, subId string) ([]string, error)
	LinksForClient(host string, inbound *model.Inbound, email string) []string
	LinksForInbounds(host string, inbounds []*model.Inbound) []string
}

var registeredSubLinkProvider SubLinkProvider

func RegisterSubLinkProvider(p SubLinkProvider) {
	registeredSubLinkProvider = p
}

func (s *InboundService) GetSubLinks(host, subId string) ([]string, error) {
	if registeredSubLinkProvider == nil {
		return nil, common.NewError("sub link provider not registered")
	}
	return registeredSubLinkProvider.SubLinksForSubId(host, subId)
}

func (s *InboundService) GetAllInboundLinks(host string, userId int) ([]string, error) {
	if registeredSubLinkProvider == nil {
		return nil, common.NewError("sub link provider not registered")
	}
	inbounds, err := s.GetInbounds(userId)
	if err != nil {
		return nil, err
	}
	return registeredSubLinkProvider.LinksForInbounds(host, inbounds), nil
}

// LUCX-HOOK: Naive — per-inbound share links for the panel export button.
// Naive credentials are HMAC-derived from the panel secret, so the frontend
// cannot render them; the service-level link (authUser/authPass) mirrors the
// legacy Tunnels-page clientUrl, per-client links come from the sub engine.
func (s *InboundService) GetInboundLinks(host string, inboundId int) ([]string, error) {
	inbound, err := s.GetInbound(inboundId)
	if err != nil {
		return nil, err
	}
	var links []string
	if inbound.Protocol == model.Naive {
		if cfg, ok := tunnel.ConfigFromInbound(inbound); ok && !cfg.UseRawConfig {
			if u := cfg.ClientURL(); u != "" {
				links = append(links, u)
			}
		}
	}
	if registeredSubLinkProvider == nil {
		return links, nil
	}
	return append(links, registeredSubLinkProvider.LinksForInbounds(host, []*model.Inbound{inbound})...), nil
}

// END LUCX-HOOK

func (s *InboundService) GetAllClientLinks(host string, email string) ([]string, error) {
	if email == "" {
		return nil, common.NewError("client email is required")
	}
	if registeredSubLinkProvider == nil {
		return nil, common.NewError("sub link provider not registered")
	}
	rec, err := s.clientService.GetRecordByEmail(nil, email)
	if err != nil {
		return nil, err
	}
	inboundIds, err := s.clientService.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		return nil, err
	}
	var links []string
	for _, ibId := range inboundIds {
		inbound, getErr := s.GetInbound(ibId)
		if getErr != nil {
			return nil, getErr
		}
		links = append(links, registeredSubLinkProvider.LinksForClient(host, inbound, email)...)
	}
	return links, nil
}
