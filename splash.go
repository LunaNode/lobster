package lobster

import "net/http"

type SplashTemplateParams struct {
	Title string
	Message string
	Token string
}

func getSplashHandler(template string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		message := ""
		if r.URL.Query()["message"] != nil {
			message = r.URL.Query()["message"][0]
		}

		params := SplashTemplateParams{
			Title: template,
			Message: message,
		}

		RenderTemplate(w, "splash", template, params)
	}
}

func getSplashFormHandler(template string) func(w http.ResponseWriter, r *http.Request, db *Database, session *Session) {
	return func(w http.ResponseWriter, r *http.Request, db *Database, session *Session) {
		message := ""
		if r.URL.Query()["message"] != nil {
			message = r.URL.Query()["message"][0]
		}

		params := SplashTemplateParams{
			Title: template,
			Message: message,
			Token: CSRFGenerate(db, session),
		}

		RenderTemplate(w, "splash", template, params)
	}
}

func splashNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(404)
	RenderTemplate(w, "splash", "notfound", SplashTemplateParams{Title: "404 Not Found"})
}
