package cctx

import (
	"activity-bot/internal/chat"
	"activity-bot/internal/chatmember"
	"activity-bot/internal/i18n"
	"activity-bot/internal/permission"
	"activity-bot/internal/rp"
	"activity-bot/internal/user"
	"context"
	"fmt"

	"github.com/gotd/botapi"
)

type (
	chatKey                struct{}
	chatMemberKey          struct{}
	argsKey                struct{}
	parsedArgsKey          struct{}
	commandPermissionKey   struct{}
	devPassed              struct{}
	devPassedVal           struct{}
	localizerKey           struct{}
	commandPrefixKey       struct{}
	rpCommandDefinitionKey struct{}
	userKey                struct{}
	userUpdateKey          struct{}
)

func WithChat(ctx context.Context, ch chat.Chat) context.Context {
	return context.WithValue(ctx, chatKey{}, ch)
}

func Chat(ctx context.Context) (chat.Chat, error) {
	ch, ok := ctx.Value(chatKey{}).(chat.Chat)
	if !ok {
		return chat.Chat{}, fmt.Errorf("chat not found")
	}

	return ch, nil
}

func WithChatMember(
	ctx context.Context,
	member chatmember.ChatMember,
) context.Context {
	return context.WithValue(ctx, chatMemberKey{}, member)
}

func ChatMember(
	ctx context.Context,
) (chatmember.ChatMember, error) {
	member, ok := ctx.Value(chatMemberKey{}).(chatmember.ChatMember)
	if !ok {
		return chatmember.ChatMember{}, fmt.Errorf("chat member not found")
	}

	return member, nil
}

func WithArgsMessage(
	ctx context.Context,
	message botapi.Message,
) context.Context {
	return context.WithValue(ctx, argsKey{}, message)
}

func ArgsMessage(
	ctx context.Context,
) (botapi.Message, error) {
	message, ok := ctx.Value(argsKey{}).(botapi.Message)
	if !ok {
		return botapi.Message{}, fmt.Errorf("args message not found")
	}

	return message, nil
}

func WithParsedArgs(
	ctx context.Context,
	args ParsedArgs,
) context.Context {
	return context.WithValue(ctx, parsedArgsKey{}, args)
}

func Args(
	ctx context.Context,
) (ParsedArgs, error) {
	args, ok := ctx.Value(parsedArgsKey{}).(ParsedArgs)
	if !ok {
		return ParsedArgs{}, fmt.Errorf("parsed args not found")
	}

	return args, nil
}

func WithCommandPermission(ctx context.Context, status permission.Status) context.Context {
	return context.WithValue(ctx, commandPermissionKey{}, status)
}

func Permission(ctx context.Context) (permission.Status, error) {
	status, ok := ctx.Value(commandPermissionKey{}).(permission.Status)
	if !ok {
		return permission.StatusMember, fmt.Errorf("parsed args not found")
	}

	return status, nil
}

func PassDev(ctx context.Context) context.Context {
	return context.WithValue(ctx, devPassed{}, devPassedVal{})
}

func DevPassed(ctx context.Context) bool {
	_, ok := ctx.Value(devPassed{}).(devPassedVal)
	return ok
}

func WithLocalizer(ctx context.Context, loc *i18n.Localizer) context.Context {
	return context.WithValue(ctx, localizerKey{}, loc)
}

func Localizer(c *botapi.Context) (*i18n.Localizer, error) {
	loc, ok := c.Context.Value(localizerKey{}).(*i18n.Localizer)
	if !ok {
		return nil, fmt.Errorf("localizer not found")
	}

	return loc, nil
}

func MustChat(ctx context.Context) chat.Chat {
	ch, ok := ctx.Value(chatKey{}).(chat.Chat)
	if !ok {
		panic("chat.Chat not found in context")
	}

	return ch
}

func MustChatMember(ctx context.Context) chatmember.ChatMember {
	member, ok := ctx.Value(chatMemberKey{}).(chatmember.ChatMember)
	if !ok {
		panic("chatmember.ChatMember not found in context")
	}

	return member
}

func MustArgsMessage(ctx context.Context) botapi.Message {
	message, ok := ctx.Value(argsKey{}).(botapi.Message)
	if !ok {
		panic("botapi.Message not found in context")
	}

	return message
}

func MustArgs(ctx context.Context) ParsedArgs {
	args, ok := ctx.Value(parsedArgsKey{}).(ParsedArgs)
	if !ok {
		panic("cctx.ParsedArgs not found in context")
	}

	return args
}

func MustPermission(ctx context.Context) permission.Status {
	status, ok := ctx.Value(commandPermissionKey{}).(permission.Status)
	if !ok {
		panic("chatmember.Status not found in context")
	}

	return status
}

func MustLocalizer(ctx context.Context) *i18n.Localizer {
	loc, ok := ctx.Value(localizerKey{}).(*i18n.Localizer)
	if !ok {
		panic("i18n.Localizer not found in context")
	}

	return loc
}

func WithCommandPrefix(ctx context.Context, prefix string) context.Context {
	return context.WithValue(ctx, commandPrefixKey{}, prefix)
}

func MustCommandPrefix(ctx context.Context) string {
	prefix, ok := ctx.Value(commandPrefixKey{}).(string)
	if !ok {
		panic("commandPrefix not found in context")
	}

	return prefix
}

func WithRPCommand(ctx context.Context, command rp.Definition) context.Context {
	return context.WithValue(ctx, rpCommandDefinitionKey{}, command)
}

func MustRPCommand(ctx context.Context) rp.Definition {
	def, ok := ctx.Value(rpCommandDefinitionKey{}).(rp.Definition)
	if !ok {
		panic("rpDefinition not found in context")
	}

	return def
}

func WithUser(ctx context.Context, u user.User) context.Context {
	return context.WithValue(ctx, userKey{}, u)
}

func MustUser(c *botapi.Context) user.User {
	u, ok := c.Context.Value(userKey{}).(user.User)
	if !ok {
		panic("user not found in context")
	}

	return u
}

func WithUserUpdate(ctx context.Context, u user.UserUpdate) context.Context {
	return context.WithValue(ctx, userUpdateKey{}, u)
}

func MustUserUpdate(c *botapi.Context) user.UserUpdate {
	u, ok := c.Context.Value(userUpdateKey{}).(user.UserUpdate)
	if !ok {
		panic("userUpdate not found in context")
	}
	return u
}
