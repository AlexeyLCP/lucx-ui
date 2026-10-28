package service

import (
	"net/url"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"gorm.io/gorm"
)

// ExternalLinkInput is one row from the client form's Links tab.
type ExternalLinkInput struct {
	Kind       string `json:"kind"`
	Value      string `json:"value"`
	Remark     string `json:"remark"`
	Enable     *bool  `json:"enable"`
	ExpiryTime int64  `json:"expiryTime"`
	NamePrefix string `json:"namePrefix"`
}

func (s *ClientService) GetExternalLinksForRecord(id int) ([]model.ClientExternalLink, error) {
	var rows []model.ClientExternalLink
	if err := database.GetDB().
		Where("client_id = ?", id).
		Order("sort_index ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// normalizeExternalLinks validates and orders the incoming rows. A "link" must
// parse to a supported share-link scheme; a "subscription" must be an http(s)
// URL. Blank values are dropped; an invalid value is a hard error so the
// operator gets immediate feedback instead of a silently missing config.
func normalizeExternalLinks(inputs []ExternalLinkInput) ([]model.ClientExternalLink, error) {
	out := make([]model.ClientExternalLink, 0, len(inputs))
	for _, in := range inputs {
		value := strings.TrimSpace(in.Value)
		if value == "" {
			continue
		}
		kind := strings.TrimSpace(in.Kind)
		switch kind {
		case model.ExternalLinkKindSubscription:
			if !isHTTPURL(value) {
				return nil, common.NewError("external subscription must be an http(s) URL: " + value)
			}
		case model.ExternalLinkKindLink, "":
			kind = model.ExternalLinkKindLink
			if !isShareURI(value) {
				return nil, common.NewError("external link must be a URI with a scheme (scheme://value): " + value)
			}
		default:
			return nil, common.NewError("unknown external link kind: " + kind)
		}
		if in.ExpiryTime < 0 {
			return nil, common.NewError("external link expiryTime must be 0 (never) or a future unix millisecond timestamp: " + value)
		}
		enable := true
		if in.Enable != nil {
			enable = *in.Enable
		}
		out = append(out, model.ClientExternalLink{
			Kind:       kind,
			Value:      value,
			Remark:     strings.TrimSpace(in.Remark),
			Enable:     &enable,
			ExpiryTime: in.ExpiryTime,
			NamePrefix: in.NamePrefix,
			SortIndex:  len(out),
		})
	}
	return out, nil
}

func isHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// LUCX-HOOK: any scheme is accepted for external client links (issue #145),
// not only the schemes the panel can parse — client apps know protocols
// LucX does not (openflux, snolc, ...). Validation stays at "has a scheme
// and a non-blank payload"; unparsable values degrade gracefully downstream.
func isShareURI(value string) bool {
	colon := strings.IndexByte(value, ':')
	if colon < 1 || !validScheme(value[:colon]) {
		return false
	}
	rest := strings.TrimLeft(value[colon+1:], "/")
	return strings.TrimSpace(rest) != ""
}

func validScheme(scheme string) bool {
	for i := 0; i < len(scheme); i++ {
		c := scheme[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case i > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'):
		default:
			return false
		}
	}
	return true
}

// SetExternalLinksForRecord replaces a client's entire external-link set.
func (s *ClientService) SetExternalLinksForRecord(id int, inputs []ExternalLinkInput) error {
	rows, err := normalizeExternalLinks(inputs)
	if err != nil {
		return err
	}
	db := database.GetDB()
	return db.Transaction(func(tx *gorm.DB) error {
		var existing []model.ClientExternalLink
		if err := tx.Where("client_id = ?", id).Find(&existing).Error; err != nil {
			return err
		}
		byKindValue := make(map[string]model.ClientExternalLink, len(existing))
		for _, row := range existing {
			key := row.Kind + "\x00" + row.Value
			if _, ok := byKindValue[key]; !ok {
				byKindValue[key] = row
			}
		}
		if err := tx.Where("client_id = ?", id).Delete(&model.ClientExternalLink{}).Error; err != nil {
			return err
		}
		for i := range rows {
			if old, ok := byKindValue[rows[i].Kind+"\x00"+rows[i].Value]; ok {
				rows[i].LastFetchAt = old.LastFetchAt
				rows[i].LastFetchError = old.LastFetchError
			}
			rows[i].ClientId = id
			if err := tx.Create(&rows[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *ClientService) SetExternalLinksByEmail(email string, inputs []ExternalLinkInput) error {
	if strings.TrimSpace(email) == "" {
		return common.NewError("client email is required")
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return err
	}
	return s.SetExternalLinksForRecord(rec.Id, inputs)
}
