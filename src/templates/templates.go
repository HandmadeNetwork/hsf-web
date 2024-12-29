package templates

import (
	"embed"
	"errors"
	"hsf/src/ee"
	"hsf/src/jobs"
	"hsf/src/logging"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

//go:embed files
var embeddedTemplateFs embed.FS

var templateReloadMutex = sync.Mutex{}
var allTemplates map[string]*template.Template

var ErrTemplateNotFound = errors.New("template not found")

func Render(wr io.Writer, name string, data any) error {
	template, ok := allTemplates[name]
	if !ok {
		return ee.New(ErrTemplateNotFound, "trying to load template %s", name)
	}
	return template.Execute(wr, data)
}

func LoadEmbedded() {
	var err error
	allTemplates, err = ReloadTemplates(embeddedTemplateFs)
	if err != nil {
		panic(err)
	}
	logging.Debug().Msg("Loaded embedded templates")
}

func ReloadTemplates(templateFS fs.FS) (map[string]*template.Template, error) {
	result := make(map[string]*template.Template)
	var err error
	logging.Debug().Msg("Reloading templates")

	err = fs.WalkDir(templateFS, "files", func(filepath string, d fs.DirEntry, err error) error {
		if err != nil {
			return ee.New(err, "Failed to walk dir: %s", filepath)
		}
		if !d.IsDir() && path.Dir(filepath) == "files" {
			name := d.Name()[:len(d.Name())-len(path.Ext(d.Name()))]
			tmpl := template.New(d.Name())
			tmpl.Funcs(hsfTemplateFuncs)
			tmpl, err = tmpl.ParseFS(templateFS,
				"files/include/*",
				"files/layouts/*",
				filepath,
			)
			if err != nil {
				return ee.New(err, "Failed to parse templates for filepath: %s", filepath)
			}
			result[name] = tmpl
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// WatchTemplates Watches the files/ folder for changes and reloads templates.
func WatchTemplates() *jobs.Job {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	baseDir := path.Join("src", "templates")
	watchFS := os.DirFS(baseDir)

	debouncer := time.NewTimer(time.Minute)
	debouncer.Stop()
	debouncerRunning := false

	job := jobs.New("Template watcher")

	go func() {
		defer watcher.Close()
		for {
			select {
			case <-watcher.Events:
				if !debouncer.Stop() && debouncerRunning {
					<-debouncer.C
				}
				debouncerRunning = true
				debouncer.Reset(time.Millisecond * 20)
			case err = <-watcher.Errors:
				panic(err)
			case <-debouncer.C:
				debouncerRunning = false
				newTemplates, err := ReloadTemplates(watchFS)
				if err != nil {
					logging.Error().Err(err).Msg("Failed to reload templates")
				} else {
					templateReloadMutex.Lock()
					allTemplates = newTemplates
					templateReloadMutex.Unlock()
					logging.Debug().Msg("Reloaded templates")
				}
			case <-job.Ctx.Done():
				logging.Info().Msg("Shutting down template watcher")
				job.Finish()
				return
			}
		}
	}()

	err = fs.WalkDir(watchFS, "files", func(filename string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			err = watcher.Add(path.Join(baseDir, filename))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return job
}

var hsfTemplateFuncs = map[string]any{}
