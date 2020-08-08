package template

import (
	"os"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/blushft/sweet-release/version"
)

type Templates struct {
	tmpl *template.Template
}

func New() (*Templates, error) {
	t := template.New("templates")

	t.Funcs(sprig.TxtFuncMap())

	versionFileTmpl(t)
	goVersionTmpl(t)

	return &Templates{
		tmpl: t,
	}, nil
}

func (t *Templates) Execute(n string, ver *version.Version) error {
	return t.tmpl.ExecuteTemplate(os.Stdout, n, ver)
}

func versionFileTmpl(t *template.Template) {
	template.Must(t.New("version_file").Parse(`{{.Semver}}`))
}

func goVersionTmpl(t *template.Template) {
	const tmpl = `
package version

var (
	Version = "{{.Semver}}"
	Commit = "{{.Commit}}"
	Branch = "{{.Branch}}"
)
`

	template.Must(t.New("go_package").Parse(tmpl))
}
