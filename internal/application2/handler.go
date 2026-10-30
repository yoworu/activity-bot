package application2

import (
	"activity-bot/internal/chatmember"
	"activity-bot/internal/roles"
	"fmt"
	"strconv"
	"strings"

	fsm "github.com/fluffur/botapi-fsm"

	"github.com/gotd/botapi"
)

type Handler struct {
	appFSM    *fsm.Machine[State, StateData]
	rejectFSM *fsm.Machine[RejectState, RejectStateData]

	chatMemberService *chatmember.Service
	repository        *Repository
	rolesRepository   roles.Repository

	targetChatID      int64
	applicationChatID int64
	targetChatLink    string
}

func NewHandler(
	appFSM *fsm.Machine[State, StateData],
	rejectFSM *fsm.Machine[RejectState, RejectStateData],
	chatMemberService *chatmember.Service,
	repository *Repository,
	rolesRepository roles.Repository,
	targetChatID int64,
	applicationChatID int64,
	targetChatLink string,
) *Handler {
	return &Handler{
		appFSM:            appFSM,
		rejectFSM:         rejectFSM,
		chatMemberService: chatMemberService,
		repository:        repository,
		rolesRepository:   rolesRepository,
		targetChatID:      targetChatID,
		applicationChatID: applicationChatID,
		targetChatLink:    targetChatLink,
	}
}

func (h *Handler) Start(c *botapi.Context) error {
	sess, ok, err := h.appFSM.Get(c)
	if err != nil {
		return err
	}
	if ok && sess.State == StatePending {
		_, err := c.Reply(
			"Пожалуйста, подождите пока вашу заявку обработают, перед тем как отправлять еще одну",
		)
		return err
	}

	if err := h.appFSM.Enter(c, StateAwaitRole, StateData{}); err != nil {
		return err
	}

	return h.sendRolePrompt(c)
}

func (h *Handler) StartCallback(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil || cq.Message == nil {
		return nil
	}

	if err := h.appFSM.Enter(c, StateAwaitRole, StateData{}); err != nil {
		return err
	}

	if _, err := c.Bot.EditMessageReplyMarkup(
		c,
		botapi.ID(cq.Message.Chat.ID),
		cq.Message.MessageID,
		nil,
	); err != nil {
		return fmt.Errorf("remove old keyboard: %w", err)
	}

	if err := h.sendRolePromptTo(c, cq.From.ID); err != nil {
		return err
	}

	return c.AnswerCallback(
		botapi.WithCallbackText("Можно отправить новую заявку"),
	)
}

func (h *Handler) ShowRegions(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil || cq.Message == nil {
		return nil
	}

	if err := h.appFSM.Enter(c, StateAwaitRole, StateData{}); err != nil {
		return err
	}

	if err := h.editRolePrompt(c, cq.Message.Chat.ID, cq.Message.MessageID); err != nil {
		return err
	}

	return c.AnswerCallback()
}

func (h *Handler) SelectRegion(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil || cq.Message == nil {
		return nil
	}

	categoryID, err := strconv.ParseInt(
		strings.TrimPrefix(cq.Data, callbackRegionPrefix),
		10,
		64,
	)
	if err != nil {
		return fmt.Errorf("parse region: %w", err)
	}

	fandom, err := h.loadFandom(c)
	if err != nil {
		return err
	}

	var category *roles.Category

	for i := range fandom.Categories {
		if fandom.Categories[i].ID == categoryID {
			category = &fandom.Categories[i]
			break
		}
	}

	if category == nil {
		return c.AnswerCallback(
			botapi.WithCallbackText("Регион не найден"),
		)
	}

	available, err := h.availableRoles(c, category.Roles)
	if err != nil {
		return err
	}

	if err := h.appFSM.Enter(c, StateAwaitRole, StateData{
		CategoryID: category.ID,
	}); err != nil {
		return err
	}

	text := "Выберите роль"

	if len(available) == 0 {
		text = "В этом регионе нет свободных ролей"
	}

	if _, err := c.Bot.EditMessageText(
		c,
		botapi.ID(cq.Message.Chat.ID),
		cq.Message.MessageID,
		text,
		botapi.WithReplyMarkup(roleKeyboard(available)),
	); err != nil {
		return fmt.Errorf("edit role keyboard: %w", err)
	}

	return c.AnswerCallback()
}

