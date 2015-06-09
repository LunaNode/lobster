package lobster

import "html/template"
import "net/http"
import "io/ioutil"
import "strings"
import "time"
import "fmt"

var templates map[string]*template.Template

func templateFuncMap() template.FuncMap {
	return template.FuncMap {
		"Title": strings.Title,
		"FormatTime": func(t time.Time) string {
			return t.Format(TIME_FORMAT)
		},
		"FormatDate": func(t time.Time) string {
			return t.Format(DATE_FORMAT)
		},
		"FormatCredit": func(x int64) string {
			return fmt.Sprintf("$%.3f", float64(x) / BILLING_PRECISION)
		},
		"FormatGB": func(x int64) string {
			return fmt.Sprintf("%.2f", float64(x) / 1024 / 1024 / 1024)
		},
		"FormatFloat2": func(x float64) string {
			return fmt.Sprintf("%.2f", x)
		},
		"MonthInteger": func(x time.Month) int {
			return int(x)
		},
	}
}

func loadTemplates() {
	templates = make(map[string]*template.Template)
	for _, category := range []string{"splash", "panel", "admin"} {
		contents, _ := ioutil.ReadDir("tmpl/" + category)
		templatePaths := make([]string, 0)
		for _, fileInfo := range contents {
			if fileInfo.Mode().IsRegular() && strings.HasSuffix(fileInfo.Name(), ".html") {
				templatePaths = append(templatePaths, "tmpl/" + category + "/" + fileInfo.Name())
			}
		}
		templates[category] = template.Must(template.New("").Funcs(templateFuncMap()).ParseFiles(templatePaths...))
	}
}

func renderTemplate(w http.ResponseWriter, category string, tmpl string, data interface{}) error {
	err := templates[category].ExecuteTemplate(w, tmpl + ".html", data)
	if err != nil {
		http.Error(w, "Template render failure: " + err.Error(), http.StatusInternalServerError)
	}
	return err
}
