package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ilsrbn/nuxt-analyzer/assets"
)

type runCmdFunc func(name string, args []string, input []byte) ([]byte, error)

// Bridge extracts and invokes the embedded Node parser bundle.
type Bridge struct {
	parserPath string
	tempDir    string
	runCmd     runCmdFunc
	initOnce   sync.Once
	initErr    error
}

// newNodeBridge extracts the embedded parser bundle into a temp file and returns a Bridge.
// Returns cleanup function that should be called when done.
func newNodeBridge() (*Bridge, func(), error) {
	b := &Bridge{}
	cleanup := func() {
		if b.tempDir != "" {
			os.RemoveAll(b.tempDir)
		}
	}

	b.initOnce.Do(func() {
		dir, err := os.MkdirTemp("", "nuxt-analyzer-parser-*")
		if err != nil {
			b.initErr = fmt.Errorf("create temp dir: %w", err)
			return
		}

		parserPath := filepath.Join(dir, "parser.bundle.js")
		if err := os.WriteFile(parserPath, assets.ParserBundle, 0o644); err != nil {
			b.initErr = fmt.Errorf("write parser bundle: %w", err)
			return
		}

		b.parserPath = parserPath
		b.tempDir = dir
		b.runCmd = defaultRunCmd
	})

	if b.initErr != nil {
		return nil, cleanup, b.initErr
	}

	return b, cleanup, nil
}

// Cleanup removes the temporary directory used for the parser bundle.
// It should be called (typically via defer) when the Bridge is no longer needed.
func (b *Bridge) Cleanup() {
	if b != nil && b.tempDir != "" {
		os.RemoveAll(b.tempDir)
	}
}

// Parse sends file paths and auto-import names to the Node parser and returns
// parsed file metadata. autoImportNames is the set of locally-defined
// auto-importable identifiers read from .nuxt/imports.d.ts; pass nil or empty
// to skip auto-import usage detection.
func (b *Bridge) Parse(files []string, autoImportNames []string) ([]ParsedFile, error) {
	if b == nil {
		return nil, fmt.Errorf("node bridge: nil bridge")
	}
	if b.initErr != nil {
		return nil, fmt.Errorf("node bridge init: %w", b.initErr)
	}
	if b.parserPath == "" {
		return nil, fmt.Errorf("node bridge: parser path is empty")
	}
	if b.runCmd == nil {
		b.runCmd = defaultRunCmd
	}
	if autoImportNames == nil {
		autoImportNames = []string{}
	}

	payload := struct {
		Files           []string `json:"files"`
		AutoImportNames []string `json:"autoImportNames"`
	}{
		Files:           files,
		AutoImportNames: autoImportNames,
	}

	input, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal bridge input: %w", err)
	}

	output, err := b.runCmd("node", []string{b.parserPath}, input)
	if err != nil {
		return nil, fmt.Errorf("node bridge: %w", err)
	}

	var response struct {
		Results []ParsedFile `json:"results"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("unmarshal bridge output: %w", err)
	}

	return response.Results, nil
}

func defaultRunCmd(name string, args []string, input []byte) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = bytes.NewReader(input)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := fmt.Sprintf("run %s %s: %v", name, strings.Join(args, " "), err)
		if stderr.Len() > 0 {
			msg += fmt.Sprintf("; stderr: %s", strings.TrimSpace(stderr.String()))
		}
		if stdout.Len() > 0 {
			msg += fmt.Sprintf("; stdout: %s", strings.TrimSpace(stdout.String()))
		}
		return nil, fmt.Errorf("%s: %w", msg, err)
	}

	return stdout.Bytes(), nil
}
