package config

// Settings 是 config/config.yaml 的强类型镜像。
//
// 设计上和 Section() 是两条平行入口，互不冲突：
//   - 各 domain 模块仍可继续用 config.Section("xxx", &local) 自洽地读自己的段；
//   - 想要一次拿到所有配置（例如 admin / debug 工具、启动时打印 effective
//     config），可以直接 GetConfig() 拿一份只读快照。
//
// 注意：infra/config 不能反向依赖任何 domain/* 包，所以这里的字段类型必须是
// 本地定义的镜像结构，不能复用 domain.account.Config 等类型。
type Settings struct {
	Account AccountSettings `yaml:"account" json:"account"`
	LLM     LLMSettings     `yaml:"llm"     json:"llm"`
	Video   VideoSettings   `yaml:"video"   json:"video"`
	Agent   AgentSettings   `yaml:"agent"   json:"agent"`
	Memory  MemorySettings  `yaml:"memory"  json:"memory"`
	Logging LoggingSettings `yaml:"logging" json:"logging"`
}

// AccountSettings 对应 config.yaml 中的 account 段。
type AccountSettings struct {
	Source SourceConfig `yaml:"source" json:"source"`
}

// SourceType 账号数据源类型。
type SourceType string

const (
	SourceFile SourceType = "file" // 从一个文件夹读取所有 *.json 账号
	SourceDB   SourceType = "db"   // 从数据库读取（暂未实现）
)

// FileSourceConfig 文件源配置。
type FileSourceConfig struct {
	Dir string `yaml:"dir" json:"dir"`
}

// DBConfig 数据库账号源配置。第一版未实现，仅占位。
type DBConfig struct {
	Driver   string `yaml:"driver"   json:"driver"`
	DSN      string `yaml:"dsn"      json:"dsn"`
	Table    string `yaml:"table"    json:"table"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
}

// SourceConfig 账号数据源配置（对应 config.yaml 中的 llm.source 段）。
//
// 通过 NewLoader() 拿到对应的 Loader 实现；账号 Service 在初始化时调用一次
// 把所有账号读进内存。
type SourceConfig struct {
	Type SourceType       `yaml:"type" json:"type"`
	File FileSourceConfig `yaml:"file" json:"file"`
	DB   DBConfig         `yaml:"db"   json:"db"`
}

// AccountFileSource 文件源配置。
type AccountFileSource struct {
	Dir string `yaml:"dir" json:"dir"`
}

// AccountDBSource 数据库源配置（占位）。
type AccountDBSource struct {
	Driver   string `yaml:"driver"             json:"driver"`
	DSN      string `yaml:"dsn"                json:"dsn"`
	Table    string `yaml:"table"              json:"table"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
}

// LLMSettings 对应 config.yaml 中的 llm 段。当前没有必填字段，留空结构占位。
type LLMSettings struct{}

// VideoSettings 对应 config.yaml 中的 video 段，是视频生成模块的运行期参数。
type VideoSettings struct {
	PollIntervalS int    `yaml:"poll_interval_s" json:"poll_interval_s"` // 轮询任务状态的间隔（秒）
	DefaultModel  string `yaml:"default_model"   json:"default_model"`   // 未显式指定时用的默认模型
	OutputDir     string `yaml:"output_dir"      json:"output_dir"`      // 生成视频落盘目录
}

// AgentSettings 对应 config.yaml 中的 agent 段。
type AgentSettings struct {
	MaxIterations int    `yaml:"max_iterations" json:"max_iterations"`
	Verbose       bool   `yaml:"verbose"        json:"verbose"`
	Type          string `yaml:"type"           json:"type"` // react | zero-shot | plan-and-execute
}

// MemorySettings 对应 config.yaml 中的 memory 段。
type MemorySettings struct {
	Type    string `yaml:"type"     json:"type"` // buffer | summary | vector
	MaxSize int    `yaml:"max_size" json:"max_size"`
}

// LoggingSettings 对应 config.yaml 中的 logging 段。
type LoggingSettings struct {
	Level  string `yaml:"level"  json:"level"`  // debug | info | warn | error
	Format string `yaml:"format" json:"format"` // json | text
}