func (h *Handler) SelectRole(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil || cq.Message == nil {
		return nil
	}

	roleID, err := strconv.ParseInt(
		strings.TrimPrefix(cq.Data, callbackRolePrefix),
		10,
		64,
	)
	if err != nil {
		return fmt.Errorf("parse role: %w", err)
	}

	sess, ok, err := h.appFSM.Get(c)
	if err != nil {
		return err
	}
	if !ok || sess.State != StateAwaitRole {
		return c.AnswerCallback(
			botapi.WithCallbackText("Сначала выберите регион"),
		)
	}

	fandom, err := h.loadFandom(c)
	if err != nil {
		return err
	}

	foundRole, ok := findRoleInFandom(fandom, sess.Data.CategoryID, roleID)
	if !ok {
		return c.AnswerCallback(
			botapi.WithCallbackText("Роль не найдена"),
		)
	}

	available, err := h.availableRoles(c, []roles.Role{foundRole})
	if err != nil {
		return err
	}
	if len(available) == 0 {
		_ = c.AnswerCallback(
			botapi.WithCallbackText("Эта роль уже занята или забронирована"),
		)

		return h.refreshRoleKeyboard(c, sess.Data.CategoryID)
	}

	if err := h.appFSM.Enter(c, StateAwaitAtmosphere, StateData{
		CategoryID: sess.Data.CategoryID,
		Role:       foundRole,
	}); err != nil {
		return err
	}

	if _, err := c.Bot.EditMessageText(
		c,
		botapi.ID(cq.Message.Chat.ID),
		cq.Message.MessageID,
		fmt.Sprintf(
			"Выбрана роль: %s\n\nЧто для вас важно в атмосфере общего чата?",
			foundRole.Name,
		),
	); err != nil {
		return fmt.Errorf("edit atmosphere prompt: %w", err)
	}

	return c.AnswerCallback()
}

func (h *Handler) Atmosphere(c *botapi.Context) error {
	msg := c.Message()
	if msg == nil {
		return nil
	}

	sess, ok, err := h.appFSM.Get(c)
	if err != nil {
		return err
	}
	if !ok || sess.State != StateAwaitAtmosphere {
		return nil
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		_, err := c.Reply("Ответ не может быть пустым.")
		return err
	}

	sess.Data.Atmosphere = text

	if err := h.appFSM.Enter(c, StateAwaitWasInFloods, sess.Data); err != nil {
		return err
	}

	_, err = c.Reply(
		"Бывали ли вы раньше в подобных флудах?",
		botapi.WithReplyMarkup(floodsKeyboard()),
	)

	return err
}

func (h *Handler) WasInFloods(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil || cq.Message == nil {
		return nil
	}

	sess, ok, err := h.appFSM.Get(c)
	if err != nil {
		return err
	}
	if !ok || sess.State != StateAwaitWasInFloods {
		return c.AnswerCallback(
			botapi.WithCallbackText("Сначала ответьте на предыдущий вопрос"),
		)
	}

	switch cq.Data {
	case callbackFloodYes:
		sess.Data.WasInFloods = true

		if err := h.appFSM.Enter(c, StateAwaitLeaveReason, sess.Data); err != nil {
			return err
		}

		if _, err := c.Bot.EditMessageText(
			c,
			botapi.ID(cq.Message.Chat.ID),
			cq.Message.MessageID,
			fmt.Sprintf(
				"Выбрана роль: %s\n\nЧто стало причиной ухода или дискомфорта?",
				sess.Data.Role.Name,
			),
		); err != nil {
			return fmt.Errorf("edit leave reason prompt: %w", err)
		}

		return c.AnswerCallback()

	case callbackFloodNo:
		sess.Data.WasInFloods = false

		if err := h.submit(c, sess.Data, cq.Message.MessageID); err != nil {
			return err
		}

		return c.AnswerCallback()
	}

	return c.AnswerCallback()
}

func (h *Handler) LeaveReason(c *botapi.Context) error {
	msg := c.Message()
	if msg == nil {
		return nil
	}

	sess, ok, err := h.appFSM.Get(c)
	if err != nil {
		return err
	}
	if !ok || sess.State != StateAwaitLeaveReason {
		return nil
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		_, err := c.Reply("Ответ не может быть пустым.")
		return err
	}

	sess.Data.WasInFloods = true
	sess.Data.LeaveReason = text

	return h.submit(c, sess.Data, 0)
}

