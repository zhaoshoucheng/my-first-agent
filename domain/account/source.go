package account

import "fmt"

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

// SourceConfig 账号数据源配置（对应 config.yaml 中的 llm.source 段）。
//
// 通过 NewLoader() 拿到对应的 Loader 实现；账号 Service 在初始化时调用一次
// 把所有账号读进内存。
type SourceConfig struct {
	Type SourceType       `yaml:"type" json:"type"`
	File FileSourceConfig `yaml:"file" json:"file"`
	DB   DBConfig         `yaml:"db"   json:"db"`
}

// NewLoader 根据 Type 选择具体的 Loader 实现。
func (c SourceConfig) NewLoader() (Loader, error) {
	switch c.Type {
	case SourceFile:
		if c.File.Dir == "" {
			return nil, fmt.Errorf("account.SourceConfig: source.file.dir is required when type=file")
		}
		return NewFileLoader(c.File.Dir), nil
	case SourceDB:
		return NewDBLoader(c.DB), nil
	case "":
		return nil, fmt.Errorf("account.SourceConfig: source.type is required (file|db)")
	default:
		return nil, fmt.Errorf("account.SourceConfig: source.type %q is not supported", c.Type)
	}
}
