package viewhelpers

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/golang/glog"
)

const (
	javascriptExt = ".js"
	stylesheetExt = ".css"
	javascriptTag = "<script src='/static/js/%s'></script>"
	stylesheetTag = "<link href='/static/css/%s' rel='stylesheet'>"
)

type Assets struct {
	dir string
}

func New(dir string) Assets {
	return Assets{dir: dir}
}

func getFilePaths(root string, extension string) ([]string, error) {
	var filepaths []string
	var getFile = func(fp string, _ os.FileInfo, _ error) error {
		if filepath.Ext(fp) == extension {
			filepaths = append(filepaths, filepath.Base(fp)) // we only want the base file paths
		}
		return nil
	}
	err := filepath.Walk(root, getFile)
	if err != nil {
		return nil, err
	}
	return filepaths, nil
}

func getJavascriptFiles(folderpath string) []string {
	fps, err := getFilePaths(folderpath, javascriptExt)
	if err != nil {
		glog.Errorf("Failed to get all javascript file paths: %v", err)
		return nil
	}
	return fps
}

func getStylesheetFiles(folderpath string) []string {
	fps, err := getFilePaths(folderpath, stylesheetExt)
	if err != nil {
		glog.Errorf("Failed to get all Stylesheet file paths: %v", err)
		return nil
	}
	return fps
}

func makeJavascriptTemplate(folderpath string) template.HTML {
	fps := getJavascriptFiles(folderpath)
	var str string = ""
	for _, fp := range fps {
		str += fmt.Sprintf(javascriptTag, fp)
	}
	return template.HTML(str)
}

func makeStylesheetTemplate(folderpath string) template.HTML {
	fps := getStylesheetFiles(folderpath)
	var str string = ""
	for _, fp := range fps {
		str += fmt.Sprintf(stylesheetTag, fp)
	}
	return template.HTML(str)
}

func (a Assets) Templates() (js, css template.HTML) {
	sfp, err := filepath.Abs(a.dir)
	if err != nil {
		var tmpl = template.HTML("")
		glog.Errorf("Failed to locate static file path: %v", err)
		return tmpl, tmpl
	}
	js = makeJavascriptTemplate(filepath.Join(sfp, "js"))
	css = makeStylesheetTemplate(filepath.Join(sfp, "css"))
	return js, css
}