func (h *Handler) submit(
	c *botapi.Context,
	data StateData,
	messageID int,
) error {
	if data.Role.Name == "" {
		return fmt.Errorf("application role is empty")
	}

	var senderID int64
	var username, firstname, lastname string

	if sender := c.Sender(); sender != nil {
		username = sender.Username
		firstname = sender.FirstName
		lastname = sender.LastName
		senderID = sender.ID
	} else if msg := c.Message(); msg != nil {
		username = msg.Chat.Username
		firstname = msg.Chat.FirstName
		lastname = msg.Chat.LastName
		senderID = msg.Chat.ID
	}

	var userRef string

	if username != "" {
		userRef = "@" + username
	} else {
		userRef = strings.TrimSpace(firstname + " " + lastname)

		if userRef == "" {
			userRef = fmt.Sprintf("ID: %d", senderID)
		}
	}

	app := Application{
		UserID:      senderID,
		ChatID:      h.targetChatID,
		Role:        data.Role,
		Username:    username,
		Atmosphere:  data.Atmosphere,
		WasInFloods: data.WasInFloods,
		LeaveReason: data.LeaveReason,
	}

	adminMsg := fmt.Sprintf(
		"Новая заявка на вступление!\n\n"+
			"Роль: %s\n"+
			"Что важно в атмосфере: %s\n"+
			"Бывали раньше в подобных флудах: %s\n"+
			"Пользователь: %s",
		data.Role.Name,
		data.Atmosphere,
		yesNo(data.WasInFloods),
		userRef,
	)

	if data.WasInFloods && data.LeaveReason != "" {
		adminMsg = fmt.Sprintf(
			"%s\nПричина ухода или дискомфорта: %s",
			adminMsg,
			data.LeaveReason,
		)
	}

	appID := strconv.FormatInt(senderID, 10)

	keyboard := botapi.WithReplyMarkup(
		botapi.InlineKeyboard(
			botapi.InlineRow(
				botapi.InlineButtonData(
					"Принять",
					callbackAcceptPrefix+appID,
				),
				botapi.InlineButtonData(
					"Отклонить",
					callbackRejectPrefix+appID,
				),
			),
		),
	)

	if _, err := c.Bot.SendMessage(
		c,
		botapi.ID(h.applicationChatID),
		adminMsg,
		keyboard,
	); err != nil {
		return fmt.Errorf("send application: %w", err)
	}

	if err := h.appFSM.Enter(c, StatePending, StateData{}); err != nil {
		return err
	}

	if err := h.repository.Save(c, app); err != nil {
		return err
	}

	chatID, _ := c.Chat()

	_, err := c.Bot.SendMessage(
		c,
		chatID,
		"Ваша заявка отправлена на рассмотрение. Ожидайте скорого ответа",
	)
	if err != nil {
		return err
	}

	if messageID != 0 {
		_, _ = c.Bot.EditMessageReplyMarkup(
			c,
			chatID,
			messageID,
			nil,
		)
	}

	return nil
}

func (h *Handler) Accept(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil || cq.Message == nil {
		return nil
	}

	userID, err := strconv.ParseInt(
		strings.TrimPrefix(cq.Data, callbackAcceptPrefix),
		10,
		64,
	)
	if err != nil {
		return fmt.Errorf("accept: %w", err)
	}

	application, err := h.repository.Get(c, h.targetChatID, userID)
	if err != nil {
		return err
	}
	if application == nil {
		return c.AnswerCallback(
			botapi.WithCallbackText("Заявка не найдена или устарела"),
		)
	}

	text := "Ваша заявка принята, добро пожаловать!"
	if h.targetChatLink != "" {
		text = fmt.Sprintf("%s\n%s", text, h.targetChatLink)
	}

	if _, err := c.Bot.SendMessage(
		c,
		botapi.ID(application.UserID),
		text,
	); err != nil {
		return fmt.Errorf("notify applicant: %w", err)
	}

	if err := h.appFSM.ClearByKey(
		c.Background(),
		application.UserID,
	); err != nil {
		return err
	}

	chatID, _ := c.Chat()

	if _, err := c.Bot.EditMessageReplyMarkup(
		c,
		botapi.ID(cq.Message.Chat.ID),
		cq.Message.MessageID,
		nil,
	); err != nil {
		return err
	}

	if _, err := c.Bot.SendMessage(
		c,
		chatID,
		"Заявка успешно принята",
		botapi.ReplyTo(cq.Message.MessageID),
	); err != nil {
		return err
	}

	return c.AnswerCallback(
		botapi.WithCallbackText("Заявка принята"),
	)
}

func (h *Handler) Reject(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil || cq.Message == nil {
		return nil
	}

	userID, err := strconv.ParseInt(
		strings.TrimPrefix(cq.Data, callbackRejectPrefix),
		10,
		64,
	)
	if err != nil {
		return err
	}

	sess, ok, err := h.appFSM.GetByKey(
		c.Background(),
		userID,
	)
	if err != nil {
		return err
	}

	if !ok || sess.State != StatePending {
		_ = c.AnswerCallback(
			botapi.WithCallbackText("Заявка не найдена или устарела"),
		)

		return nil
	}

	if err := h.rejectFSM.Enter(
		c,
		RejectStateAwaitReason,
		RejectStateData{
			UserID: userID,
			ChatID: h.targetChatID,
		},
	); err != nil {
		return err
	}

	chatID, _ := c.Chat()

	_, _ = c.Bot.EditMessageReplyMarkup(
		c,
		botapi.ID(cq.Message.Chat.ID),
		cq.Message.MessageID,
		nil,
	)

	_, err = c.Bot.SendMessage(
		c,
		chatID,
		"Введите причину отказа:",
	)

	return err
}

