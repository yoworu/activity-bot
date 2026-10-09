package summon

import (
	"activity-bot/internal/cctx"
	"activity-bot/internal/chat"
	"activity-bot/internal/chatmember"
	"activity-bot/internal/i18n"
	"activity-bot/internal/utils/tghtml"
	"context"

	tdhtml "github.com/gotd/td/telegram/message/html"

	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/log"
	"github.com/gotd/td/constant"
	"github.com/gotd/td/telegram/message/entity"
	"github.com/gotd/td/tg"
	"golang.org/x/time/rate"

	"github.com/gotd/botapi"
)

func AppendHTML(eb *entity.Builder, html string) error {
	return tdhtml.HTML(
		strings.NewReader(html),
		eb,
		tdhtml.Options{},
	)
}

func TextFromArgs(ch chat.Chat, args botapi.Message) string {
	text := args.OriginalTextHTML()

	if strings.TrimSpace(text) == "" {
		text = ch.WelcomeCallMessage
	}

	if strings.TrimSpace(text) == "" && ch.MentionTypes == 0 {
		text = "ㅤ"
	}

	return text
}

func (h *Handler) Summon(
	c *botapi.Context,
	text string,
	msgID int,
	ch chat.Chat,
	cms []chatmember.ChatMember,
) error {
	chatID, ok := c.Chat()
	if !ok {
		return nil
	}

	return h.RunSummon(
		c.Background(),
		c.Bot,
		cctx.MustLocalizer(c),
		int64(chatID.(botapi.ChatIDInt)),
		msgID,
		text,
		ch,
		cms,
	)
}

func (h *Handler) RunSummon(
	ctx context.Context,
	bot *botapi.Bot,
	loc *i18n.Localizer,
	chatID int64,
	msgID int,
	text string,
	ch chat.Chat,
	cms []chatmember.ChatMember,
) error {
	perMsg := int(ch.MentionsPerMessage)
	if perMsg <= 0 {
		perMsg = len(cms)
	}

	groups := chunk(cms, perMsg)

	go func() {
		if err := SendMessages(
			ctx,
			bot,
			loc,
			chatID,
			msgID,
			text,
			ch.MentionTypes,
			groups,
		); err != nil {
			log.For(bot.Logger()).Error(ctx, "send messages", log.Error(err))
		}
	}()

	return nil
}

var customEmojiRegexp = regexp.MustCompile(
	`<tg-emoji emoji-id="(\d+)">(.*?)</tg-emoji>`,
)

func ParseCustomEmoji(s string) (int64, string) {
	m := customEmojiRegexp.FindStringSubmatch(s)
	if len(m) != 3 {
		return 0, ""
	}

	id, _ := strconv.ParseInt(m[1], 10, 64)

	return id, m[2]
}

func BuildMentionMessage(
	eb *entity.Builder,
	text string,
	members []chatmember.ChatMember,
	mentionTypes chat.MentionTypes,
) {
	if strings.TrimSpace(text) != "" && text != "ㅤ" {
		if err := AppendHTML(eb, text); err != nil {
			eb.Plain(text)
		}

		if mentionTypes != 0 {
			eb.Plain("\n\n")
		}
	}

	sep := " "
	if mentionTypes.Has(chat.MentionName) || mentionTypes.Has(chat.MentionRole) {
		sep = ", "
	}

	for i, cm := range members {
		RenderMentionEB(eb, cm, mentionTypes)

		if i != len(members)-1 {
			eb.Plain(sep)
		}
	}

	eb.Plain("ㅤ")
}

func SendMessages(
	ctx context.Context,
	bot *botapi.Bot,
	loc *i18n.Localizer,
	chatID int64,
	msgID int,
	text string,
	mentionTypes chat.MentionTypes,
	groups [][]chatmember.ChatMember,
) error {
	limiter := rate.NewLimiter(rate.Every(1500*time.Millisecond), 3)

	peer, err := bot.Peers().ResolveTDLibID(
		ctx,
		constant.TDLibPeerID(chatID),
	)
	if err != nil {
		return err
	}

	inputPeer := peer.InputPeer()

	for _, group := range groups {
		if err := limiter.Wait(ctx); err != nil {
			return err
		}

		eb := &entity.Builder{}

		BuildMentionMessage(
			eb,
			text,
			group,
			mentionTypes,
		)

		//if i == len(groups)-1 {
		//	eb.Plain("\n\n")
		//	eb.Plain(
		//		loc.T(i18n.Cmd.Summon.Completed, nil),
		//	)
		//}

		finalText, entities := eb.Complete()

		opts := &tg.MessagesSendMessageRequest{
			Peer:     inputPeer,
			Message:  finalText,
			Entities: entities,
			RandomID: rand.Int63(),
		}

		if msgID != 0 {
			opts.ReplyTo = &tg.InputReplyToMessage{
				ReplyToMsgID: msgID,
			}
		}
		_, err = bot.Raw().MessagesSendMessage(
			ctx,
			opts,
		)

		if err != nil {
			return err
		}
	}

	return nil
}

