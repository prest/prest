package middlewares

import (
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/clbanning/mxj/j2x"
	"github.com/nuveo/prest/statements"
)

func getVars(path string) (paths map[string]string) {
	pathList := strings.Split(path, "/")

	if len(pathList) < 3 || len(pathList) > 4 {
		return nil
	} else if len(pathList) == 4 {
		pathList = pathList[1:]
	}
	paths = make(map[string]string, 0)
	paths["database"] = pathList[0]
	paths["schema"] = pathList[1]
	paths["table"] = pathList[2]

	return
}

func permissionByMethod(method string) (permission string) {
	switch method {
	case "GET":
		permission = statements.READ
	case "POST", "PATCH", "PUT":
		permission = statements.WRITE
	case "DELETE":
		permission = statements.DELETE
	default:
		permission = ""
	}

	return
}

func renderFormat(w http.ResponseWriter, body io.Reader, format string) {
	byt, err := ioutil.ReadAll(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch format {
	case "xml":
		xmldata, err := j2x.JsonToXml(byt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write(xmldata)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.Write(byt)
	}
}
