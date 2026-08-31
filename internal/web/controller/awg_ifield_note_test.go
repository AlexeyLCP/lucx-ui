package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
)

// 3596 chars = 3604 IBytes against a 3492 budget — the set from the field report.
const oversizeIFieldChars = 3596

func inboundSaveRouter(t *testing.T) *gin.Engine {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("I18n", func(_ locale.I18nType, key string, _ ...string) string { return key })
		session.SetAPIAuthUser(c, &model.User{Id: 1, Username: "test"})
		c.Next()
	})
	NewInboundController(router.Group("/panel/api/inbounds"))
	return router
}

func postInbound(t *testing.T, router *gin.Engine, path, body string) (bool, string, int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	var m struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
		Obj     struct {
			Id int `json:"id"`
		} `json:"obj"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal response: %v; body=%s", err, resp.Body.String())
	}
	return m.Success, m.Msg, m.Obj.Id
}

func awgInboundBody(t *testing.T, port int, i1, dns string) string {
	t.Helper()
	settings, err := json.Marshal(map[string]any{"clients": []any{}, "i1": i1, "dns": dns})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	body, err := json.Marshal(map[string]any{
		"tag": "in-51820-udp", "enable": true, "listen": "0.0.0.0", "port": port,
		"protocol": "awg", "streamSettings": `{"network":"udp"}`, "settings": string(settings),
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return string(body)
}

// entity.Msg carries no warning channel, so the note an over-budget I-set earns
// has to ride the success text of the very save that stored it.
func TestInboundSave_OversizeIFieldSetWarnsInTheSuccessMessage(t *testing.T) {
	router := inboundSaveRouter(t)
	oversize := strings.Repeat("x", oversizeIFieldChars)

	ok, msg, id := postInbound(t, router, "/panel/api/inbounds/add", awgInboundBody(t, 51820, oversize, "1.1.1.1"))
	if !ok {
		t.Fatalf("add(oversize I-set) success = false, msg = %q", msg)
	}
	for _, want := range []string{"pages.inbounds.form.awgIFieldBudget", "3604 > 3492"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("add message = %q, want it to carry %q", msg, want)
		}
	}

	ok, msg, _ = postInbound(t, router, "/panel/api/inbounds/update/"+strconv.Itoa(id), awgInboundBody(t, 51820, oversize, "9.9.9.9"))
	if !ok {
		t.Fatalf("update(oversize I-set) success = false, msg = %q", msg)
	}
	for _, want := range []string{"pages.inbounds.form.awgIFieldBudget", "3604 > 3492"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("update message = %q, want it to carry %q", msg, want)
		}
	}
}

// The note is about one set on one protocol; every other save must read exactly
// as it did before, or operators learn to skim the success toast.
func TestInboundSave_LeavesOrdinarySavesUntouched(t *testing.T) {
	router := inboundSaveRouter(t)

	ok, msg, _ := postInbound(t, router, "/panel/api/inbounds/add", awgInboundBody(t, 51821, strings.Repeat("x", 100), "1.1.1.1"))
	if !ok || msg != "pages.inbounds.toasts.inboundCreateSuccess" {
		t.Fatalf("add(within-budget AWG) success = %v, msg = %q, want the plain success text", ok, msg)
	}

	vless, err := json.Marshal(map[string]any{
		"tag": "in-51822-tcp", "enable": true, "listen": "0.0.0.0", "port": 51822,
		"protocol": "vless", "streamSettings": `{"network":"tcp","security":"none"}`,
		"settings": `{"clients":[],"decryption":"none","fallbacks":[]}`,
	})
	if err != nil {
		t.Fatalf("marshal vless body: %v", err)
	}
	ok, msg, _ = postInbound(t, router, "/panel/api/inbounds/add", string(vless))
	if !ok || msg != "pages.inbounds.toasts.inboundCreateSuccess" {
		t.Fatalf("add(vless) success = %v, msg = %q, want the plain success text", ok, msg)
	}
}
