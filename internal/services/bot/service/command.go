package service

import (
	"strconv"
	"teo/internal/common"
	"teo/internal/config"
	"teo/internal/pkg"
	"teo/internal/services/bot/model"
	"teo/internal/utils"
)

type CommandFactory interface {
	HandleCommand(user *model.User, args string) (bool, string, error)
}

type StartCommand struct {
	r *BotServiceImpl
}

func NewStartCommand(r *BotServiceImpl) CommandFactory {
	return &StartCommand{r: r}
}

func (c *StartCommand) HandleCommand(user *model.User, args string) (bool, string, error) {
	return true, common.CommandStart(), nil
}

type AboutCommand struct {
	r *BotServiceImpl
}

func NewAboutCommand(r *BotServiceImpl) CommandFactory {
	return &AboutCommand{r: r}
}

func (c *AboutCommand) HandleCommand(user *model.User, args string) (bool, string, error) {
	return true, common.CommandAbout(), nil
}

type SystemCommand struct {
	r *BotServiceImpl
}

func NewSystemCommand(r *BotServiceImpl) CommandFactory {
	return &SystemCommand{r: r}
}

func (c *SystemCommand) HandleCommand(user *model.User, args string) (bool, string, error) {
	if args == "" {
		return true, common.CommandSystemNeedArgs(), nil
	}
	err := c.r.userRepo.UpdateSystem(user.UserId, args)
	if err != nil {
		return true, common.CommandSystemFailed(), nil
	}
	return true, common.CommandSystem(), nil
}

type ResetCommand struct {
	r *BotServiceImpl
}

func NewResetCommand(r *BotServiceImpl) CommandFactory {
	return &ResetCommand{r: r}
}

func (c *ResetCommand) HandleCommand(user *model.User, args string) (bool, string, error) {
	_, err := c.r.conversationRepo.CreateConversation(user.UserId, "")
	if err != nil {
		return true, common.CommandResetFailed(), nil
	}
	return true, common.CommandReset(), nil
}

type ModelsCommand struct {
	r *BotServiceImpl
}

func NewModelsCommand(r *BotServiceImpl) CommandFactory {
	return &ModelsCommand{r: r}
}

func (c *ModelsCommand) HandleCommand(user *model.User, args string) (bool, string, error) {
	var models []string
	provider := c.r.llmProvider.ProviderName()
	modelCache, err := pkg.GetModelNamesFromRedis(config.RedisClient, provider)
	if err != nil {
		return true, common.CommandModelsFailed(), nil
	}

	if modelCache != nil {
		models = modelCache
	} else {
		models, err = c.r.llmProvider.Models()
		if err != nil {
			return true, common.CommandModelsFailed(), nil
		}
		pkg.SaveModelNamesToRedis(config.RedisClient, provider, models)
	}

	if args == "" {
		return true, utils.ListModels(*user, c.r.llmProvider.ProviderName(), models), nil
	}

	idModel, err := strconv.Atoi(args)
	if err != nil {
		return true, common.CommandModelsArgsNotInt(), nil
	}

	if idModel < 0 || idModel >= len(models) {
		return true, common.CommandModelsNotFound(), nil
	}

	err = c.r.userRepo.UpdateModel(user.UserId, models[idModel])
	if err != nil {
		return true, common.CommandModelsUpdateFailed(), nil
	}

	reset := NewResetCommand(c.r)
	reset.HandleCommand(user, args)

	return true, common.CommandModels(), nil
}

type PromptsCommand struct {
	r *BotServiceImpl
}

func NewPromptsCommand(r *BotServiceImpl) CommandFactory {
	return &PromptsCommand{r: r}
}

func (c *PromptsCommand) HandleCommand(user *model.User, args string) (bool, string, error) {
	list, detailPrompts := utils.TemplatePrompts()

	if args == "" {
		return true, list, nil
	}

	idPrompt, err := strconv.Atoi(args)
	if err != nil {
		return true, common.CommandPromptsArgsNotInt(), nil
	}

	if idPrompt < 0 || idPrompt >= len(detailPrompts) {
		return true, common.CommandPromptsNotFound(), nil
	}

	if prompt, ok := detailPrompts[idPrompt]["prompt"].(string); ok {
		var cf CommandFactory

		cf = &ResetCommand{r: c.r}
		cf.HandleCommand(user, args)

		cf = &SystemCommand{r: c.r}
		return cf.HandleCommand(user, prompt)
	}

	return true, "", nil
}

type MeCommand struct {
	r *BotServiceImpl
}

func NewMeCommand(r *BotServiceImpl) CommandFactory {
	return &MeCommand{r: r}
}

func (c *MeCommand) HandleCommand(user *model.User, args string) (bool, string, error) {
	return true, utils.CommandMe(user), nil
}

type NotFoundCommand struct {
	r *BotServiceImpl
}

func NewNotFoundCommand(r *BotServiceImpl) CommandFactory {
	return &NotFoundCommand{r: r}
}

func (c *NotFoundCommand) HandleCommand(user *model.User, args string) (bool, string, error) {
	return true, common.CommandNotFound(), nil
}

type CommandExecutor struct {
	commandMap map[string]CommandFactory
}

func NewCommandExecutor(r *BotServiceImpl) *CommandExecutor {
	return &CommandExecutor{
		commandMap: map[string]CommandFactory{
			"start":   NewStartCommand(r),
			"menu":    NewStartCommand(r),
			"help":    NewStartCommand(r),
			"about":   NewAboutCommand(r),
			"system":  NewSystemCommand(r),
			"reset":   NewResetCommand(r),
			"models":  NewModelsCommand(r),
			"prompts": NewPromptsCommand(r),
			"me":      NewMeCommand(r),
		},
	}
}

func (e *CommandExecutor) ExecuteCommand(command string, user *model.User, args string) (bool, string, error) {
	cmd, exists := e.commandMap[command]
	if !exists {
		cmd = NewNotFoundCommand(nil)
	}
	return cmd.HandleCommand(user, args)
}

func (r *BotServiceImpl) command(user *model.User, chat *pkg.TelegramIncommingChat) (bool, string, error) {
	isCommand, command, args := utils.ParseCommand(chat.Message.Text)
	if !isCommand {
		return false, "", nil
	}

	executor := NewCommandExecutor(r)
	return executor.ExecuteCommand(command, user, args)
}
