package lobster

import "net/http"
import "path/filepath"
import "os"

var validAssets map[string]bool

func loadAssets() {
	validAssets = make(map[string]bool)
	filepath.Walk("assets/", func(path string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			validAssets[path] = true
		}
		return nil
	})
}

func assetsHandler(w http.ResponseWriter, r *http.Request) {
	assetPath := r.URL.Path[1:]
	if validAssets[assetPath] {
		http.ServeFile(w, r, assetPath)
	} else {
		http.NotFound(w, r)
	}
}
