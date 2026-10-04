package starter

import (
	"crypto/sha256"
	"encoding/base64"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestAppearanceBootstrapsInInitialHTMLWithCSPHashes(t *testing.T) {
	app := Application{Assets: os.DirFS("../../web")}
	response := app.handleManagement(ManagementRequest{Method: "GET", Path: "/v0/resource/plugins/" + PluginID + "/status"})
	if response.StatusCode != 200 {
		t.Fatal(response.StatusCode)
	}
	html := string(response.Body)
	policy := response.Headers.Get("Content-Security-Policy")
	if strings.Contains(policy, "unsafe-inline") {
		t.Fatal("CSP must not allow arbitrary inline code")
	}
	for _, tag := range []string{"script", "style"} {
		pattern := regexp.MustCompile("(?s)<" + tag + ">(.*?)</" + tag + ">")
		content := pattern.FindStringSubmatch(html)
		if len(content) != 2 {
			t.Fatalf("missing initial %s", tag)
		}
		digest := sha256.Sum256([]byte(content[1]))
		hash := "'sha256-" + base64.StdEncoding.EncodeToString(digest[:]) + "'"
		if !strings.Contains(policy, tag+"-src 'self' "+hash) {
			t.Fatalf("%s blocked by CSP", tag)
		}
	}
	bootstrap := strings.Index(html, "CPAWindowUI.applyAppearance(window);")
	if bootstrap < 0 || bootstrap > strings.Index(html, "<body>") || bootstrap > strings.Index(html, `href="./style.css"`) {
		t.Fatal("theme must initialize before external styles and visible content")
	}
	if strings.Contains(html, `id="theme"`) || strings.Contains(html, `id="language"`) {
		t.Fatal("plugin preferences must not be displayed")
	}
}
