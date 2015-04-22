package helpers

import (
	"fmt"
	"html/template"
	"log"
	"os"
	"path"
	"path/filepath"
)

const (
	javascriptExt = ".js"
	stylesheetExt = ".css"
	javascriptTag = "<script src='/static/js/%s'></script>"
	stylesheetTag = "<link href='/static/css/%s' rel='stylesheet'>"
)

func getFilePaths(root string, extension string) ([]string, error) {
	var filepaths []string
	var getFile = func(fp string, _ os.FileInfo, _ error) error {
		if path.Ext(fp) == extension {
			filepaths = append(filepaths, path.Base(fp))
		}
		return nil
	}
	err := filepath.Walk(root, getFile)
	if err != nil {
		return nil, err
	}
	return filepaths, nil
}

func GetJavascriptFiles(folderpath string) []string {
	fps, err := getFilePaths(folderpath, javascriptExt)
	if err != nil {
		log.Printf("Failed to get all javascript file paths: %s", err)
		return nil
	}
	return fps
}

func GetStylesheetFiles(folderpath string) []string {
	fps, err := getFilePaths(folderpath, stylesheetExt)
	if err != nil {
		log.Printf("Failed to get all Stylesheet file paths: %s", err)
		return nil
	}
	return fps
}

func MakeJavascriptTemplate(folderpath string) template.HTML {
	fps := GetJavascriptFiles(folderpath)
	var str string = ""
	for _, fp := range fps {
		str += fmt.Sprintf(javascriptTag, fp)
	}
	return template.HTML(str)
}

func MakeStylesheetTemplate(folderpath string) template.HTML {
	fps := GetStylesheetFiles(folderpath)
	var str string = ""
	for _, fp := range fps {
		str += fmt.Sprintf(stylesheetTag, fp)
	}
	return template.HTML(str)
}
