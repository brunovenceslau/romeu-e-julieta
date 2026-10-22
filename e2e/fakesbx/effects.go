// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// envFile is what the fake reads of an sbxenv.yaml: the name of the
// sandbox and the primary workspace. Every other key is sbx's own to
// read (https://docs.docker.com/ai/sandboxes/configuration/environment-files/).
type envFile struct {
	Name      string `yaml:"name"`
	Agent     string `yaml:"agent"`
	Workspace any    `yaml:"workspace"`
}

// readEnvFile reads <dir>/sbxenv.yaml and returns the name of the
// sandbox and the absolute path of its primary workspace: the name is
// the file's `name`, else <agent>-<workspace basename>, and a relative
// workspace resolves from the directory of the file, as sbx does.
func readEnvFile(dir string) (name, workspace string, err error) {
	data, err := os.ReadFile(filepath.Join(dir, "sbxenv.yaml"))
	if err != nil {
		return "", "", err
	}
	var f envFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return "", "", fmt.Errorf("%s: %w", filepath.Join(dir, "sbxenv.yaml"), err)
	}
	switch w := f.Workspace.(type) {
	case string:
		workspace = w
	case map[string]any:
		workspace, _ = w["path"].(string)
	}
	if workspace == "" {
		return "", "", errors.New("sbxenv.yaml names no primary workspace")
	}
	workspace = strings.NewReplacer("${{ env.projectDir }}", dir, "${{ env.fileDir }}", dir).Replace(workspace)
	if !filepath.IsAbs(workspace) {
		workspace = filepath.Join(dir, workspace)
	}
	workspace = filepath.Clean(workspace)
	name = f.Name
	if name == "" {
		name = f.Agent + "-" + filepath.Base(workspace)
	}
	return name, workspace, nil
}

// applyEffects writes what the entry recorded outside its answer: on a
// create, the stanza of the remote of the sandbox into the config of the
// primary clone, as sbx does (01 1.2, REQ-0929).
func applyEffects(e entry, argv []string, cfg config) error {
	if e.Effects == nil || e.Effects.GitRemote == nil {
		return nil
	}
	i := slices.Index(e.Argv, "<dir>")
	if i < 0 {
		return errors.New("an entry with a gitRemote effect has no <dir> in its argv")
	}
	name, workspace, err := readEnvFile(argv[i])
	if err != nil {
		return err
	}
	if cfg.GitURL == "" && strings.Contains(e.Effects.GitRemote.URL, "<git-url>") {
		return errors.New("the entry writes a remote with <git-url>, and the configuration has no gitUrl")
	}
	subst := strings.NewReplacer("<name>", name, "<git-url>", cfg.GitURL)
	r := e.Effects.GitRemote
	stanza := fmt.Sprintf("[remote %q]\n\turl = %s\n\tfetch = %s\n", subst.Replace(r.Name), subst.Replace(r.URL), subst.Replace(r.Fetch))
	config := filepath.Join(workspace, ".git", "config")
	f, err := os.OpenFile(config, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("the primary clone: %w", err)
	}
	if _, err := f.WriteString(stanza); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
