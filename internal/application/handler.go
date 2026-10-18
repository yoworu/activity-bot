package application

import (
	"activity-bot/internal/chatmember"
	"activity-bot/internal/info"
	"activity-bot/internal/roles"
	"activity-bot/internal/utils/tghtml"
	"fmt"
	"strconv"
	"strings"
	"time"

	fsm "github.com/fluffur/botapi-fsm"

	"github.com/gotd/botapi"
)

type Handler struct {
	appFSM    *fsm.Machine[State, AppStateData]
	rejectFSM *fsm.Machine[RejectState, RejectStateData]

	chatMemberService *chatmember.Service
	repository        *Repository
	rolesRepository   roles.Repository
	updater           *info.Updater

	targetChatID      int64
	targetChatLink    string
	applicationChatID int64
	rolesPostLink     string
}

func NewHandler(
	appFSM *fsm.Machine[State, AppStateData],
	rejectFSM *fsm.Machine[RejectState, RejectStateData],
	chatMemberService *chatmember.Service,
	repository *Repository,
	rolesRepository roles.Repository,
	updater *info.Updater,
	targetChatID int64,
	applicationChatID int64,
	targetChatLink string,
	rolesPostLink string,
) *Handler {
	return &Handler{
		appFSM:            appFSM,
		rejectFSM:         rejectFSM,
		chatMemberService: chatMemberService,
		targetChatID:      targetChatID,
		updater:           updater,
		applicationChatID: applicationChatID,
		rolesPostLink:     rolesPostLink,
		targetChatLink:    targetChatLink,
		repository:        repository,
		rolesRepository:   rolesRepository,
	}
}

func (h *Handler) Start(c *botapi.Context) error {
	sess, ok, err := h.appFSM.Get(c)
	if err != nil {
		return err
	}
	if ok && sess.State == AppStatePending {
		_, err := c.Reply(
			"Пожалуйста, подождите пока вашу заявку обработают, перед тем как отправлять еще одну",
		)
		return err
	}

	if err := h.appFSM.Enter(
		c,
		AppStateAwaitAction,
		AppStateData{},
	); err != nil {
		return err
	}

	return h.sendActionPrompt(c)
}

func (h *Handler) StartCallback(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil {
		return nil
	}

	if err := h.appFSM.Enter(
		c,
		AppStateAwaitAction,
		AppStateData{},
	); err != nil {
		return err
	}

	_, err := c.Bot.EditMessageReplyMarkup(
		c,
		botapi.ID(cq.Message.Chat.ID),
		cq.Message.MessageID,
		nil,
	)
	if err != nil {
		return fmt.Errorf("remove old keyboard: %w", err)
	}

	if err := h.sendActionPromptTo(c, cq.From.ID); err != nil {
		return err
	}

	return c.AnswerCallback(
		botapi.WithCallbackText(
			"Можно отправить новую заявку",
		),
	)
}

func (h *Handler) ShowActions(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil {
		return nil
	}

	chatID, _ := c.Chat()
	chatIDInt := chatID.(botapi.ChatIDInt)

	if err := h.appFSM.Enter(
		c,
		AppStateAwaitAction,
		AppStateData{},
	); err != nil {
		return err
	}

	if err := h.editActionPrompt(c, int64(chatIDInt), cq.Message.MessageID); err != nil {
		return err
	}

	return c.AnswerCallback()
}

func (h *Handler) SelectAction(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil || cq.Message == nil {
		return nil
	}

	kind := KindJoin
	if cq.Data == callbackActionReserve {
		kind = KindReserve
	}

	if err := h.appFSM.Enter(
		c,
		AppStateAwaitRole,
		AppStateData{Kind: kind},
	); err != nil {
		return err
	}

	if err := h.editRegionPrompt(c, cq.Message.Chat.ID, cq.Message.MessageID); err != nil {
		return err
	}

	return c.AnswerCallback()
}

func (h *Handler) ShowRegions(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil {
		return nil
	}

	chatID, _ := c.Chat()
	chatIDInt := chatID.(botapi.ChatIDInt)

	if err := h.editRegionPrompt(c, int64(chatIDInt), cq.Message.MessageID); err != nil {
		return err
	}

	return c.AnswerCallback()
}

