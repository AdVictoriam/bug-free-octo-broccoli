package studio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var videoIDRE = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func YouTubeID(raw string) (string, error) {
	if videoIDRE.MatchString(raw) {
		return raw, nil
	}
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || u.User != nil || u.Port() != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", errors.New("provide a valid YouTube URL")
	}
	host := strings.ToLower(u.Hostname())
	var id string
	switch host {
	case "youtu.be":
		id = strings.Trim(u.Path, "/")
	case "youtube.com", "www.youtube.com", "m.youtube.com":
		if u.Path == "/watch" {
			id = u.Query().Get("v")
		} else {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "embed" || parts[0] == "live") {
				id = parts[1]
			}
		}
	default:
		return "", errors.New("only youtube.com and youtu.be links are accepted")
	}
	if !videoIDRE.MatchString(id) {
		return "", errors.New("invalid YouTube video ID")
	}
	return id, nil
}

var tagsRE = regexp.MustCompile(`<[^>]+>`)
var stampRE = regexp.MustCompile(`^\d{1,2}:\d{2}(?::\d{2})?[.,]\d{3}\s*-->`)
var captionNumberRE = regexp.MustCompile(`^\d+$`)

func NormalizeTranscript(raw string) string {
	raw = strings.TrimPrefix(raw, "\ufeff")
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	out := []string{}
	stamp := ""
	last := ""
	skip := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			skip = false
			continue
		}
		if strings.HasPrefix(line, "WEBVTT") || strings.HasPrefix(line, "Kind:") || strings.HasPrefix(line, "Language:") {
			continue
		}
		if strings.HasPrefix(line, "NOTE") || line == "STYLE" || line == "REGION" {
			skip = true
			continue
		}
		if skip {
			continue
		}
		if stampRE.MatchString(line) {
			stamp = strings.TrimSpace(strings.Split(line, "-->")[0])
			continue
		}
		if captionNumberRE.MatchString(line) {
			continue
		}
		line = html.UnescapeString(tagsRE.ReplaceAllString(line, ""))
		if line == last {
			continue
		}
		last = line
		if stamp != "" {
			out = append(out, "["+stamp+"] "+line)
			stamp = ""
		} else {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
func allowedOfficial(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	if !contains(OfficialDomains, strings.ToLower(u.Hostname())) {
		return false
	}
	if u.Hostname() == "discord.com" && !strings.HasPrefix(u.Path, "/developers/") && !strings.HasPrefix(u.Path, "/safety/") {
		return false
	}
	return true
}
func publicIP(a netip.Addr) bool {
	return a.IsValid() && a.IsGlobalUnicast() && !a.IsPrivate() && !a.IsLoopback() && !a.IsLinkLocalUnicast() && !a.IsMulticast() && !a.IsUnspecified() && !a.Is4In6()
}
func officialClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	tr := &http.Transport{TLSHandshakeTimeout: 10 * time.Second, MaxConnsPerHost: 2, IdleConnTimeout: 20 * time.Second}
	tr.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		if port != "443" {
			return nil, errors.New("non-HTTPS destination blocked")
		}
		ips, e := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if e != nil {
			return nil, e
		}
		for _, ip := range ips {
			if !publicIP(ip) {
				return nil, errors.New("non-public destination blocked")
			}
		}
		for _, ip := range ips {
			conn, e := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if e == nil {
				return conn, nil
			}
		}
		return nil, errors.New("official source connection failed")
	}
	return &http.Client{Transport: tr, Timeout: 25 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || !allowedOfficial(req.URL.String()) {
			return errors.New("off-allowlist redirect blocked")
		}
		return nil
	}}
}

var scriptStyleRE = regexp.MustCompile(`(?is)<(script|style|nav|header|footer)\b[^>]*>.*?</(?:script|style|nav|header|footer)>`)
var spaceRE = regexp.MustCompile(`\s+`)

