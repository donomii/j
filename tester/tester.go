package tester

import (
	"fmt"
	"os/exec"

	"github.com/donomii/j/types"
	"github.com/donomii/j/workspace"
)

type PlaywrightTester struct {
	workspaceRoot types.WorkspaceRoot
}

func New(workspaceRoot types.WorkspaceRoot) *PlaywrightTester {
	return &PlaywrightTester{workspaceRoot: workspaceRoot}
}

func (t *PlaywrightTester) RunTest(targetURL, screenshotPath string) (string, error) {
	resolvedScreenshot, err := workspace.Resolve(t.workspaceRoot, screenshotPath, false)
	if err != nil {
		return "", fmt.Errorf("prepare browser screenshot path %q: %w", screenshotPath, err)
	}
	cmd := exec.Command("npx", "playwright", "screenshot", "--browser=chromium", targetURL, resolvedScreenshot)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("capture browser screenshot from %q into %q: %w: %s", targetURL, resolvedScreenshot, err, out)
	}
	return resolvedScreenshot, nil
}
