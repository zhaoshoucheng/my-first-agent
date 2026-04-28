package llms

import (
	"encoding/base64"
	"encoding/json"
)

const providerScopedSignatureFormat = "llm_router_signature"

// SignatureScope identifies the backend path a replay signature belongs to.
type SignatureScope struct {
	Provider string `json:"provider"`
	APIType  string `json:"api_type"`
}

func (s SignatureScope) Valid() bool {
	return s.Provider != "" && s.APIType != ""
}

type providerScopedSignatureEnvelope struct {
	Format    string `json:"format"`
	Provider  string `json:"provider"`
	APIType   string `json:"api_type"`
	Signature string `json:"signature"`
}

// WrapScopedSignature adds router-side provenance to a provider-native signature.
// When no scope is provided, it returns the signature unchanged.
func WrapScopedSignature(signature string, scope *SignatureScope) string {
	if signature == "" || scope == nil || !scope.Valid() {
		return signature
	}
	payload, err := json.Marshal(providerScopedSignatureEnvelope{
		Format:    providerScopedSignatureFormat,
		Provider:  scope.Provider,
		APIType:   scope.APIType,
		Signature: signature,
	})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

// UnwrapScopedSignature returns the provider-native signature when it matches the selected scope.
// When no scope is provided, it returns the signature unchanged.
func UnwrapScopedSignature(signature string, scope *SignatureScope) (string, bool) {
	if signature == "" {
		return "", false
	}
	if scope == nil || !scope.Valid() {
		return signature, true
	}
	data, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return "", false
	}
	var envelope providerScopedSignatureEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", false
	}
	if envelope.Format != providerScopedSignatureFormat {
		return "", false
	}
	if envelope.Provider != scope.Provider || envelope.APIType != scope.APIType || envelope.Signature == "" {
		return "", false
	}
	return envelope.Signature, true
}

// ScopedSignatureMatches reports whether the signature belongs to the selected scope.
func ScopedSignatureMatches(signature string, scope *SignatureScope) bool {
	if signature == "" {
		return true
	}
	_, ok := UnwrapScopedSignature(signature, scope)
	return ok
}
