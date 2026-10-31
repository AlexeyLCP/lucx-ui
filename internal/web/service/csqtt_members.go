// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

const csqttMemberWebPort = 46002

type csqttMemberServerClient struct {
	Password string `json:"password"`
	Name     string `json:"name"`
	Active   bool   `json:"active"`
	Link     string `json:"link"`
}

type csqttMemberClientRequest struct {
	Name string `json:"name"`
	Days int    `json:"days"`
	Hash string `json:"hash"`
}

type csqttMemberCreated struct {
	Password string `json:"password"`
	Link     string `json:"link"`
}

type CsqttMemberService struct {
	mu sync.Mutex
}

var csqttMemberSync CsqttMemberService

func GetCsqttMemberService() *CsqttMemberService { return &csqttMemberSync }

// TriggerCsqttMemberSync refreshes members after a committed panel mutation.
// The periodic job remains the recovery path when the CSQTT API is unavailable.
func TriggerCsqttMemberSync() {
	common.GoRecover("csqtt-member-sync", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := GetCsqttMemberService().SyncAll(ctx); err != nil {
			logger.Warning("csqtt members immediate sync:", err)
		}
	})
}

func (s *CsqttMemberService) SyncAll(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var inbounds []*model.Inbound
	if err := database.GetDB().Where("protocol = ?", model.Csqtt).Find(&inbounds).Error; err != nil {
		return err
	}
	var firstErr error
	for _, inbound := range inbounds {
		if err := s.syncInbound(ctx, inbound); err != nil {
			logger.Warning("csqtt members: inbound", inbound.Id, "sync failed:", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (s *CsqttMemberService) Link(inboundID int, email string) (string, bool) {
	var member model.CsqttMember
	err := database.GetDB().Where("inbound_id = ? AND client_email = ?", inboundID, email).First(&member).Error
	return member.Link, err == nil && strings.TrimSpace(member.Link) != ""
}

func (s *CsqttMemberService) syncInbound(ctx context.Context, inbound *model.Inbound) error {
	cfg, ok := tunnel.CsqttConfigFromInbound(inbound)
	if !ok || strings.TrimSpace(cfg.WebPass) == "" || strings.TrimSpace(cfg.Password) == "" {
		return errors.New("CSQTT web password or shared password is empty")
	}
	var clients []model.Client
	if inbound.Enable {
		var err error
		clients, err = (&ClientService{}).ListForInbound(nil, inbound.Id)
		if err != nil {
			return err
		}
	}
	api, err := newCsqttMemberAPI(inbound, cfg)
	if err != nil {
		return err
	}
	serverClients, err := api.list(ctx)
	if err != nil {
		return err
	}
	byName := make(map[string]csqttMemberServerClient, len(serverClients))
	for _, client := range serverClients {
		byName[client.Name] = client
	}
	desired := make(map[string]struct{}, len(clients))
	for _, client := range clients {
		if strings.TrimSpace(client.Email) == "" {
			continue
		}
		desired[client.Email] = struct{}{}
		wantActive := client.Enable && (client.ExpiryTime == 0 || client.ExpiryTime > time.Now().UnixMilli())
		if wantActive && client.TotalGB > 0 {
			if traffic, trafficErr := (&InboundService{}).GetClientTrafficByEmail(client.Email); trafficErr == nil && traffic != nil {
				wantActive = traffic.Up+traffic.Down < client.TotalGB
			}
		}
		member, managed := s.managedMember(inbound.Id, client.Email)
		server, exists := byName[client.Email]
		if !exists {
			server, err = api.create(ctx, client.Email, cfg.VkHashes)
			if err != nil {
				return err
			}
			if server.Password == "" {
				return fmt.Errorf("CSQTT returned an empty member password for %q", client.Email)
			}
			managed = true
		}
		if managed && member.Password != "" && member.Password != server.Password {
			managed = false
			if err := database.GetDB().Delete(&model.CsqttMember{}, member.Id).Error; err != nil {
				return err
			}
		}
		if !managed {
			continue
		}
		if server.Active != wantActive {
			if err := api.toggle(ctx, server.Password); err != nil {
				return err
			}
			server.Active = wantActive
		}
		row := &model.CsqttMember{InboundId: inbound.Id, ClientEmail: client.Email, Password: server.Password, Link: server.Link}
		if err := database.GetDB().Where("inbound_id = ? AND client_email = ?", inbound.Id, client.Email).Assign(row).FirstOrCreate(row).Error; err != nil {
			return err
		}
	}
	var managed []model.CsqttMember
	if err := database.GetDB().Where("inbound_id = ?", inbound.Id).Find(&managed).Error; err != nil {
		return err
	}
	for _, member := range managed {
		if _, ok := desired[member.ClientEmail]; ok {
			continue
		}
		if server, ok := byName[member.ClientEmail]; ok && server.Password == member.Password {
			if err := api.delete(ctx, member.Password); err != nil {
				return err
			}
		}
		if err := database.GetDB().Delete(&model.CsqttMember{}, member.Id).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *CsqttMemberService) managedMember(inboundID int, email string) (model.CsqttMember, bool) {
	var member model.CsqttMember
	err := database.GetDB().Where("inbound_id = ? AND client_email = ?", inboundID, email).First(&member).Error
	return member, err == nil
}

type csqttMemberAPI struct {
	base   string
	key    string
	host   string
	client *http.Client
}

func newCsqttMemberAPI(inbound *model.Inbound, cfg tunnel.CsqttConfig) (*csqttMemberAPI, error) {
	host := strings.TrimSpace(cfg.SubHost)
	if inbound.NodeID == nil {
		host = "127.0.0.1"
	}
	if host == "" {
		return nil, errors.New("CSQTT public host is empty")
	}
	if parsed, err := url.Parse("https://" + host); err != nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("invalid CSQTT host %q", host)
	}
	publicHost := strings.TrimSpace(cfg.SubHost)
	if h, _, splitErr := net.SplitHostPort(publicHost); splitErr == nil {
		publicHost = h
	}
	return &csqttMemberAPI{
		base: "https://" + host + ":" + strconv.Itoa(csqttMemberWebPort),
		key:  cfg.WebPass,
		host: publicHost,
		client: &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // CSQTT uses a self-signed panel certificate.
		}},
	}, nil
}

func (a *csqttMemberAPI) do(ctx context.Context, method, path string, in, out any) error {
	var body *strings.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = strings.NewReader(string(raw))
	} else {
		body = strings.NewReader("")
	}
	req, err := http.NewRequestWithContext(ctx, method, a.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", a.key)
	if a.host != "" {
		req.Host = a.host
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("CSQTT API %s %s returned HTTP %d", method, path, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (a *csqttMemberAPI) list(ctx context.Context) ([]csqttMemberServerClient, error) {
	var out []csqttMemberServerClient
	err := a.do(ctx, http.MethodGet, "/api/clients", nil, &out)
	return out, err
}

func (a *csqttMemberAPI) create(ctx context.Context, name, hashes string) (csqttMemberServerClient, error) {
	var out csqttMemberCreated
	err := a.do(ctx, http.MethodPost, "/api/clients", csqttMemberClientRequest{Name: name, Days: 0, Hash: hashes}, &out)
	return csqttMemberServerClient{Password: out.Password, Name: name, Active: true, Link: out.Link}, err
}

func (a *csqttMemberAPI) toggle(ctx context.Context, password string) error {
	return a.do(ctx, http.MethodPost, "/api/clients/"+url.PathEscape(password)+"/toggle", nil, nil)
}

func (a *csqttMemberAPI) delete(ctx context.Context, password string) error {
	return a.do(ctx, http.MethodDelete, "/api/clients/"+url.PathEscape(password), nil, nil)
}
