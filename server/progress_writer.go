package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type ProgressWriter interface {
	Start(w http.ResponseWriter)
	Chunk(w http.ResponseWriter, v string)
	Finish(w http.ResponseWriter, v HandleFunc)
}

type DefaultProgressWriter struct{}

func (p *DefaultProgressWriter) Start(w http.ResponseWriter) {
	w.Header().Set("Content-Type", ContentTypeHTML)
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
  <title>Progress</title>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    html, body { 
      margin: 0;
      padding: 0; 
      height: 100%; 
      overflow: hidden;
      margin: 0 auto; 
    }
    #progress {
      box-sizing: border-box;
      width: 100vw; height: 100vh;
      padding: 1rem;
      font-family: Menlo, monospace;
      font-size: 1rem;
      line-height: 1.4;
      overflow-y: scroll;
      scrollbar-width: none;       /* Firefox */
      -ms-overflow-style: none;    /* IE 10+ */
    }
    #progress::-webkit-scrollbar { width: 0; height: 0; }
  </style>
</head>
<body>
  <div id="progress" style="white-space: pre;"></div>
`))

	flusher, ok := w.(http.Flusher)
	if ok {
		flusher.Flush()
	}
}

func (p *DefaultProgressWriter) Chunk(w http.ResponseWriter, v string) {
	setTextContent(w, v, true)

	flusher, ok := w.(http.Flusher)
	if ok {
		flusher.Flush()
	}
}

type dummyResponseWriter struct {
	headers http.Header
	body    []byte
	code    int
}

func (d *dummyResponseWriter) Header() http.Header {
	if d.headers == nil {
		d.headers = make(http.Header)
	}

	return d.headers
}

func (d *dummyResponseWriter) Write(b []byte) (int, error) {
	d.body = append(d.body, b...)
	return len(b), nil
}

func (d *dummyResponseWriter) WriteHeader(statusCode int) {
	d.code = statusCode
}

func (p *DefaultProgressWriter) Finish(w http.ResponseWriter, v HandleFunc) {
	ww := &dummyResponseWriter{
		headers: make(http.Header),
		body:    []byte{},
		code:    0,
	}

	err := v(ww)
	if err != nil {
		if errors.Is(err, ErrUnsafe) {
			setReload(w)
			return
		}

		setTextContent(w, err.Error(), false)

		return
	}

	switch ww.code {
	case 0, http.StatusOK, http.StatusAccepted:
		setReload(w)
	default:
		text := fmt.Sprintf("%d\n\n%s", ww.code, string(ww.body))
		setTextContent(w, text, false)
	}
}

func setReload(w http.ResponseWriter) {
	_, _ = w.Write([]byte(`<script>location.reload();</script></body></html>`))
}

func setTextContent(w http.ResponseWriter, text string, app bool) {
	vv, err := json.Marshal(text)
	if err != nil {
		return
	}

	operator := "="

	if app {
		operator = "+="
	}

	snippet := fmt.Sprintf(`<script>
		(function(){
		var prog = document.getElementById("progress");
		prog.textContent %s %s;
		prog.scrollTop = prog.scrollHeight;
	})();
	</script>`, operator, vv)
	_, _ = w.Write([]byte(snippet))
}