func htmlText(raw string) string {
	return strings.TrimSpace(spaceRE.ReplaceAllString(html.UnescapeString(tagsRE.ReplaceAllString(scriptStyleRE.ReplaceAllString(raw, " "), " ")), " "))
}
func FetchEvidence(ctx context.Context, raw string) EvidencePage {
	page := EvidencePage{URL: raw, RetrievedAt: now()}
	if !allowedOfficial(raw) {
		page.Error = "non-official URL blocked"
		return page
	}
	req, e := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if e != nil {
		page.Error = "invalid official URL"
		return page
	}
	req.Header.Set("User-Agent", "BoltyStudio/1.0 (source verification)")
	resp, e := officialClient().Do(req)
	if e != nil {
		page.Error = "official page could not be fetched; verdict must remain unknown"
		return page
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		page.Error = fmt.Sprintf("source HTTP %d", resp.StatusCode)
		return page
	}
	media := resp.Header.Get("Content-Type")
	if !strings.Contains(media, "text/") && !strings.Contains(media, "json") {
		page.Error = "source must be HTML/text/JSON; PDF/image sources require manual review"
		return page
	}
	data, e := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if e != nil {
		page.Error = "source read failed"
		return page
	}
	if len(data) >= 2<<20 {
		page.Error = "source larger than 2 MiB limit"
		return page
	}
	text := htmlText(string(data))
	if len(text) < 80 {
		page.Error = "source has insufficient readable text"
		return page
	}
	if len(text) > 24000 {
		text = text[:24000]
	}
	page.Text = text
	page.Hash = hash(text)
	return page
}

type YouTubeCredentials struct{ ClientID, ClientSecret, RefreshToken string }

func (y YouTubeCredentials) Ready() bool {
	return y.ClientID != "" && y.ClientSecret != "" && y.RefreshToken != ""
}
func (y YouTubeCredentials) Transcript(ctx context.Context, videoID string) (string, error) {
	if !y.Ready() {
		return "", errors.New("YouTube owner OAuth is not configured; upload or paste an authorised transcript instead")
	}
	if !videoIDRE.MatchString(videoID) {
		return "", errors.New("invalid video ID")
	}
	client := &http.Client{Timeout: 40 * time.Second}
	form := url.Values{"client_id": {y.ClientID}, "client_secret": {y.ClientSecret}, "refresh_token": {y.RefreshToken}, "grant_type": {"refresh_token"}}
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, e := client.Do(req)
	if e != nil {
		return "", errors.New("YouTube OAuth refresh failed")
	}
	data, e := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	if e != nil || resp.StatusCode != 200 {
		return "", errors.New("YouTube OAuth credentials could not be refreshed")
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(data, &token) != nil || token.AccessToken == "" {
		return "", errors.New("missing OAuth access token")
	}
	get := func(endpoint string) ([]byte, error) {
		req, _ := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		resp, e := client.Do(req)
		if e != nil {
			return nil, errors.New("YouTube request failed")
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("YouTube HTTP %d: the account must have caption access to this video; use transcript upload for other references", resp.StatusCode)
		}
		return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	}
	data, e = get("https://www.googleapis.com/youtube/v3/captions?part=snippet&videoId=" + url.QueryEscape(videoID))
	if e != nil {
		return "", e
	}
	var list struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Language string `json:"language"`
				Status   string `json:"status"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if e = json.Unmarshal(data, &list); e != nil {
		return "", e
	}
	if len(list.Items) == 0 {
		return "", errors.New("no accessible caption track; paste or upload a transcript")
	}
	id := list.Items[0].ID
	for _, item := range list.Items {
		if strings.HasPrefix(item.Snippet.Language, "en") && item.Snippet.Status == "serving" {
			id = item.ID
			break
		}
	}
	data, e = get("https://www.googleapis.com/youtube/v3/captions/" + url.PathEscape(id) + "?tfmt=vtt")
	if e != nil {
		return "", e
	}
	return NormalizeTranscript(string(data)), nil
}
