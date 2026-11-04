package upstreambalance

import (
	"context"
	"encoding/json"
	"strings"
	"unicode"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type walletCredential struct {
	WalletID string `json:"wallet_id"`
	Binding  string `json:"binding"`
	Token    string `json:"token"`
}

func (in *Input) validate() error {
	if err := in.Config.validate(); err != nil {
		return err
	}
	if in.AccessToken != nil {
		token := strings.TrimSpace(*in.AccessToken)
		if len(token) > 4096 || strings.IndexFunc(token, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
			return invalid("访问令牌格式无效，请仅填写令牌本身")
		}
		if token != "" && (in.Mode != "newapi" || in.NewAPIAuth != "account") {
			return invalid("账户访问令牌仅用于 New API 账户查询")
		}
		in.AccessToken = &token
	}
	return nil
}

// Bind to the wallet, selected account, destination and proxy route. Rotating a
// model API key at the same destination does not invalidate the account token.
func credentialBinding(a *service.Account) string {
	if a == nil {
		return ""
	}
	proxy := ""
	if a.Proxy != nil {
		proxy = a.Proxy.URL()
	}
	return fingerprint([]any{a.ID, a.Type, a.GetCredential("base_url"), a.ProxyID, proxy})
}

func accountCredential(cipher service.SecretEncryptor, w Wallet, a *service.Account) (string, error) {
	if cipher == nil {
		return "", probeError("credential_storage_unavailable")
	}
	if w.accessTokenEncrypted == "" {
		return "", probeError("credential_missing")
	}
	plain, err := cipher.Decrypt(w.accessTokenEncrypted)
	if err != nil {
		return "", probeError("credential_decryption_failed")
	}
	var saved walletCredential
	if json.Unmarshal([]byte(plain), &saved) != nil || saved.Token == "" || saved.WalletID != w.ID {
		return "", probeError("credential_decryption_failed")
	}
	if saved.Binding != credentialBinding(a) {
		return "", probeError("credential_source_changed")
	}
	return saved.Token, nil
}

func (s *Store) updateCredential(ctx context.Context, w *Wallet, in Input) error {
	if in.Mode != "newapi" || (in.AccessToken != nil && *in.AccessToken == "") {
		w.accessTokenEncrypted = ""
	}
	if in.Mode == "newapi" && in.NewAPIAuth == "account" {
		if s.Cipher == nil {
			return invalid("服务端尚未配置持久化加密密钥，不能保存或使用账户访问令牌")
		}
		if s.Accounts == nil || in.QueryAccountID == nil {
			return invalid("查询来源不可用")
		}
		a, err := s.Accounts.GetByID(ctx, *in.QueryAccountID)
		if err != nil || a == nil || a.ID != *in.QueryAccountID || a.Type != service.AccountTypeAPIKey {
			return invalid("查询来源不可用")
		}
		if in.AccessToken != nil && *in.AccessToken != "" {
			if _, err = balanceProbeURL(a.GetCredential("base_url"), s.ProbeConfig, "newapi_account"); err != nil {
				return invalid("上游地址不符合余额查询安全策略")
			}
			plain, _ := json.Marshal(walletCredential{WalletID: w.ID, Binding: credentialBinding(a), Token: *in.AccessToken})
			w.accessTokenEncrypted, err = s.Cipher.Encrypt(string(plain))
			if err != nil {
				return invalid("账户访问令牌加密失败，未保存")
			}
		}
		if _, err = accountCredential(s.Cipher, *w, a); err != nil {
			return invalid("账户访问令牌未配置、无法解密或查询来源已变化，请重新填写令牌")
		}
	}
	w.HasAccessToken = w.accessTokenEncrypted != ""
	return nil
}