func RenderMention(cm chatmember.ChatMember, mentionTypes chat.MentionTypes) string {
	hasEmoji := mentionTypes.Has(chat.MentionEmoji)
	hasRole := mentionTypes.Has(chat.MentionRole)
	hasName := mentionTypes.Has(chat.MentionName)

	var text strings.Builder

	role := cm.Role()
	if role == "" {
		role = cm.User.FullName()
	}

	switch {
	case hasRole && hasName:
		if role == cm.User.FullName() {
			text.WriteString(role)
		} else {
			text.WriteString(fmt.Sprintf("%s (%s)", role, cm.User.FullName()))
		}

	case hasRole:
		text.WriteString(role)

	case hasName:
		text.WriteString(cm.User.FullName())
	}

	mentionText := strings.TrimSpace(text.String())

	if mentionText == "" {
		if hasEmoji {
			if emoji := cm.AnyEmoji(); emoji != "" && !strings.Contains(emoji, "<tg-emoji") {
				return tghtml.UserMention(cm.ID(), emoji)
			}
		}

		mentionText = "\u200B"
	}

	mention := tghtml.UserMention(cm.ID(), mentionText)

	if hasEmoji {
		if emoji := cm.AnyEmoji(); emoji != "" {
			return emoji + " " + mention
		}
	}

	return mention
}

func RenderMentionEB(
	eb *entity.Builder,
	cm chatmember.ChatMember,
	mentionTypes chat.MentionTypes,
) {
	hasEmoji := mentionTypes.Has(chat.MentionEmoji)
	hasRole := mentionTypes.Has(chat.MentionRole)
	hasName := mentionTypes.Has(chat.MentionName)

	role := cm.Role()
	if role == "" {
		role = cm.User.FullName()
	}

	var title string

	switch {
	case hasRole && hasName:
		if role == cm.User.FullName() {
			title = role
		} else {
			title = fmt.Sprintf("%s (%s)", role, cm.User.FullName())
		}

	case hasRole:
		title = role

	case hasName:
		title = cm.User.FullName()
	}

	if !hasEmoji {
		writeMention(eb, cm.ID(), title)
		return
	}

	emoji := cm.AnyEmoji()

	if emoji == "" {
		writeMention(eb, cm.ID(), title)
		return
	}

	if strings.Contains(emoji, "<tg-emoji") {
		id, char := ParseCustomEmoji(emoji)

		eb.CustomEmoji(char, id)

		if title != "" {
			writeMention(eb, cm.ID(), title)
		} else {
			writeMention(eb, cm.ID(), "\u200B")
		}

		return
	}

	// unicode emoji

	if title == "" {
		writeMention(eb, cm.ID(), emoji)
		return
	}

	eb.Plain(emoji)
	writeMention(eb, cm.ID(), title)
}

func writeMention(eb *entity.Builder, id int64, text string) {
	if text == "" {
		text = "\u200B"
	}

	eb.MentionName(text, &tg.InputUser{
		UserID: id,
	})
}
func BuildMentions(
	cms []chatmember.ChatMember,
	mentionTypes chat.MentionTypes,
) []string {
	mentions := make([]string, len(cms))

	for i, cm := range cms {
		mentions[i] = RenderMention(cm, mentionTypes)
	}

	return mentions
}

func check(enabled bool) string {
	if enabled {
		return "✅"
	}

	return ""
}

func MentionSeparator(mt chat.MentionTypes) string {
	if mt.Has(chat.MentionEmoji) && !mt.Has(chat.MentionRole) && !mt.Has(chat.MentionName) || mt == 0 {
		return " "
	}

	return ", "
}

func chunk[T any](items []T, size int) [][]T {
	if size <= 0 {
		return [][]T{items}
	}

	var chunks [][]T

	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}

		chunks = append(chunks, items[i:end])
	}

	return chunks
}

func (h *Handler) summonSession(
	c *botapi.Context,
) (*StateData, chat.Chat, error) {
	session, ok, err := h.summonFSM.Get(c)
	if err != nil {
		return nil, chat.Chat{}, err
	}

	if !ok {
		return nil, chat.Chat{}, nil
	}

	ch, err := h.chatService.Get(c, session.Data.ChatID)
	if err != nil {
		return nil, chat.Chat{}, fmt.Errorf("get chat: %w", err)
	}

	sender := c.Sender()
	if sender == nil {
		return nil, ch, nil
	}

	if session.Data.UserID != sender.ID {
		loc := cctx.MustLocalizer(c)

		return nil, ch, c.AnswerCallback(
			botapi.WithCallbackText(
				loc.T(i18n.Cmd.Summon.Confirm.OnlyInitiatorConfirm, nil),
			),
		)
	}

	return &session.Data, ch, nil
}
