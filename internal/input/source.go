// Package input 输入边界：参考工程 E/F 层（WebSocket/队列/鉴权）对本项目
// 收缩成一个接口——能进来一条用户消息即可。stdin、REPL、HTTP 都只是实现。
package input

import (
	"bufio"
	"context"
	"io"
	"os"
)

// Source 用户输入源
type Source interface {
	// Next 阻塞等待并返回下一条用户消息；输入结束返回 io.EOF。
	Next(ctx context.Context) (string, error)
}

// StdinSource 从标准输入按行读取（占位实现）。
type StdinSource struct {
	scanner *bufio.Scanner
}

func NewStdinSource() *StdinSource {
	return &StdinSource{scanner: bufio.NewScanner(os.Stdin)}
}

func (s *StdinSource) Next(ctx context.Context) (string, error) {
	if !s.scanner.Scan() {
		if err := s.scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return s.scanner.Text(), nil
}
