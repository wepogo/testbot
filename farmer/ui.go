package farmer

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"
)

var funcMap = template.FuncMap{
	"reltime": reltime,
}

var page = template.Must(template.New("page").Funcs(funcMap).Parse(`

{{- define "resultline" -}}
<time
  datetime={{.CreatedAt.Local.Format "2006-01-02T15:04:05.000Z07:00"}}
  title={{.CreatedAt.Local.Format "2006-01-02T15:04:05.000Z07:00"}}
>
{{- reltime .CreatedAt | printf "%8s" -}}
</time> <a href=/result/{{.ID}}>result</a>
{{- .ElapsedSp}} {{.ElapsedMS}}ms
{{- if eq .State "success"}} ok {{else}} <b>fail</b> {{end -}}
{{- $org := .Org }}
{{- $repo := .Repo }}
{{- range .PR -}}
<a href=https://github.com/{{$org}}/{{$repo}}/pull/{{.}}>#{{.}}</a> {{end -}}
{{- printf "%.8s" .SHA}} {{.Dir}} {{.Name -}}
{{- if eq .State "success"}}{{else}} <b>{{.Desc}}</b>{{end -}}
{{- end -}}

{{- define "prlist" -}}
{{- $org := .Org }}
{{- $repo := .Repo }}
{{range .PR -}}
<a href="https://github.com/{{$org}}/{{$repo}}/pull/{{.}}">https://github.com/{{$org}}/{{$repo}}/pull/{{.}}</a>
{{end}}
{{- end -}}

<!doctype html>
<meta name=viewport content="initial-scale=1">
<title>{{block "title" .}}testbot{{end}}</title>
<link rel=stylesheet href=/static/a.css>
<script src=/static/a.js async></script>
<pre style="white-space: pre-wrap">
{{- template "content" . -}}
{{- /*
Leave the pre element open. The resultpage
and livepage handlers append more output
after the template is rendered, so we have
no chance to put anything, such as a closing
pre tag, at the end of the document.
*/ -}}
`))

var homePage = template.Must(template.Must(page.Clone()).Parse(`

{{define "content"}}
<b>testbot</b> <a href=guide.txt>guide.txt</a>

<b>boxes</b>
{{- range .Boxes}}
{{.}}
{{- else}}
{{.ErrBox}}
{{- end}}

<b>assignments</b>
{{- range .States}}
{{.}}
{{- else}}
(none)
{{- end}}

<b>jobs</b>
{{- range .Jobs}}
{{.}}
{{- else}}
{{.ErrJob}}
{{- end}}

<b>results</b> (just the last {{len .Results}} of them)
{{- range .Results}}
{{template "resultline" .}}
{{- else}}
{{.ErrResult}}
{{- end}}
{{end}}

`))

var livePage = template.Must(template.Must(page.Clone()).Parse(`

{{define "title"}}{{.Title}}{{end}}

{{define "content"}}
<b>{{.Title}}</b>
{{template "prlist" .}}
{{- if .Live}}
<form method=post action=/cancel><input type=submit value=cancel></form>
{{- end}}

<b>past results</b>
{{- range .Results}}
{{template "resultline" .}}
{{- else}}
{{.ErrResult}}
{{- end}}

<b>output</b>
{{- if not .Live}}
(there is no such job currently live.)
{{- end}}
{{end}}

`))

var resultPage = template.Must(template.Must(page.Clone()).Parse(`

{{define "title"}}{{.Title}}{{end}}

{{define "content"}}
<b>{{.Title}}</b>
{{template "prlist" .}}
<form method=post action=/retry><input type=submit value=retry></form>
<b>output</b>
{{end}}

`))

const js = `
const s=1000, m=60*s, h=60*m, a=24*h;
const month = [
	"Jan", "Feb", "Mar", "Apr", "May", "Jun",
	"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
];

function reltime(t) {
	const d = Date.now() - t.getTime();
	if (d < 5*s) return '<5s ago';
	if (d < 2*m) return Math.round(d/s) + 's ago';
	if (d < 2*h) return Math.round(d/m) + 'm ago';
	if (d < 2*a) return Math.round(d/h) + 'h ago';
	if (d < 90*a) return Math.round(d/a) + 'd ago';
	return month[t.getMonth()] + ' ' + t.getFullYear();
}

function update() {
	for (const e of document.querySelectorAll('time[datetime]')) {
		const s = reltime(new Date(e.dateTime));
		const p = ' '.repeat(Math.max(0, 8 - s.length));
		e.innerText = p + s;
	}
}

setInterval(update, 5*s);
`

// reltime returns the approximate duration since time t.
// For times more than 90 days ago, it returns the
// absolute month and year.
func reltime(t time.Time) string {
	switch d := time.Since(t); {
	case d < 5*time.Second:
		return "<5s ago"
	case d < 2*time.Minute:
		return fmt.Sprintf("%ds ago", roundDur(d, time.Second))
	case d < 2*time.Hour:
		return fmt.Sprintf("%dm ago", roundDur(d, time.Minute))
	case d < 2*24*time.Hour:
		return fmt.Sprintf("%dh ago", roundDur(d, time.Hour))
	case d < 90*24*time.Hour:
		return fmt.Sprintf("%dd ago", roundDur(d, 24*time.Hour))
	}
	return t.Format("Jan 2006")
}

func roundDur(n, d time.Duration) int {
	return int((n + d/2) / d)
}

var modtime = time.Now()

func static(name, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		r := strings.NewReader(body)
		http.ServeContent(w, req, name, modtime, r)
	}
}

// pad returns a string containing
// enough spaces to pad s to length 6.
func pad(s string) string {
	const sp = "      "
	if len(s) >= len(sp) {
		return ""
	}
	return sp[:len(sp)-len(s)]
}
