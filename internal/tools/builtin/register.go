package builtin

import (
	"github.com/shoucheng/my-first-agent/internal/tools"
	"github.com/shoucheng/my-first-agent/internal/tools/sandbox"
)

func NewBuiltinRegistry(sb sandbox.Sandbox) (*tools.Registry, error) {
	registry := tools.NewRegistry()
	ts := []tools.Tool{
		NewCalculator(),
		// terminal
		NewShellExec(sb),
		NewShellView(sb),
		// file
		NewFileRead(sb),
		NewFileWriteText(sb),
		NewFileReplaceText(sb),
		NewFileAppendText(sb),
		// browser
		NewBrowserNavigate(sb),
		NewBrowserView(sb),
		NewBrowserClick(sb),
		NewBrowserInput(sb),
		NewBrowserScrollUp(sb),
		NewBrowserScrollDown(sb),
		// search
		NewOmniSearch(),
		// video
		NewGenerateVideo(),
	}
	for _, t := range ts {
		if err := registry.Register(t); err != nil {
			return nil, err
		}
	}
	return registry, nil
}
