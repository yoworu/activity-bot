package main

import (
	"activity-bot/internal/application"
	"activity-bot/internal/ban"
	"activity-bot/internal/crocodile"
	redis2 "activity-bot/internal/db/redis"
	"activity-bot/internal/fun"
	"activity-bot/internal/info"
	"activity-bot/internal/reward"
	"activity-bot/internal/roles"
	"activity-bot/internal/rp"
	"activity-bot/internal/weather"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"activity-bot/internal/chat"
	chatHandler "activity-bot/internal/chat/handler"
	"activity-bot/internal/chatmember"
	chatMemberHandler "activity-bot/internal/chatmember/handler"
	"activity-bot/internal/command"
	"activity-bot/internal/config"
	"activity-bot/internal/db/postgres"
	db "activity-bot/internal/db/postgres/sqlc"
	"activity-bot/internal/events"
	"activity-bot/internal/handler"
	"activity-bot/internal/help"
	"activity-bot/internal/i18n"
	"activity-bot/internal/manage"
	"activity-bot/internal/marriage"
	"activity-bot/internal/message"
	"activity-bot/internal/middleware"
	"activity-bot/internal/moderation"
	"activity-bot/internal/norm"
	permissionHandler "activity-bot/internal/permission/handler"
	"activity-bot/internal/pmsession"
	"activity-bot/internal/predicate"
	"activity-bot/internal/register"
	"activity-bot/internal/rest"
	rpHandler "activity-bot/internal/rp/handler"
	"activity-bot/internal/stats"
	"activity-bot/internal/summon"
	"activity-bot/internal/user"
	userHandler "activity-bot/internal/user/handler"

	"github.com/cohesion-org/deepseek-go"
	fsm "github.com/fluffur/botapi-fsm"
	glog "github.com/gotd/log"
	"github.com/gotd/log/logzap"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/gotd/botapi"
	"github.com/gotd/botapi/storage"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	log, _ := zap.NewProduction()
	defer func() { _ = log.Sync() }()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Load config", zap.Error(err))
	}

	pool, err := pgxpool.New(ctx, cfg.DBDSN)
	if err != nil {
		log.Fatal("Connect to database", zap.Error(err))
	}
	defer pool.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisADDR,
	})
	defer func() { _ = redisClient.Close() }()

	deepseekClient := deepseek.NewClient(cfg.DeepseekAPIKey)

	translator, err := i18n.New()
	if err != nil {
		log.Fatal("Create translator", zap.Error(err))
	}

	queries := db.New(pool)

	chatRepository := postgres.NewChatRepository(queries)
	userRepository := postgres.NewUserRepository(queries)
	chatMemberRepository := postgres.NewChatMemberRepository(queries)
	messageRepository := postgres.NewMessageRepository(queries)
	pmSessionRepository := postgres.NewPMSessionRepository(queries)
	permissionRepository := postgres.NewPermissionRepository(queries)
	normRepository := postgres.NewNormRepository(queries)
	statsRepository := postgres.NewStatsRepository(queries)
	restRepository := postgres.NewRestRepository(queries, pool)
	moderationRepository := postgres.NewModerationRepository(queries)
	marriageRepository := postgres.NewMarriageRepository(queries)
	rpRepository := postgres.NewRPRepository(queries)
	crocodileRepository := redis2.NewCrocodileRepository(redisClient)
	wordRepository := postgres.NewWordRepository(queries)
	rewardRepository := postgres.NewRewardRepository(queries)
	rolesRepository := postgres.NewRolesRepository(pool, queries)
	banRepository := postgres.NewBanRepository(queries)

	chatService := chat.NewService(chatRepository, cfg.DeveloperID)
	chatMemberService := chatmember.NewService(chatRepository, userRepository, chatMemberRepository)
	statsService := stats.NewService(chatMemberRepository, normRepository, statsRepository)
	restService := rest.NewService(restRepository)
	marriageService := marriage.NewService(marriageRepository)
	moderationService := moderation.NewService(moderationRepository, chatMemberRepository, cfg.DeveloperID)
	crocodileService := crocodile.NewService(crocodileRepository, wordRepository)

	weatherClient := weather.NewClient(cfg.WeatherAPIKey)

	loc := translator.Default()

	permissions := predicate.NewPermissionsChecker(permissionRepository, cfg.DeveloperID)
	rules := predicate.NewRuleChecker(chatMemberRepository, messageRepository)

	var wg sync.WaitGroup

	for _, token := range cfg.BotTokens {
		wg.Add(1)

		go func(tkn string) {
			defer wg.Done()

			if err := runBotInstance(
				ctx,
				log,
				cfg,

				redisClient,
				deepseekClient,
				weatherClient,
				translator,

				tkn,

				chatRepository,
				userRepository,
				chatMemberRepository,
				messageRepository,
				pmSessionRepository,
				permissionRepository,
				normRepository,
				rpRepository,
				rewardRepository,
				rolesRepository,
				banRepository,

				chatService,
				chatMemberService,
				statsService,
				restService,
				marriageService,
				moderationService,
				crocodileService,

				permissions,
				rules,

				loc,
			); err != nil {
				log.Error(
					"Bot instance execution failed",
					zap.String("bot_prefix", tkn[:8]),
					zap.Error(err),
				)
			}
		}(token)
	}

	if cfg.ApplicationBotToken != "" {
		wg.Go(func() {
			if err := runApplicationBot(
				ctx,
				log,
				cfg,
				redisClient,
				chatMemberService,
				rolesRepository,
			); err != nil {
				log.Error("Application bot failed", zap.Error(err))
			}
		})
	}

	log.Info("All bot instances successfully initialized and running")
	wg.Wait()
	log.Info("All bot instances gracefully stopped")
}

