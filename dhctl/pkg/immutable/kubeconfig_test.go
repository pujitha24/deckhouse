package immutable

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The node hands back a certificate; the key it belongs to never travelled.
// Pairing them here is what turns a public document into credentials — and a
// node that returned a key would mean the channel carried a secret after all.
func TestWithClientKeyPairsTheCertificateWithTheKeyThatStayedHome(t *testing.T) {
	keyless := []byte(`apiVersion: v1
kind: Config
clusters:
- name: kubernetes
  cluster:
    server: https://10.0.0.1:6443
users:
- name: kubernetes-admin
  user:
    client-certificate-data: Y2VydA==
contexts:
- name: kubernetes-admin@kubernetes
  context:
    cluster: kubernetes
    user: kubernetes-admin
current-context: kubernetes-admin@kubernetes
`)

	completed, err := WithClientKey(keyless, "-----BEGIN EC PRIVATE KEY-----\ntest\n-----END EC PRIVATE KEY-----\n")
	require.NoError(t, err)
	require.Contains(t, string(completed), "client-key-data")

	// Refused rather than quietly accepted: a key coming back means the channel
	// is carrying something worth stealing.
	withKey := []byte(strings.Replace(string(keyless),
		"    client-certificate-data: Y2VydA==",
		"    client-certificate-data: Y2VydA==\n    client-key-data: a2V5", 1))
	_, err = WithClientKey(withKey, "-----BEGIN EC PRIVATE KEY-----\ntest\n-----END EC PRIVATE KEY-----\n")
	require.Error(t, err)
	require.Contains(t, err.Error(), "must never carry")
}
