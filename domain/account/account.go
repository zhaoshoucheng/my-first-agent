package account

import (
	"errors"
)

// Account 账号实体。
type Account struct {
	Name       string     `json:"name"`
	Provider   Provider   `json:"provider"`
	Credential Credential `json:"credential"`
}

// 校验错误。Provider 维度的字段校验放在 LLM 工厂里做（拿到具体 SDK 报错更清晰）。
var (
	ErrEmptyName       = errors.New("account name is required")
	ErrInvalidName     = errors.New("account name must match ^[a-zA-Z0-9_@-]+$")
	ErrUnknownProvider = errors.New("unknown provider")
	ErrMissingAPIKey   = errors.New("credential.api_key is required")
)

// Validate 基本字段校验。
func (a *Account) Validate() error {
	if a.Name == "" {
		return ErrEmptyName
	}
	if !a.Provider.IsValid() {
		return ErrUnknownProvider
	}
	if a.Credential.APIKey == "" {
		return ErrMissingAPIKey
	}
	return nil
}
