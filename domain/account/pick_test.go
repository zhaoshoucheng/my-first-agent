package account

import "testing"

// TestPickAccountForModel_NameMatch 覆盖"同 Provider 多账号"的选择：按账号名与
// model 名的贴合度挑，避免选错（真实场景：azure-openai 同时有 gpt 与 sora 账号）。
func TestPickAccountForModel_NameMatch(t *testing.T) {
	cred := Credential{APIKey: "k", BaseURL: "https://x"}
	accs := []*Account{
		{Name: "gpt-5.1", Provider: ProviderAzureOpenAI, Credential: cred},
		{Name: "sora-2", Provider: ProviderAzureOpenAI, Credential: cred},
	}
	svc, err := newServiceFromAccounts(accs)
	if err != nil {
		t.Fatalf("newServiceFromAccounts: %v", err)
	}

	cases := []struct {
		model    string
		wantName string
	}{
		{"sora-2", "sora-2"},         // 完全相等
		{"sora-2-pro", "sora-2"},     // 账号名是 model 前缀
		{"gpt-5.1", "gpt-5.1"},       // 完全相等
		{"gpt-5.1-mini", "gpt-5.1"},  // 账号名是 model 前缀
	}
	for _, c := range cases {
		acc, err := svc.PickAccountForModel(c.model)
		if err != nil {
			t.Errorf("PickAccountForModel(%q): %v", c.model, err)
			continue
		}
		if acc.Name != c.wantName {
			t.Errorf("PickAccountForModel(%q)=%q want %q", c.model, acc.Name, c.wantName)
		}
	}
}

// TestPickAccountForModel_Single 单账号时无条件选中。
func TestPickAccountForModel_Single(t *testing.T) {
	accs := []*Account{
		{Name: "any-name", Provider: ProviderModelArk, Credential: Credential{APIKey: "k"}},
	}
	svc, _ := newServiceFromAccounts(accs)
	acc, err := svc.PickAccountForModel("seedance-1-0")
	if err != nil {
		t.Fatalf("PickAccountForModel: %v", err)
	}
	if acc.Name != "any-name" {
		t.Errorf("got %q want any-name", acc.Name)
	}
}
