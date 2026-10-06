package info

import (
	"activity-bot/internal/chatmember"
	"activity-bot/internal/utils/tghtml"
	"bytes"
	"text/template"
	"time"
)

type RestRenderData struct {
	Members []RestMember
}

type RestMember struct {
	Name  string
	Until string
}

const restsTemplate = `𓇬⋆ ﾟ꩜‧˚𑁍ܓ ˖ 𖥔 ˖ 𐙚 ˖ 𖥔 ˖

<blockquote>рест – временное освобождение от нормы</blockquote>
рест можно взять в среднем до 2-3 недель

{{range .Members}}{{.Name}} — до {{.Until}}
{{end}}⋆ если нужен рест на больший срок, обсуждайте этот вопрос с администрацией

.｡:･*ﾟﾟ*･.｡
`

var restsTmpl = template.Must(
	template.New("rests").Parse(restsTemplate),
)

func BuildRestMembers(members []chatmember.ChatMember) []RestMember {
	rests := make([]RestMember, 0)
	now := time.Now()
	for _, member := range members {
		if !member.IsResting(now) || member.IsLeft() {
			continue
		}

		rests = append(rests, RestMember{
			Name:  member.Display("", false),
			Until: tghtml.DefaultDateTime(member.RestUntil),
		})
	}

	return rests
}

func RenderRests(members []RestMember) (string, error) {
	var buf bytes.Buffer

	err := restsTmpl.Execute(&buf, RestRenderData{
		Members: members,
	})

	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
