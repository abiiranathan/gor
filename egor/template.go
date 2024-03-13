package egor

import (
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func BaseLayout(baseLayout string) RouterOption {
	return func(r *Router) {
		r.baseLayout = baseLayout
	}
}

// ErrorTemplate sets the error template for the router.
// If set, this template will be used to render errors.
func ErrorTemplate(errorTemplate string) RouterOption {
	return func(r *Router) {
		r.errorTemplate = errorTemplate
	}
}

func ContentBlock(contentBlock string) RouterOption {
	return func(r *Router) {
		r.contentBlock = contentBlock
	}
}

func PassContextToViews(passContextToViews bool) RouterOption {
	return func(r *Router) {
		r.passContextToViews = passContextToViews
	}
}

func WithTemplates(t *template.Template) RouterOption {
	return func(r *Router) {
		r.template = t
	}
}

func ParseTemplatesRecursive(rootDir string, funcMap template.FuncMap, suffix ...string) (*template.Template, error) {
	ext := ".html"
	if len(suffix) > 0 {
		ext = suffix[0]
	}

	cleanRoot := filepath.Clean(rootDir)
	pfx := len(cleanRoot) + 1
	root := template.New("")

	err := filepath.WalkDir(cleanRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ext) {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			t := root.New(path[pfx:]).Funcs(funcMap)
			_, err = t.Parse(string(b))
			return err
		}
		return nil
	})

	return root, err
}

func ParseTemplatesRecursiveFS(root fs.FS, rootDir string, funcMap template.FuncMap, suffix ...string) (*template.Template, error) {
	ext := ".html"
	if len(suffix) > 0 {
		ext = suffix[0]
	}

	pfx := len(rootDir) + 1  // +1 for the trailing slash
	tmpl := template.New("") // Create a new template

	err := fs.WalkDir(root, rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ext) {
			if d != nil && d.IsDir() {
				return nil
			}

			b, err := fs.ReadFile(root, path)
			if err != nil {
				return err
			}

			t := tmpl.New(path[pfx:]).Funcs(funcMap)
			_, err = t.Parse(string(b))

			return err
		}
		return nil
	})
	return tmpl, err
}
