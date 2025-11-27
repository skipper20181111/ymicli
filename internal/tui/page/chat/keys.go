package chat

import (
	"charm.land/bubbles/v2/key"
)

type KeyMap struct {
	NewSession    key.Binding
	AddAttachment key.Binding
	Cancel        key.Binding
	Tab           key.Binding
	Details       key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		NewSession: key.NewBinding(
			key.WithKeys("ctrl+n"),
			key.WithHelp("ctrl+n", "新建会话"),
		),
		AddAttachment: key.NewBinding(
			key.WithKeys("ctrl+f"),
			key.WithHelp("ctrl+f", "添加附件"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
			key.WithHelp("esc", "取消"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "切换焦点"),
		),
		Details: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "切换详情"),
		),
	}
}
