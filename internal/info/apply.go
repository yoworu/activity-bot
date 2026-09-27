package info

import (
	"bytes"
	"html/template"
)

const applicationTemplate = `
──────────────────
       𝗔𝗣𝗣𝗟𝗬
──────────────────

     статус – <u>{{if .Open}}открыт{{else}}закрыт{{end}}</u>
                {{.Current}}/{{.Max}}

     для вступления
     <a href="http://t.me/HavenGateBot?start=true">напишите боту</a>

      ˚₊·— ⁠⁠⁠♡ <a href="https://t.me/h4venflood/41">список ролей</a>

˚₊‧✩ ˚₊‧꒰ა ʚིᵋº‌‌‌‌‌‌ᵌɞྀ ໒꒱ ‧₊˚ ✩‧₊˚•*¨*•.¸¸☆*
`

var applicationTmpl = template.Must(
	template.New("application").Parse(applicationTemplate),
)

type ApplicationPostData struct {
	Current int
	Max     int
	Open    bool
}

func RenderApplication(current, max int) (string, error) {
	var b bytes.Buffer

	err := applicationTmpl.Execute(&b, ApplicationPostData{
		Current: current,
		Max:     max,
		Open:    current < max,
	})

	if err != nil {
		return "", err
	}

	return b.String(), nil
}
