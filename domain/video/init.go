package video

var (
	defSvc *Service
)

// Init 构造包内默认视频生成服务并存入单例。
func Init() {
	defSvc = NewService()
}

// Default 返回包内默认视频生成服务。Init 未成功时调用会 panic：
// 这是显式契约 — 默认服务必须在任何业务逻辑之前完成初始化。
func Default() *Service {
	if defSvc == nil {
		panic("video: Default() called before successful Init()")
	}
	return defSvc
}
