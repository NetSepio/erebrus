package wg

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/NetSepio/erebrus/internal/store"
)

// PrivateKeyPlaceholder is emitted in client configs in place of the client's
// private key. The node never sees the client private key; the client (which
// generated the keypair) substitutes its own key locally.
const PrivateKeyPlaceholder = "REPLACE_WITH_PRIVATE_KEY"

var serverTpl = template.Must(template.New("server").
	Funcs(template.FuncMap{"join": strings.Join}).
	Parse(`# Erebrus wg0 — generated, do not edit by hand
[Interface]
Address = {{ .Address }}
ListenPort = {{ .ListenPort }}
PrivateKey = {{ .PrivateKey }}
{{- if .MTU }}
MTU = {{ .MTU }}
{{- end }}
{{- if .PreUp }}
PreUp = {{ .PreUp }}
{{- end }}
{{- if .PostUp }}
PostUp = {{ .PostUp }}
{{- end }}
{{- if .PreDown }}
PreDown = {{ .PreDown }}
{{- end }}
{{- if .PostDown }}
PostDown = {{ .PostDown }}
{{- end }}
{{ range .Peers }}{{ if .Enabled }}
# {{ .Name }} / {{ .Wallet }} / id={{ .ID }}
[Peer]
PublicKey = {{ .WGPublicKey }}
{{- if .WGPresharedKey }}
PresharedKey = {{ .WGPresharedKey }}
{{- end }}
AllowedIPs = {{ .WGAllowedIP }}
{{ end }}{{ end }}`))

var clientTpl = template.Must(template.New("client").Parse(`[Interface]
Address = {{ .Address }}
PrivateKey = ` + PrivateKeyPlaceholder + `
{{- if .DNS }}
DNS = {{ .DNS }}
{{- end }}

[Peer]
PublicKey = {{ .ServerPublicKey }}
{{- if .PresharedKey }}
PresharedKey = {{ .PresharedKey }}
{{- end }}
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = {{ .Endpoint }}
PersistentKeepalive = 16
`))

type serverTplData struct {
	Address    string
	ListenPort int
	PrivateKey string
	MTU        int
	PreUp      string
	PostUp     string
	PreDown    string
	PostDown   string
	Peers      []*store.Peer
}

type clientTplData struct {
	Address         string
	DNS             string
	ServerPublicKey string
	PresharedKey    string
	Endpoint        string
}

func renderServer(d serverTplData) ([]byte, error) {
	var buf bytes.Buffer
	if err := serverTpl.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderClient(d clientTplData) (string, error) {
	var buf bytes.Buffer
	if err := clientTpl.Execute(&buf, d); err != nil {
		return "", err
	}
	return buf.String(), nil
}
