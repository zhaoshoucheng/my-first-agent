package account

import (
	"context"
	"fmt"
	"sync"

	"github.com/shoucheng/my-first-agent/infra/config"
)

var (
	initOnce sync.Once
	defSvc   *Service
)

// Init 从全局配置里读 account 段，构造默认账号服务并存入包内单例。
func Init(ctx context.Context) {
	initOnce.Do(func() {
		account := config.GetConfig()
		if account == nil {
			panic("conf not init")
		}
		s, err := NewService(ctx, account.Account.Source)
		if err != nil {
			panic(fmt.Errorf("account.Init: %w", err))
		}
		defSvc = s
	})
	return
}

// Default 返回包内默认账号服务。Init 未成功时调用会 panic：
func Default() *Service {
	if defSvc == nil {
		panic("account: Default() called before successful Init()")
	}
	return defSvc
}