func runBotInstance(
	ctx context.Context,
	log *zap.Logger,
	cfg config.Config,

	redisClient *redis.Client,
	deepseekClient *deepseek.Client,
	weatherClient *weather.Client,
	translator *i18n.Translator,

	token string,

	chatRepository chat.Repository,
	userRepository user.Repository,
	chatMemberRepository chatmember.Repository,
	messageRepository message.Repository,
	pmSessionRepository pmsession.Repository,
	permissionRepository *postgres.PermissionRepository,
	normRepository norm.Repository,
	rpRepository rp.Repository,
	rewardRepository reward.Repository,
	rolesRepository roles.Repository,
	banRepository ban.Repository,

	chatService *chat.Service,
	chatMemberService *chatmember.Service,
	statsService *stats.Service,
	restService *rest.Service,
	marriageService *marriage.Service,
	moderationService *moderation.Service,
	crocodileService *crocodile.Service,

	permissions *predicate.PermissionChecker,
	rules *predicate.RuleChecker,

	loc *i18n.Localizer,
) error {
	botKey := token[:8]
	botLog := log.With(zap.String("bot_key", botKey))

	sessionsDir := filepath.Join(cfg.StoragePath, "sessions")
	if err := os.MkdirAll(sessionsDir, 0750); err != nil {
		return fmt.Errorf("failed to create sessions directory for %s: %w", botKey, err)
	}

	instanceStoragePath := filepath.Join(sessionsDir, fmt.Sprintf("session_%s.bbolt", botKey))
	store, err := storage.Open(instanceStoragePath)
	if err != nil {
		return fmt.Errorf("open storage for %s: %w", botKey, err)
	}
	defer func() { _ = store.Close() }()

	summonFSM := fsm.NewRedisFSM[summon.State, summon.StateData](
		redisClient,
		fmt.Sprintf("fsm:%s:summon:", botKey),
		5*time.Hour,
		summon.StateIdle,
	)
	statsFSM := fsm.NewRedisFSM[stats.State, stats.StateData](
		redisClient,
		fmt.Sprintf("fsm:%s:stats:", botKey),
		10*time.Hour,
		stats.StateIdle,
	)
	rpFsm := fsm.NewRedisFSM[rp.State, rp.StateData](
		redisClient,
		fmt.Sprintf("fsm:%s:rp", botKey),
		10*time.Hour,
		rp.StateIdle,
	)
	infoUpdater := info.NewUpdater(chatMemberRepository, rolesRepository, cfg.TargetChatID)

	registry := command.NewRegistry()
	summonH := summon.NewHandler(chatService, chatMemberService, summonFSM)
	handlers := []handler.Handler{
		help.NewHandler(registry, permissionRepository),
		summonH,
		norm.NewHandler(normRepository),
		stats.NewHandler(statsService, normRepository, rewardRepository, summonH, statsFSM),
		rest.NewHandler(restService, chatMemberService, infoUpdater),
		moderation.NewHandler(moderationService, chatMemberService, infoUpdater),
		permissionHandler.NewHandler(registry, permissionRepository),
		manage.NewHandler(chatService, pmSessionRepository),
		userHandler.NewHandler(userRepository),
		chatMemberHandler.NewHandler(chatMemberRepository, chatMemberService, marriageService, infoUpdater),
		marriage.NewHandler(marriageService, chatService, chatMemberService),
		chatHandler.NewHandler(chatService),
		rpHandler.NewHandler(rpRepository, chatMemberService, userRepository, rpFsm),
		crocodile.NewHandler(crocodileService),
		reward.NewHandler(rewardRepository, chatMemberService),
		weather.NewHandler(weatherClient),
		fun.NewHandler(deepseekClient, chatMemberService),
		roles.NewHandler(rolesRepository),
		ban.NewHandler(banRepository),
	}

	for _, h := range handlers {
		registry.Add(h.Actions())
	}

	var bot *botapi.Bot
	bot, err = botapi.New(token, botapi.Options{
		AppID:   cfg.AppID,
		AppHash: cfg.AppHash,
		Logger:  logzap.New(botLog),
		Storage: store,
		OnStart: func(ctx context.Context) {
			registerBotCommands(ctx, bot, registry, loc)
			registerDefaultAdminRights(ctx, bot)
		},
		FloodWait:                  false,
		RequestsPerSecond:          25,
		RequestBurst:               1,
		DisableCommandRegistration: true,
	})
	if err != nil {
		return fmt.Errorf("create bot for %s: %w", botKey, err)
	}

	registerMiddlewares(
		bot,
		translator,
		chatRepository,
		pmSessionRepository,
		userRepository,
		chatMemberRepository,
		messageRepository,
		banRepository,
		summonH,
	)

	register.Attach(bot, registry, permissions, rules, rpRepository)
	events.NewHandler(
		bot,
		translator,
		chatMemberService,
		infoUpdater,
		application.NewRepository(redisClient),
		summonH,
	).Attach()

	botLog.Info("Starting bot instance listener...")
	return bot.Run(ctx)
}

