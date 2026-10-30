package jsplugin

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignTencentTC3MatchesOfficialCanonicalRequest(t *testing.T) {
	// Official TC3 example from https://cloud.tencent.com/document/api/213/30654
	// The payload is the ASCII JSON with \u escapes, not the decoded Chinese.
	body := `{"Limit": 1, "Filters": [{"Values": ["\u672a\u547d\u540d"], "Name": "instance-name"}]}`
	payload := sha256.Sum256([]byte(body))
	assert.Equal(t, "35e9c5b0e3ae67532d3c9f17ead6c90222632e5b1ff7f6e89887f1398934f064", hex.EncodeToString(payload[:]))

	signed, err := signTencentTC3(tencentSignRequest{
		Method:    "POST",
		URL:       "https://cvm.tencentcloudapi.com",
		Headers:   map[string]string{"Content-Type": "application/json; charset=utf-8", "X-TC-Action": "DescribeInstances"},
		Body:      body,
		SecretId:  "AKID********************************",
		SecretKey: "********************************",
		Service:   "cvm",
		Timestamp: 1551113065,
	}, func() time.Time { return time.Unix(0, 0) })
	require.NoError(t, err)
	assert.Equal(t, "1551113065", signed["X-TC-Timestamp"])
	assert.Equal(t, "application/json; charset=utf-8", signed["Content-Type"])
	assert.Equal(t, "cvm.tencentcloudapi.com", signed["Host"])
	assert.Equal(t, "TC3-HMAC-SHA256 Credential=AKID********************************/2019-02-25/cvm/tc3_request, SignedHeaders=content-type;host;x-tc-action, Signature=10b1a37a7301a02ca19a647ad722d5e43b4b3cff309d421d85b46093f6ab6c4f", signed["Authorization"])
}

func TestSignTencentTC3RejectsEmptyURL(t *testing.T) {
	_, err := signTencentTC3(tencentSignRequest{SecretId: "id", SecretKey: "key"}, time.Now)
	require.Error(t, err)
}
