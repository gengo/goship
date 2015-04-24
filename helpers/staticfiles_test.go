package helpers

import (
	"html/template"
	"path/filepath"
	"testing"
)

var getFilePathsTests = []struct {
	currentPath string
	extension   string
	expected    []string
	expectErr   error
}{
	{"../static/js", ".js", []string{}, nil},
	{"../static/js", ".gitkeep", []string{".gitkeep"}, nil},
	{"../static/css", ".css", []string{"styles.css"}, nil},
}

func TestGetFilePaths(t *testing.T) {
	for _, tt := range getFilePathsTests {
		rootPath, err := filepath.Abs(tt.currentPath)
		if err != nil {
			t.Fatalf("Failed to get absolute file path of %s. err: %s", tt.currentPath, err)
		}
		fps, err := getFilePaths(rootPath, tt.extension)
		if err != nil {
			if tt.expectErr != nil && err != tt.expectErr {
				t.Errorf("Error while getting (%s) files from %s. err: %s", tt.extension, rootPath, err)
			}
		}
		if len(fps) != len(tt.expected) {
			t.Errorf("Failed to get correct number of (%s) files from path: got: %d, want: %d", tt.extension, len(fps), len(tt.expected))
		}
		for i, _ := range fps {
			if fps[i] != tt.expected[i] {
				t.Errorf("Failed to get correct base file path for file. got: %s, want: %s", fps[i], tt.expected[i])
			}
		}
	}
}

var stylesheetTemplateTests = []struct {
	currentPath string
	expected    string
	expectErr   error
}{
	{"../static/js", "", nil},
	{"../static/css", "<link href='/static/css/styles.css' rel='stylesheet'>", nil},
}

func TestMakeStylesheetTemplate(t *testing.T) {
	for _, tt := range stylesheetTemplateTests {
		rootPath, err := filepath.Abs(tt.currentPath)
		if err != nil {
			t.Fatalf("Failed to get absolute file path of %s. err: %s", tt.currentPath, err)
		}
		tmpl := MakeStylesheetTemplate(rootPath)
		var expectTmpl = template.HTML(tt.expected)
		if tmpl != expectTmpl {
			t.Errorf("Failed to make right HTML template structure for stylesheet. got: %v, want: %v", tmpl, expectTmpl)
		}
	}
}

var javascriptTemplateTests = []struct {
	currentPath string
	expected    string
	expectErr   error
}{
	{"../static/js", "", nil},
	{"../static/css", "", nil},
}

func TestMakeJavascriptTemplate(t *testing.T) {
	for _, tt := range javascriptTemplateTests {
		rootPath, err := filepath.Abs(tt.currentPath)
		if err != nil {
			t.Fatalf("Failed to get absolute file path of %s. err: %s", tt.currentPath, err)
		}
		tmpl := MakeJavascriptTemplate(rootPath)
		var expectTmpl = template.HTML(tt.expected)
		if tmpl != expectTmpl {
			t.Errorf("Failed to make right HTML template structure for javascript. got: %v, want: %v", tmpl, expectTmpl)
		}
	}
}
