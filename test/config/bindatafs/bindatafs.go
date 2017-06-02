package bindatafs

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jteeuwen/go-bindata"
	"github.com/qor/admin"
)

type AssetFSInterface interface {
	NameSpace(nameSpace string) AssetFSInterface
	RegisterPath(path string) error
	Asset(name string) ([]byte, error)
	Glob(pattern string) (matches []string, err error)
	FileServer(dir http.Dir, assetPaths ...string) http.Handler
	Compile() error
}

var AssetFS AssetFSInterface

type viewPath struct {
	Dir        string
	AssetPaths []string
}

type bindataFS struct {
	Path            string
	viewPaths       []viewPath
	AssetFileSystem admin.AssetFSInterface
	nameSpacedFS    []*nameSpacedBindataFS
}

type nameSpacedBindataFS struct {
	*bindataFS
	nameSpace       string
	viewPaths       []viewPath
	AssetFileSystem admin.AssetFSInterface
}

func init() {
	AssetFS = &bindataFS{AssetFileSystem: &admin.AssetFileSystem{}, Path: "test/config/bindatafs"}
}

func (assetFS *bindataFS) NameSpace(nameSpace string) AssetFSInterface {
	nameSpacedFS := &nameSpacedBindataFS{bindataFS: assetFS, nameSpace: nameSpace, AssetFileSystem: &admin.AssetFileSystem{}}
	assetFS.nameSpacedFS = append(assetFS.nameSpacedFS, nameSpacedFS)
	return nameSpacedFS
}

func (assetFS *bindataFS) registerPath(path interface{}) error {
	var viewPth viewPath
	if pth, ok := path.(viewPath); ok {
		viewPth = pth
	} else {
		viewPth = viewPath{Dir: fmt.Sprint(path)}
	}

	assetFS.viewPaths = append(assetFS.viewPaths, viewPth)
	return assetFS.AssetFileSystem.RegisterPath(viewPth.Dir)
}

func (assetFS *bindataFS) RegisterPath(path string) error {
	return assetFS.registerPath(path)
}

func (assetFS *bindataFS) Asset(name string) ([]byte, error) {
	name = strings.TrimPrefix(name, "/")
	if len(_bindata) > 0 {
		return Asset(name)
	}
	return assetFS.AssetFileSystem.Asset(name)
}

func (assetFS *bindataFS) Glob(pattern string) (matches []string, err error) {
	if len(_bindata) > 0 {
		for key, _ := range _bindata {
			if ok, err := filepath.Match(pattern, key); ok && err == nil {
				matches = append(matches, key)
			}
		}
		return matches, nil
	}

	return assetFS.AssetFileSystem.Glob(pattern)
}

func (assetFS *bindataFS) Compile() error {
	fmt.Println("Compiling templates...")
	os.RemoveAll(filepath.Join(assetFS.Path, "templates"))
	copyFiles(filepath.Join(assetFS.Path, "templates"), assetFS.viewPaths)
	for _, fs := range assetFS.nameSpacedFS {
		copyFiles(filepath.Join(assetFS.Path, "templates", fs.nameSpace), fs.viewPaths)
	}

	config := bindata.NewConfig()
	config.Input = []bindata.InputConfig{
		{
			Path:      filepath.Join(assetFS.Path, "templates"),
			Recursive: true,
		},
	}
	config.Package = "bindatafs"
	config.Tags = "bindatafs"
	config.Output = filepath.Join(assetFS.Path, "templates_bindatafs.go")
	config.Prefix = filepath.Join(assetFS.Path, "templates")
	config.NoMetadata = true

	defer os.Exit(0)
	return bindata.Translate(config)
}

var cacheSince = time.Now().Format(http.TimeFormat)

func (assetFS *bindataFS) FileServer(dir http.Dir, assetPaths ...string) http.Handler {
	fileServer := assetFS.NameSpace("file_server")
	if fs, ok := fileServer.(*nameSpacedBindataFS); ok {
		fs.registerPath(viewPath{Dir: string(dir), AssetPaths: assetPaths})
	} else {
		fileServer.RegisterPath(string(dir))
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-Modified-Since") == cacheSince {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Last-Modified", cacheSince)

		requestPath := r.URL.Path
		if content, err := fileServer.Asset(requestPath); err == nil {
			etag := fmt.Sprintf("%x", md5.Sum(content))
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}

			if ctype := mime.TypeByExtension(filepath.Ext(requestPath)); ctype != "" {
				w.Header().Set("Content-Type", ctype)
			}

			w.Header().Set("Cache-control", "private, must-revalidate, max-age=300")
			w.Header().Set("ETag", etag)
			w.Write(content)
			return
		}

		http.NotFound(w, r)
	})
}

func copyFiles(templatesPath string, viewPaths []viewPath) {
	for i := len(viewPaths) - 1; i >= 0; i-- {
		pth := viewPaths[i]
		filepath.Walk(pth.Dir, func(path string, info os.FileInfo, err error) error {
			if err == nil {
				var relativePath = strings.TrimPrefix(strings.TrimPrefix(path, pth.Dir), "/")

				if len(pth.AssetPaths) > 0 {
					included := false
					for _, assetPath := range pth.AssetPaths {
						if strings.HasPrefix(relativePath, strings.Trim(assetPath, "/")+"/") || relativePath == strings.Trim(assetPath, "/") {
							included = true
							break
						}
					}
					if !included {
						return nil
					}
				}

				if info.IsDir() {
					err = os.MkdirAll(filepath.Join(templatesPath, relativePath), os.ModePerm)
				} else if info.Mode().IsRegular() {
					if source, err := ioutil.ReadFile(path); err == nil {
						if err = ioutil.WriteFile(filepath.Join(templatesPath, relativePath), source, os.ModePerm); err != nil {
							fmt.Println(err)
						}
					}
				}
			}
			return err
		})
	}
}

func (assetFS *nameSpacedBindataFS) registerPath(path interface{}) error {
	var viewPth viewPath
	if pth, ok := path.(viewPath); ok {
		viewPth = pth
	} else {
		viewPth = viewPath{Dir: fmt.Sprint(path)}
	}

	assetFS.viewPaths = append(assetFS.viewPaths, viewPth)
	return assetFS.AssetFileSystem.RegisterPath(viewPth.Dir)
}

func (assetFS *nameSpacedBindataFS) RegisterPath(path string) error {
	return assetFS.registerPath(path)
}

func (assetFS *nameSpacedBindataFS) Asset(name string) ([]byte, error) {
	name = strings.TrimPrefix(name, "/")
	if len(_bindata) > 0 {
		return Asset(filepath.Join(assetFS.nameSpace, name))
	}
	return assetFS.AssetFileSystem.Asset(name)
}

func (assetFS *nameSpacedBindataFS) Glob(pattern string) (matches []string, err error) {
	if len(_bindata) > 0 {
		nameSpacedPattern := filepath.Join(assetFS.nameSpace, pattern)
		for key := range _bindata {
			if ok, err := filepath.Match(nameSpacedPattern, key); ok && err == nil {
				matches = append(matches, strings.TrimPrefix(key, assetFS.nameSpace))
			}
		}
		return matches, nil
	}

	return assetFS.AssetFileSystem.Glob(pattern)
}
