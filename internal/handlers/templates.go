package handlers

import (
	"io"
)

// TemplateExecutor is an interface for template execution
// This allows both *template.Template and custom template registries to be used
type TemplateExecutor interface {
	ExecuteTemplate(wr io.Writer, name string, data interface{}) error
}
