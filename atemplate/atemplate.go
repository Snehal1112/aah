// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package atemplate

import (
	"errors"
	"fmt"
	"html/template"
	"path/filepath"
	"strings"

	"aahframework.org/config"
	"aahframework.org/essentials"
	"aahframework.org/log"
)

var (
	// TemplateFuncMap aah framework Go template function map.
	TemplateFuncMap = make(template.FuncMap)

	// TemplateEngine must comply TemplateEnginer
	_ TemplateEnginer = &TemplateEngine{}
)

type (
	// TemplateEnginer interface defines a methods for pluggable template engine.
	TemplateEnginer interface {
		Init(appCfg *config.Config, viewsBaseDir string)
		Load() error
		Reload() error
		Get(layout, path, tmplName string) *template.Template
	}

	// TemplateEngine struct is default template engine of aah framework using Go
	// and "html/template" package. Implements `TemplateEnginer`.
	TemplateEngine struct {
		appConfig *config.Config
		baseDir   string
		layouts   map[string]*Templates
	}

	// Templates hold template reference by layouts.
	Templates struct {
		TemplateLower map[string]*template.Template
		Template      map[string]*template.Template
	}
)

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Global methods
//___________________________________

// AddTemplateFunc method adds given Go template funcs into function map.
func AddTemplateFunc(funcMap template.FuncMap) {
	for fname, funcImpl := range funcMap {
		TemplateFuncMap[fname] = funcImpl
	}
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// TemplateEngine methods
//___________________________________

// Init method initialize a template engine with given aah application config
// and application views base path.
func (te *TemplateEngine) Init(cfg *config.Config, viewsBaseDir string) {
	te.appConfig = cfg
	te.baseDir = viewsBaseDir
	te.layouts = make(map[string]*Templates)
}

// Load method loads the view layouts and pages. It composes the Go template with
// layouts to support possible template inheritance over the views.
func (te *TemplateEngine) Load() error {
	if !ess.IsFileExists(te.baseDir) {
		return fmt.Errorf("views base dir is not exists: %s", te.baseDir)
	}

	layoutsBaseDir := filepath.Join(te.baseDir, "layouts")
	if !ess.IsFileExists(layoutsBaseDir) {
		return fmt.Errorf("layouts base dir is not exists: %s", layoutsBaseDir)
	}

	pagesBaseDir := filepath.Join(te.baseDir, "pages")
	if !ess.IsFileExists(pagesBaseDir) {
		return fmt.Errorf("pages base dir is not exists: %s", pagesBaseDir)
	}

	templateFileExt := te.appConfig.StringDefault("template.ext", ".html")

	layouts, err := te.glob(filepath.Join(layoutsBaseDir, "*"+templateFileExt))
	if err != nil {
		return err
	}

	pageDirs, err := ess.DirsPath(pagesBaseDir)
	if err != nil {
		return err
	}

	return te.processTemplates(layouts, pageDirs, "*"+templateFileExt)
}

// Reload method reloads the view layouts and pages again cleanly.
func (te *TemplateEngine) Reload() error {
	te.layouts = make(map[string]*Templates)
	return te.Load()
}

// Get method returns the template based given name if found, otherwise nil.
func (te *TemplateEngine) Get(layout, path, tmplName string) *template.Template {
	if l, ok := te.layouts[layout]; ok {
		path = te.DirKey(path)
		if te.appConfig.BoolDefault("template.case_sensitive", false) {
			if t, ok := l.Template[path]; ok {
				return t.Lookup(tmplName)
			}
		} else {
			if t, ok := l.TemplateLower[strings.ToLower(path)]; ok {
				return t.Lookup(strings.ToLower(tmplName))
			}
		}
	}

	return nil
}

// DirKey returns the unique key for given path.
func (te *TemplateEngine) DirKey(path string) string {
	path = path[strings.Index(path, "pages"):]
	path = strings.Replace(path, "/", "_", -1)
	path = strings.Replace(path, "\\", "_", -1)
	return path
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// TemplateEngine Unexported methods
//___________________________________

// glob method returns the template base name and path for given pattern
func (te *TemplateEngine) glob(pattern string) (map[string]string, error) {
	templates := make(map[string]string)
	files, err := filepath.Glob(pattern)
	if err != nil {
		return templates, err
	}

	for _, f := range files {
		templates[ess.StripExt(filepath.Base(f))] = f
	}
	return templates, nil
}

// processTemplates method process the layouts and pages dir wise.
func (te *TemplateEngine) processTemplates(layouts map[string]string, pageDirs []string, filePattern string) error {
	errorOccurred := false
	for layout, lpath := range layouts {
		lTemplate := &Templates{
			Template:      make(map[string]*template.Template),
			TemplateLower: make(map[string]*template.Template),
		}

		for _, dir := range pageDirs {
			files, err := filepath.Glob(filepath.Join(dir, filePattern))
			if err != nil {
				log.Error(err)
				errorOccurred = true
				continue
			}

			if len(files) == 0 {
				continue
			}

			files = append(files, lpath)

			// create key and init template with funcs
			dirKey := te.DirKey(dir)
			tmpl := template.New(dirKey).Funcs(TemplateFuncMap)

			// Set custom delimiters from aah.conf
			if te.appConfig.IsExists("template.delimiters") {
				delimiters := strings.Split(te.appConfig.StringDefault("template.delimiter", "{{.}}"), ".")
				if len(delimiters) == 2 {
					tmpl.Delims(delimiters[0], delimiters[1])
				} else {
					log.Error("config 'template.delimiter' value is not valid")
				}
			}

			_, err = tmpl.ParseFiles(files...)
			if err != nil {
				log.Error(err)
				errorOccurred = true
				continue
			}

			lTemplate.Template[dirKey] = tmpl
			lTemplate.TemplateLower[strings.ToLower(dirKey)] = tmpl
		}
		te.layouts[layout] = lTemplate
	}

	if errorOccurred {
		return errors.New("error processing templates, check the log")
	}

	return nil
}