func registerDefaultAdminRights(ctx context.Context, bot *botapi.Bot) {
	err := bot.SetMyDefaultAdministratorRights(ctx, botapi.ChatAdminRights{
		IsAnonymous:         false,
		CanManageChat:       true,
		CanDeleteMessages:   true,
		CanManageVideoChats: true,
		CanRestrictMembers:  true,
		CanPromoteMembers:   true,
		CanChangeInfo:       true,
		CanInviteUsers:      true,
		CanPostMessages:     true,
		CanEditMessages:     true,
		CanPinMessages:      true,
		CanManageTopics:     true,
	}, false)

	if err != nil {
		if strings.Contains(err.Error(), "RIGHTS_NOT_MODIFIED") {
			glog.For(bot.Logger()).Info(ctx, "Default admin rights are already up to date")
			return
		}

		glog.For(bot.Logger()).Error(ctx, "failed to register default admin rights", glog.Error(err))
		return
	}
}

func registerMiddlewares(
	bot *botapi.Bot,
	translator *i18n.Translator,
	chatRepository chat.Repository,
	pmSessionRepository pmsession.Repository,
	userRepository user.Repository,
	chatMemberRepository chatmember.Repository,
	messageRepository message.Repository,
	banRepository ban.Repository,
	summonH *summon.Handler,
) {
	bot.UseOuter(
		middleware.IgnoreChannelsMiddleware(),
		middleware.ChatMiddleware(chatRepository, pmSessionRepository),
		middleware.LocalizationMiddleware(translator),
		middleware.ChatMemberMiddleware(
			userRepository,
			chatMemberRepository,
			events.NewUsernameChangedNotifier(chatMemberRepository, summonH),
		),
		middleware.SaveMessageMiddleware(messageRepository),
	)

	bot.Use(
		botapi.Recover(),
		botapi.Timeout(time.Minute),
		botapi.Logging(),

		middleware.BanMiddleware(banRepository),
	)
}

func registerBotCommands(ctx context.Context, bot *botapi.Bot, registry *command.Registry, loc *i18n.Localizer) {
	if err := bot.SetMyCommands(ctx,
		register.BotCommands(registry, loc, command.ScopePrivate, false),
		botapi.WithCommandScope(botapi.BotCommandScopeAllPrivateChats()),
	); err != nil {
		glog.For(bot.Logger()).Error(ctx, "Set private commands", glog.Error(err))
	}

	if err := bot.SetMyCommands(ctx,
		register.BotCommands(registry, loc, command.ScopeGroup, false),
		botapi.WithCommandScope(botapi.BotCommandScopeAllGroupChats()),
	); err != nil {
		glog.For(bot.Logger()).Error(ctx, "Set group commands", glog.Error(err))
	}

	if err := bot.SetMyCommands(ctx,
		register.BotCommands(registry, loc, command.ScopeGroup, true),
		botapi.WithCommandScope(botapi.BotCommandScopeAllChatAdministrators()),
	); err != nil {
		glog.For(bot.Logger()).Error(ctx, "Set admin commands", glog.Error(err))
	}
}

