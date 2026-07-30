//go:build darwin

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const label = "com.geda.clipboard"

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist"), nil
}

// launchTarget returns the path to run at login. Inside a .app bundle we launch
// the bundle itself so LSUIElement and the Info.plist identity apply; otherwise
// we fall back to the bare executable.
func launchTarget() (program string, args []string, err error) {
	exe, err := os.Executable()
	if err != nil {
		return "", nil, err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", nil, err
	}

	// .../Foo.app/Contents/MacOS/foo -> .../Foo.app
	if idx := strings.Index(exe, ".app/Contents/MacOS/"); idx != -1 {
		bundle := exe[:idx+len(".app")]
		return "/usr/bin/open", []string{"-a", bundle}, nil
	}
	return exe, nil, nil
}

func set(enabled bool) error {
	path, err := plistPath()
	if err != nil {
		return err
	}

	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	program, args, err := launchTarget()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	sb.WriteString(`<plist version="1.0">` + "\n<dict>\n")
	sb.WriteString("\t<key>Label</key>\n\t<string>" + escapeXML(label) + "</string>\n")
	sb.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n")
	sb.WriteString("\t\t<string>" + escapeXML(program) + "</string>\n")
	for _, arg := range args {
		sb.WriteString("\t\t<string>" + escapeXML(arg) + "</string>\n")
	}
	sb.WriteString("\t</array>\n")
	sb.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n")
	// The app is expected to stay running, but relaunching it on crash would
	// fight the user's own Quit, so leave KeepAlive off.
	sb.WriteString("\t<key>KeepAlive</key>\n\t<false/>\n")
	sb.WriteString("</dict>\n</plist>\n")

	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("write launch agent: %w", err)
	}
	return nil
}

func isEnabled() bool {
	path, err := plistPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
