package gor

import (
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// BaseLayout sets the base layout template for the router.
// If set, this template will be used as the base layout for all views.
// The `contentBlock` variable will be replaced with the rendered content of the view.
//
// Example:
//
//	r := gor.NewRouter(gor.BaseLayout("layouts/base.html"))
func BaseLayout(baseLayout string) RouterOption {
	return func(r *Router) {
		r.baseLayout = baseLayout
	}
}

// ErrorTemplate sets the error template for the router.
// If set, this template will be used to render errors.
// It is passed "error", "status", "status_text" in its context.
// Example:
//
//	r := gor.NewRouter(gor.ErrorTemplate("errors/500.html"))
func ErrorTemplate(errorTemplate string) RouterOption {
	return func(r *Router) {
		r.errorTemplate = errorTemplate
	}
}

// ContentBlock sets the name of the content block in the base layout template.
// This block will be replaced with the rendered content of the view.
// The default content block name is "content".
//
// Example:
//
//	r := NewRouter()
//	r = r.WithOption(ContentBlock("main")) // Use "main" as the content block name
func ContentBlock(contentBlock string) RouterOption {
	return func(r *Router) {
		r.contentBlock = contentBlock
	}
}

// PassContextToViews enables or disables passing the router context to views.
// If enabled, the router context will be available as a variable named "ctx" in the views.
// This allows views to access information about the request and the router.
// The default value is `false`.
//
// Example:
//
//	r := NewRouter(gor.PassContextToViews(true))
func PassContextToViews(passContextToViews bool) RouterOption {
	return func(r *Router) {
		r.passContextToViews = passContextToViews
	}
}

// WithTemplates sets the template for the router.
// This template will be used to render views.
//
// Example:
//
//	t := template.Must(template.ParseFiles("views/index.html"))
//	r := NewRouter(gor.WithTemplates(t))
func WithTemplates(t *template.Template) RouterOption {
	return func(r *Router) {
		r.template = t
	}
}

// ParseTemplatesRecursive parses all templates in a directory recursively.
// It uses the specified `funcMap` to define custom template functions.
// The `suffix` argument can be used to specify a different file extension for the templates.
// The default file extension is ".html".
//
// Example:
//
//	t, err := gor.ParseTemplatesRecursive("templates", template.FuncMap{
//		"now": func() time.Time { return time.Now() },
//	}, ".tmpl") // Use ".tmpl" as the file extension
//
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	r := NewRouter(gor.WithTemplates(t))
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

// ParseTemplatesRecursiveFS parses all templates in a directory recursively from a given filesystem.
// It uses the specified `funcMap` to define custom template functions.
// The `suffix` argument can be used to specify a different file extension for the templates.
// The default file extension is ".html".
//
// Example:
//
//		t, err := gor.ParseTemplatesRecursiveFS(
//	 	http.FS(http.Dir("templates")), "templates", template.FuncMap{
//						"now": func() time.Time { return time.Now() },
//			}, ".tmpl")
//
//		 if err != nil {
//		   log.Fatal(err)
//		 }
//
//		 r := NewRouter(gor.WithTemplates(t))
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

			t := tmpl.New(rootDir + "/" + path[pfx:]).Funcs(funcMap)
			_, err = t.Parse(string(b))

			return err
		}
		return nil
	})
	return tmpl, err
}
