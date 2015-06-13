package lobster

import "fmt"
import "html/template"
import "io/ioutil"
import "net/http"
import "strings"
import "time"

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
		"modal": func(label string, action, buttonType string, token string) map[string]string {
			return map[string]string{
				"Label": label,
				"Id": stripAlphanumeric(label),
				"Action": action,
				"ButtonType": buttonType,
				"Token": token,
			}
		},
		"question": func(x bool, a string, b string) string {
			if x {
				return a
			} else {
				return b
			}
		},
	}
}

func loadTemplates() {
	templates = make(map[string]*template.Template)

	var commonPaths []string
	contents, _ := ioutil.ReadDir("tmpl/common")
	for _, fileInfo := range contents {
		if fileInfo.Mode().IsRegular() && strings.HasSuffix(fileInfo.Name(), ".html") {
			commonPaths = append(commonPaths, "tmpl/common/" + fileInfo.Name())
		}
	}

	for _, category := range []string{"splash", "panel", "admin"} {
		templatePaths := commonPaths
		contents, _ := ioutil.ReadDir("tmpl/" + category)
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
