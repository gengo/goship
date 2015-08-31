package plugin

import (
	"html/template"

	"github.com/gengo/goship/lib/config"
)

var Plugins []Plugin

// Plugin is the interface which view plugins must implement.
type Plugin interface {
	// Apply returns a list of additional columns for "p" in the main view.
	// The columns are rendered together with the standard columns like "Environment", "Hosts" or "Deployed Revision"
	Apply(p config.Project) ([]Column, error)
}

// RegisterPlugin registers "p" to Goship.
func RegisterPlugin(p Plugin) {
	Plugins = append(Plugins, p)
}

// Column is an interface that demands a RenderHeader and RenderDetails method to be able to generate a table column (with header and body)
// See templates/index.html to see how the Header and Render methods are used
type Column interface {
	// RenderHeader() returns a HTML template that should render a <th> element
	RenderHeader() (template.HTML, error)
	// RenderDetail() returns a HTML template that should render a <td> element
	RenderDetail() (template.HTML, error)
}