func runApplicationBot(
	ctx context.Context,
	log *zap.Logger,
	cfg config.Config,
	redisClient *redis.Client,
	chatMemberService *chatmember.Service,
	rolesRepository roles.Repository,
) error {
	botKey := cfg.ApplicationBotToken[:8]
	botLog := log.With(
		zap.String("bot_key", botKey),
	)

	sessionsDir := filepath.Join(cfg.StoragePath, "sessions")
	if err := os.MkdirAll(sessionsDir, 0750); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}

	storePath := filepath.Join(
		sessionsDir,
		fmt.Sprintf("application_%s.bbolt", botKey),
	)

	store, err := storage.Open(storePath)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer func() { _ = store.Close() }()

	appFSM := fsm.NewRedisFSM[
		application.State,
		application.AppStateData,
	](
		redisClient,
		fmt.Sprintf("fsm:%s:app:", botKey),
		24*time.Hour,
		application.AppStateIdle,
		fsm.WithStrategy[
			application.State,
			application.AppStateData,
		](fsm.StrategySender),
	)

	rejectFSM := fsm.NewRedisFSM[
		application.RejectState,
		application.RejectStateData,
	](
		redisClient,
		fmt.Sprintf("fsm:%s:app:reject:", botKey),
		24*time.Hour,
		application.RejectStateIdle,
	)

	var bot *botapi.Bot

	bot, err = botapi.New(cfg.ApplicationBotToken, botapi.Options{
		AppID:   cfg.AppID,
		AppHash: cfg.AppHash,
		Logger:  logzap.New(botLog),
		Storage: store,
		OnStart: func(ctx context.Context) {
			botLog.Info("Application bot started")
		},
		FloodWait: true,
	})
	if err != nil {
		return fmt.Errorf("create application bot: %w", err)
	}

	appHandler := application.NewHandler(
		appFSM,
		rejectFSM,
		chatMemberService,
		application.NewRepository(redisClient),
		rolesRepository,
		cfg.TargetChatID,
		cfg.ApplicationChatID,
		cfg.TargetChatLink,
		cfg.RolesPostLink,
	)

	bot.OnCommand(
		"start",
		"Запустить бота",
		appHandler.Start,
		botapi.ChatTypeIs(botapi.ChatTypePrivate),
	)

	bot.OnCallbackQuery(
		appHandler.StartCallback,
		botapi.CallbackData("app:new"),
	)

	bot.OnCallbackQuery(
		appHandler.ShowRegions,
		botapi.CallbackData("app:regions"),
		appFSM.State(application.AppStateAwaitRole),
		botapi.ChatTypeIs(botapi.ChatTypePrivate),
	)

	bot.OnCallbackQuery(
		appHandler.SelectRegion,
		botapi.CallbackPrefix("app:region:"),
		appFSM.State(application.AppStateAwaitRegion),
		botapi.ChatTypeIs(botapi.ChatTypePrivate),
	)

	bot.OnCallbackQuery(
		appHandler.SelectRole,
		botapi.CallbackPrefix("app:role:"),
		appFSM.State(application.AppStateAwaitRole),
		botapi.ChatTypeIs(botapi.ChatTypePrivate),
	)

	bot.OnMessage(
		appHandler.ProcessBirthDate,
		appFSM.State(application.AppStateAwaitBirthDate),
		botapi.ChatTypeIs(botapi.ChatTypePrivate),
	)

	bot.OnCallbackQuery(
		appHandler.ConfirmRules,
		botapi.CallbackData("app:confirm_rules"),
		appFSM.State(application.AppStateConfirmRules),
	)

	bot.OnCallbackQuery(
		appHandler.Accept,
		botapi.CallbackPrefix("app:accept:"),
	)

	bot.OnCallbackQuery(
		appHandler.Reject,
		botapi.CallbackPrefix("app:reject:"),
	)

	bot.OnMessage(
		appHandler.RejectMessage,
		rejectFSM.State(application.RejectStateAwaitRejectMessage),
		botapi.HasText(),
		botapi.Not(botapi.ChatTypeIs(botapi.ChatTypePrivate)),
	)

	botLog.Info("Starting application bot listener")

	return bot.Run(ctx)
}
