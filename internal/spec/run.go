// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

// Run is the run layout of a project (03 3.2): romeu's own format, which
// julieta renders for herdr. Every pane id appears exactly once across
// all tabs (REQ-0197).
type Run struct {
	// Tabs are the tabs, each a binary tree of panes.
	Tabs []Tab `json:"tabs" yaml:"tabs"`
	// Panes are the panes the tabs place.
	Panes []Pane `json:"panes" yaml:"panes"`
}

// Tab is one tab of the run layout.
type Tab struct {
	// Label is the tab's label.
	Label string `json:"label" yaml:"label"`
	// Root is the tab's tree of panes.
	Root Node `json:"root" yaml:"root"`
}

// Node is a node of a tab's tree: a leaf names a pane; any other node
// splits its area between two nodes. A node holds Pane, or Split,
// Ratio, First and Second, and nothing else.
type Node struct {
	// Pane is the id of the pane of a leaf.
	Pane string `json:"pane,omitzero" yaml:"pane,omitempty"`
	// Split is row (side by side) or column (stacked).
	Split string `json:"split,omitzero" yaml:"split,omitempty"`
	// Ratio is the share of First, from 0.1 to 0.9.
	Ratio float64 `json:"ratio,omitzero" yaml:"ratio,omitempty"`
	// First is the left or top node.
	First *Node `json:"first,omitzero" yaml:"first,omitempty"`
	// Second is the right or bottom node.
	Second *Node `json:"second,omitzero" yaml:"second,omitempty"`
}

// Pane is one pane of the run layout. Every step runs as
// "mise exec -C <pane dir> -- <argv>", in order (REQ-0196).
type Pane struct {
	// ID is the pane's id, which a tab's leaf names.
	ID string `json:"id" yaml:"id"`
	// Repo is the dir of the project repository the pane starts in.
	Repo string `json:"repo" yaml:"repo"`
	// Cwd is the pane's directory relative to the repository.
	Cwd string `json:"cwd,omitzero" yaml:"cwd,omitempty"`
	// Env are environment variables of the pane's steps only.
	Env map[string]string `json:"env,omitzero" yaml:"env,omitempty"`
	// Steps are the argv lists the pane runs, with no shell; none is a
	// login shell.
	Steps [][]string `json:"steps,omitzero" yaml:"steps,omitempty"`
	// Focus marks the pane focused in its tab; at most one per tab.
	Focus bool `json:"focus,omitzero" yaml:"focus,omitempty"`
}