func (h *Handler) RejectMessage(c *botapi.Context) error {
	msg := c.Message()
	if msg == nil {
		return nil
	}

	sess, ok, err := h.rejectFSM.Get(c)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	application, err := h.repository.Get(
		c,
		sess.Data.ChatID,
		sess.Data.UserID,
	)
	if err != nil {
		return err
	}
	if application == nil {
		return nil
	}

	reason := strings.TrimSpace(msg.Text)
	if reason == "" {
		_, err := c.Reply("Причина не может быть пустой.")
		return err
	}

	if _, err := c.Bot.SendMessage(
		c,
		botapi.ID(application.UserID),
		fmt.Sprintf(
			"К сожалению, ваша заявка была отклонена.\n\nПричина: %s",
			reason,
		),
		botapi.WithReplyMarkup(
			botapi.InlineKeyboard(
				botapi.InlineRow(
					botapi.InlineButtonData(
						"📨 Отправить новую заявку",
						callbackNew,
					),
				),
			),
		),
	); err != nil {
		return fmt.Errorf("notify applicant rejection: %w", err)
	}

	if err := h.appFSM.ClearByKey(
		c.Background(),
		application.UserID,
	); err != nil {
		return err
	}

	if err := h.rejectFSM.Clear(c); err != nil {
		return err
	}

	if _, err := c.Reply("Заявка отклонена."); err != nil {
		return err
	}

	return h.repository.Delete(
		c,
		application.ChatID,
		application.UserID,
	)
}

func (h *Handler) sendRolePrompt(c *botapi.Context) error {
	categories, err := h.availableCategories(c)
	if err != nil {
		return err
	}

	text := "Выберите роль"
	if len(categories) == 0 {
		text = "Сейчас нет свободных ролей"
	}

	_, err = c.Reply(
		text,
		botapi.WithReplyMarkup(categoryKeyboard(categories)),
	)

	return err
}

func (h *Handler) sendRolePromptTo(c *botapi.Context, chatID int64) error {
	categories, err := h.availableCategories(c)
	if err != nil {
		return err
	}

	text := "Выберите роль"
	if len(categories) == 0 {
		text = "Сейчас нет свободных ролей"
	}

	_, err = c.Bot.SendMessage(
		c,
		botapi.ID(chatID),
		text,
		botapi.WithReplyMarkup(categoryKeyboard(categories)),
	)

	return err
}

func (h *Handler) editRolePrompt(
	c *botapi.Context,
	chatID int64,
	messageID int,
) error {
	categories, err := h.availableCategories(c)
	if err != nil {
		return err
	}

	text := "Выберите роль"
	if len(categories) == 0 {
		text = "Сейчас нет свободных ролей"
	}

	_, err = c.Bot.EditMessageText(
		c,
		botapi.ID(chatID),
		messageID,
		text,
		botapi.WithReplyMarkup(categoryKeyboard(categories)),
	)
	if err != nil {
		return fmt.Errorf("edit role keyboard: %w", err)
	}

	return nil
}

func (h *Handler) refreshRoleKeyboard(c *botapi.Context, categoryID int64) error {
	cq := c.Update.CallbackQuery
	if cq == nil || cq.Message == nil {
		return nil
	}

	fandom, err := h.loadFandom(c)
	if err != nil {
		return err
	}

	var category *roles.Category

	for i := range fandom.Categories {
		if fandom.Categories[i].ID == categoryID {
			category = &fandom.Categories[i]
			break
		}
	}

	if category == nil {
		return h.editRolePrompt(c, cq.Message.Chat.ID, cq.Message.MessageID)
	}

	available, err := h.availableRoles(c, category.Roles)
	if err != nil {
		return err
	}

	text := "Выберите роль"

	if len(available) == 0 {
		text = "В этом регионе нет свободных ролей"
	}

	_, err = c.Bot.EditMessageText(
		c,
		botapi.ID(cq.Message.Chat.ID),
		cq.Message.MessageID,
		text,
		botapi.WithReplyMarkup(roleKeyboard(available)),
	)
	if err != nil {
		return fmt.Errorf("refresh role keyboard: %w", err)
	}

	return nil
}

func yesNo(value bool) string {
	if value {
		return "Да"
	}

	return "Нет"
}