func (h *Handler) SelectRegion(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil {
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

	fandom, err := h.loadGenshinFandom(c)
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

	kind := KindJoin
	if sess, ok, err := h.appFSM.Get(c); err != nil {
		return err
	} else if ok {
		kind = sess.Data.Kind
	}

	if err := h.appFSM.Enter(
		c,
		AppStateAwaitRole,
		AppStateData{
			Kind:       kind,
			CategoryID: category.ID,
		},
	); err != nil {
		return err
	}

	text := "Выберите роль"

	if len(available) == 0 {
		text = "В этом регионе нет свободных ролей"
	}

	chatID, _ := c.Chat()

	if _, err := c.Bot.EditMessageText(
		c,
		chatID,
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
	if !ok || sess.State != AppStateAwaitRole {
		return c.AnswerCallback(
			botapi.WithCallbackText("Сначала выберите регион"),
		)
	}

	fandom, err := h.loadGenshinFandom(c)
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

	if err := h.appFSM.Enter(
		c,
		AppStateAwaitBirthDate,
		AppStateData{
			Kind:       sess.Data.Kind,
			CategoryID: sess.Data.CategoryID,
			Role:       foundRole,
		},
	); err != nil {
		return err
	}

	now := time.Now()

	if _, err := c.Bot.EditMessageText(
		c,
		botapi.ID(cq.Message.Chat.ID),
		cq.Message.MessageID,
		fmt.Sprintf("Выбрана роль: %s\n\nВыберите год рождения", foundRole.Name),
		botapi.WithReplyMarkup(yearKeyboard(defaultBirthYearDecade(now), now)),
	); err != nil {
		return fmt.Errorf("edit birth year keyboard: %w", err)
	}

	return c.AnswerCallback()
}

func (h *Handler) BirthDate(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil || cq.Message == nil {
		return nil
	}

	step, err := parseBirthDateCallback(cq.Data)
	if err != nil {
		return fmt.Errorf("parse birth date: %w", err)
	}

	if step.Kind == birthDateNop {
		return c.AnswerCallback()
	}

	sess, ok, err := h.appFSM.Get(c)
	if err != nil {
		return err
	}
	if !ok || sess.State != AppStateAwaitBirthDate {
		return c.AnswerCallback(
			botapi.WithCallbackText("Сначала выберите роль"),
		)
	}

	now := time.Now()

	switch step.Kind {
	case birthDateRoles:
		if err := h.appFSM.Enter(
			c,
			AppStateAwaitRole,
			AppStateData{
				Kind:       sess.Data.Kind,
				CategoryID: sess.Data.CategoryID,
			},
		); err != nil {
			return err
		}

		if err := h.refreshRoleKeyboard(c, sess.Data.CategoryID); err != nil {
			return err
		}

		return c.AnswerCallback()

	case birthDateYears:
		decade := step.Decade
		if decade == 0 {
			decade = defaultBirthYearDecade(now)
		}

		if _, err := c.Bot.EditMessageText(
			c,
			botapi.ID(cq.Message.Chat.ID),
			cq.Message.MessageID,
			fmt.Sprintf("Выбрана роль: %s\n\nВыберите год рождения", sess.Data.Role.Name),
			botapi.WithReplyMarkup(yearKeyboard(decade, now)),
		); err != nil {
			return fmt.Errorf("edit birth year keyboard: %w", err)
		}

		return c.AnswerCallback()

	case birthDateMonths:
		if step.Year < minBirthYear(now) || step.Year > maxBirthYear(now) {
			return c.AnswerCallback(
				botapi.WithCallbackText("Некорректный год"),
			)
		}

		if _, err := c.Bot.EditMessageText(
			c,
			botapi.ID(cq.Message.Chat.ID),
			cq.Message.MessageID,
			fmt.Sprintf("Выбрана роль: %s\n\nВыберите месяц (%d)", sess.Data.Role.Name, step.Year),
			botapi.WithReplyMarkup(monthKeyboard(step.Year)),
		); err != nil {
			return fmt.Errorf("edit birth month keyboard: %w", err)
		}

		return c.AnswerCallback()

	case birthDateDays:
		if !validBirthDate(step.Year, step.Month, 1, now) {
			return c.AnswerCallback(
				botapi.WithCallbackText("Некорректный месяц"),
			)
		}

		if _, err := c.Bot.EditMessageText(
			c,
			botapi.ID(cq.Message.Chat.ID),
			cq.Message.MessageID,
			fmt.Sprintf(
				"Выбрана роль: %s\n\nВыберите день (%s %d)",
				sess.Data.Role.Name,
				strings.ToLower(monthNames[step.Month]),
				step.Year,
			),
			botapi.WithReplyMarkup(dayKeyboard(step.Year, step.Month)),
		); err != nil {
			return fmt.Errorf("edit birth day keyboard: %w", err)
		}

		return c.AnswerCallback()

	case birthDatePick:
		if !validBirthDate(step.Year, step.Month, step.Day, now) {
			return c.AnswerCallback(
				botapi.WithCallbackText("Некорректная дата"),
			)
		}

		sess.Data.BirthDate = formatBirthDate(step.Year, step.Month, step.Day)

		if sess.Data.isReserve() {
			if err := h.appFSM.Enter(
				c,
				AppStateAwaitReserveUntil,
				sess.Data,
			); err != nil {
				return err
			}

			if err := h.editReserveUntilPrompt(
				c,
				cq.Message.Chat.ID,
				cq.Message.MessageID,
				sess.Data,
				now.Year(),
				int(now.Month()),
				now,
			); err != nil {
				return err
			}

			return c.AnswerCallback()
		}

		if err := h.appFSM.Enter(
			c,
			AppStateConfirmRules,
			sess.Data,
		); err != nil {
			return err
		}

		if err := h.editConfirmRules(
			c,
			cq.Message.Chat.ID,
			cq.Message.MessageID,
			sess.Data,
		); err != nil {
			return err
		}

		return c.AnswerCallback()
	}

	return c.AnswerCallback()
}

func (h *Handler) ReserveUntil(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil || cq.Message == nil {
		return nil
	}

	step, err := parseReserveUntilCallback(cq.Data)
	if err != nil {
		return fmt.Errorf("parse reserve until: %w", err)
	}

	if step.Kind == reserveUntilNop {
		return c.AnswerCallback()
	}

	sess, ok, err := h.appFSM.Get(c)
	if err != nil {
		return err
	}
	if !ok || sess.State != AppStateAwaitReserveUntil {
		return c.AnswerCallback(
			botapi.WithCallbackText("Сначала выберите дату рождения"),
		)
	}

	now := time.Now()

	switch step.Kind {
	case reserveUntilBack:
		if err := h.appFSM.Enter(
			c,
			AppStateAwaitBirthDate,
			sess.Data,
		); err != nil {
			return err
		}

		if _, err := c.Bot.EditMessageText(
			c,
			botapi.ID(cq.Message.Chat.ID),
			cq.Message.MessageID,
			fmt.Sprintf("Выбрана роль: %s\n\nВыберите год рождения", sess.Data.Role.Name),
			botapi.WithReplyMarkup(yearKeyboard(defaultBirthYearDecade(now), now)),
		); err != nil {
			return fmt.Errorf("edit birth year keyboard: %w", err)
		}

		return c.AnswerCallback()

	case reserveUntilMonth:
		if err := h.editReserveUntilPrompt(
			c,
			cq.Message.Chat.ID,
			cq.Message.MessageID,
			sess.Data,
			step.Year,
			step.Month,
			now,
		); err != nil {
			return err
		}

		return c.AnswerCallback()

	case reserveUntilPick:
		if !validReserveUntil(step.Year, step.Month, step.Day, now) {
			return c.AnswerCallback(
				botapi.WithCallbackText("Некорректный срок брони"),
			)
		}

		sess.Data.ReserveUntil = formatReserveUntil(step.Year, step.Month, step.Day)

		if err := h.appFSM.Enter(
			c,
			AppStateConfirmRules,
			sess.Data,
		); err != nil {
			return err
		}

		if err := h.editConfirmRules(
			c,
			cq.Message.Chat.ID,
			cq.Message.MessageID,
			sess.Data,
		); err != nil {
			return err
		}

		return c.AnswerCallback()
	}

	return c.AnswerCallback()
}

func (h *Handler) ConfirmRules(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil {
		return nil
	}

	msg := cq.Message
	if msg == nil {
		return nil
	}

	var senderID int64
	var username, firstname, lastname string

	if sender := c.Sender(); sender != nil {
		username = sender.Username
		firstname = sender.FirstName
		lastname = sender.LastName
		senderID = sender.ID
	} else {
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

	sess, ok, err := h.appFSM.Get(c)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	if sess.Data.BirthDate == "" {
		return fmt.Errorf("application birth date is empty")
	}

	birthDate, err := time.Parse("02.01.2006", sess.Data.BirthDate)
	if err != nil {
		return fmt.Errorf("parse application birth date: %w", err)
	}

	kind := sess.Data.Kind
	if kind == "" {
		kind = KindJoin
	}

	app := Application{
		UserID:    senderID,
		ChatID:    h.targetChatID,
		Kind:      kind,
		Role:      sess.Data.Role,
		Username:  username,
		BirthDate: birthDate,
	}

	adminMsg := fmt.Sprintf(
		"Новая заявка на вступление!\n\n"+
			"Роль: %s\n"+
			"Дата рождения: %s\n"+
			"Пользователь: %s",
		sess.Data.Role.Name,
		sess.Data.BirthDate,
		userRef,
	)

	if sess.Data.isReserve() {
		if sess.Data.ReserveUntil == "" {
			return fmt.Errorf("application reserve until is empty")
		}

		reserveUntil, err := time.Parse("02.01.2006", sess.Data.ReserveUntil)
		if err != nil {
			return fmt.Errorf("parse application reserve until: %w", err)
		}

		app.ReserveUntil = reserveUntil
		adminMsg = fmt.Sprintf(
			"Новая заявка на бронирование роли!\n\n"+
				"Роль: %s\n"+
				"Дата рождения: %s\n"+
				"Срок брони: до %s\n"+
				"Пользователь: %s",
			sess.Data.Role.Name,
			sess.Data.BirthDate,
			sess.Data.ReserveUntil,
			userRef,
		)
	}

	appID := strconv.FormatInt(senderID, 10)

	keyboard := botapi.WithReplyMarkup(
		botapi.InlineKeyboard(
			botapi.InlineRow(
				botapi.InlineButtonData(
					"Принять",
					"app:accept:"+appID,
				),
				botapi.InlineButtonData(
					"Отклонить",
					"app:reject:"+appID,
				),
			),
		),
	)

	_, err = c.Bot.SendMessage(
		c,
		botapi.ID(h.applicationChatID),
		adminMsg,
		keyboard,
	)
	if err != nil {
		return fmt.Errorf("send application: %w", err)
	}

	if err := h.appFSM.Enter(
		c,
		AppStatePending,
		AppStateData{},
	); err != nil {
		return err
	}

	if err := h.repository.Save(c, app); err != nil {
		return err
	}

	chatID, _ := c.Chat()

	_, err = c.Bot.SendMessage(
		c,
		chatID,
		"Ваша заявка отправлена на рассмотрение. Ожидайте скорого ответа",
	)

	_, _ = c.Bot.EditMessageReplyMarkup(
		c,
		botapi.ID(msg.Chat.ID),
		msg.MessageID,
		nil,
	)

	return err
}

func (h *Handler) Accept(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil {
		return nil
	}

	prefix := "app:accept:"
	userID, err := strconv.ParseInt(
		strings.TrimPrefix(cq.Data, prefix),
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
		return c.AnswerCallback()
	}

	if application.IsReserve() {
		return h.acceptReservation(c, application)
	}

	_, err = c.Bot.SendMessage(
		c,
		botapi.ID(application.UserID),
		fmt.Sprintf(
			"Ваша заявка принята, добро пожаловать!\n%s",
			h.targetChatLink,
		),
	)
	if err != nil {
		return fmt.Errorf("notify applicant: %w", err)
	}

	if err := h.appFSM.ClearByKey(
		c.Background(),
		userID,
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
		botapi.WithCallbackText(
			"Заявка принята",
		),
	)
}

func (h *Handler) acceptReservation(c *botapi.Context, application *Application) error {
	cq := c.Update.CallbackQuery

	available, err := h.availableRoles(c, []roles.Role{application.Role})
	if err != nil {
		return err
	}
	if len(available) == 0 {
		return c.AnswerCallback(
			botapi.WithCallbackText("Эта роль уже занята или забронирована"),
		)
	}

	if err := h.rolesRepository.CreateRoleReservation(
		c,
		h.targetChatID,
		application.UserID,
		application.Role.ID,
	); err != nil {
		return fmt.Errorf("create role reservation: %w", err)
	}

	_, err = c.Bot.SendMessage(
		c,
		botapi.ID(application.UserID),
		fmt.Sprintf(
			"Ваша заявка на бронирование роли %s принята.\nСрок брони: до %s",
			application.Role.Name,
			application.ReserveUntil.Format("02.01.2006"),
		),
	)
	if err != nil {
		return fmt.Errorf("notify applicant: %w", err)
	}

	if err := h.appFSM.ClearByKey(
		c.Background(),
		application.UserID,
	); err != nil {
		return err
	}

	if err := h.repository.Delete(
		c,
		application.ChatID,
		application.UserID,
	); err != nil {
		return err
	}

	if _, err := c.Bot.EditMessageReplyMarkup(
		c,
		botapi.ID(cq.Message.Chat.ID),
		cq.Message.MessageID,
		nil,
	); err != nil {
		return err
	}

	chatID, _ := c.Chat()

	if _, err := c.Bot.SendMessage(
		c,
		chatID,
		"Заявка на бронирование принята",
		botapi.ReplyTo(cq.Message.MessageID),
	); err != nil {
		return err
	}

	if err := h.updater.UpdateRolesPost(c, h.targetChatID, c.Bot); err != nil {
		return fmt.Errorf("update roles post: %w", err)
	}

	return c.AnswerCallback(
		botapi.WithCallbackText(
			"Бронь принята",
		),
	)
}

func (h *Handler) Reject(c *botapi.Context) error {
	cq := c.Update.CallbackQuery
	if cq == nil {
		return nil
	}

	userID, err := strconv.ParseInt(
		strings.TrimPrefix(cq.Data, "app:reject:"),
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

	if !ok || sess.State != AppStatePending {
		_ = c.AnswerCallback(
			botapi.WithCallbackText(
				"Заявка не найдена или устарела",
			),
		)

		return nil
	}

	if err := h.rejectFSM.Enter(
		c,
		RejectStateAwaitRejectMessage,
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
		_, err := c.Reply(
			"Причина не может быть пустой.",
		)

		return err
	}

	_, err = c.Bot.SendMessage(
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
						"app:new",
					),
				),
			),
		),
	)
	if err != nil {
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

	_, err = c.Reply(
		"Заявка отклонена.",
	)

	if err := h.repository.Delete(
		c,
		application.ChatID,
		application.UserID,
	); err != nil {
		return err
	}

	return err
}

func (h *Handler) editReserveUntilPrompt(
	c *botapi.Context,
	chatID int64,
	messageID int,
	data AppStateData,
	year, month int,
	now time.Time,
) error {
	_, err := c.Bot.EditMessageText(
		c,
		botapi.ID(chatID),
		messageID,
		fmt.Sprintf(
			"Роль: %s\nДата рождения: %s\n\nВыберите срок брони (%s %d)",
			data.Role.Name,
			data.BirthDate,
			strings.ToLower(monthNames[month]),
			year,
		),
		botapi.WithReplyMarkup(reserveUntilKeyboard(year, month, now)),
	)
	if err != nil {
		return fmt.Errorf("edit reserve until keyboard: %w", err)
	}

	return nil
}

func (h *Handler) editConfirmRules(
	c *botapi.Context,
	chatID int64,
	messageID int,
	data AppStateData,
) error {
	summary := fmt.Sprintf(
		"Роль: %s\nДата рождения: %s",
		data.Role.Name,
		data.BirthDate,
	)
	if data.isReserve() {
		summary = fmt.Sprintf(
			"%s\nСрок брони: до %s",
			summary,
			data.ReserveUntil,
		)
	}

	_, err := c.Bot.EditMessageText(
		c,
		botapi.ID(chatID),
		messageID,
		fmt.Sprintf(
			"%s\n\nПеред отправкой заявки подтвердите, что вы ознакомились с %s",
			summary,
			tghtml.Link("https://t.me/H4venflood", "инфо флуда"),
		),
		botapi.WithParseMode(botapi.ParseModeHTML),
		botapi.DisableWebPagePreview(),
		botapi.WithReplyMarkup(
			botapi.InlineKeyboard(
				botapi.InlineRow(
					botapi.InlineButtonData(
						"✅ Подтвердить",
						"app:confirm_rules",
					),
				),
			),
		),
	)
	if err != nil {
		return fmt.Errorf("edit confirm rules: %w", err)
	}

	return nil
}
