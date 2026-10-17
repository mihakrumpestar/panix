package tui

import (
	"github.com/mihakrumpestar/panix/internal/workflow"
	"github.com/mihakrumpestar/panix/pkg/tui/zeroterm"
)

func (m *model) Init() []zeroterm.Cmd {
	if m.isSnapshot {
		m.conf.Fleet.RecalculateCachesOnly(m.conf.Phases)

		return nil
	}

	w, err := workflow.NewWorkflow(m.ctx, m.conf)
	if err != nil {
		return []zeroterm.Cmd{func() zeroterm.Msg { return errMsg{err: err} }}
	}

	m.workflow = w

	return []zeroterm.Cmd{
		workflowRunCmd(m.workflow),
		workflowUpdateHookCmd(m.workflow, m.lastWorkflowUpdate),
	}
}

//nolint:cyclop
func (m *model) Update(msg zeroterm.Msg) zeroterm.Cmd {
	var cmd []zeroterm.Cmd

	// Keep the footer keymap on the modal context that owns the keyboard before
	// any dispatch can read it.
	m.syncFooterKeyDefs()

	// Open modal overlays capture input first, so global keybinds, mouse clicks
	// and viewport scrolling never fire underneath them. A captured key can still
	// carry a cmd (the picker quitting keybinds), which runs instead of dispatch.
	if handled, modalCmd := m.captureModalInput(msg); handled {
		return modalCmd
	}

	if m.workflow != nil || m.isSnapshot {
		cmd = append(cmd, m.viewports.Update(msg))

		if m.viewports.ConsumeDirty() {
			m.cachedTree.InvalidateCache()
		}

		cmd = append(cmd, m.footer.Update(msg))
		cmd = append(cmd, m.spinners.Update(msg))
	}

	switch msg := msg.(type) {
	case errMsg:
		cmd = append(cmd, m.handleErrMsgCmd(msg))

		return zeroterm.BatchCmd(cmd...)
	case workflowDoneMsg:
		if msg.workflow != m.workflow {
			return nil
		}

		cmd = append(cmd, m.workflowDoneMsgCmd(msg))

		return zeroterm.BatchCmd(cmd...)
	case restartMsg:
		cmd = append(cmd, m.workflowRestartCmd())
	case retryMsg:
		m.workflowRetry()
	case workflowUpdateHookMsg:
		if msg.workflow == m.workflow {
			m.lastWorkflowUpdate = msg.lastUpdate
			cmd = append(cmd, workflowUpdateHookCmd(m.workflow, m.lastWorkflowUpdate))
		}
	case zeroterm.PostRenderMsg:
		cmd = append(cmd, m.spinners.ProcessPendingTicks())
	case zeroterm.KeyPressMsg:
		cmd = append(cmd, m.HandleKeyInput(msg))
	case zeroterm.MouseClickMsg:
		m.handleMouseClick(msg)
	case zeroterm.WindowSizeMsg:
		m.dimensions.Width = msg.Width

		m.dimensions.Height = msg.Height
		if m.themePicker.open {
			m.themePickerEnsureVisible(m.themeNames())
		}

		m.cachedTree.InvalidateCache()
	}

	return zeroterm.BatchCmd(cmd...)
}
