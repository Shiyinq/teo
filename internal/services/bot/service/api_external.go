package service

import (
	"errors"
	"fmt"
	"teo/internal/config"
	"teo/internal/services/bot/model"

	"github.com/go-resty/resty/v2"
)

func sendTelegramMessage(chatId int, replyId int, text string) (*model.TelegramSendMessageStatus, error) {
	client := resty.New()

	message := model.TelegramSendMessage{
		Text:             text,
		ParseMode:        "markdown",
		ReplyToMessageID: replyId,
		ChatID:           chatId,
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", config.BotToken)

	var response model.TelegramSendMessageStatus
	resp, err := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetBody(message).
		SetResult(&response).
		Post(url)

	if err != nil {
		return &response, err
	}

	if resp.StatusCode() != 200 {
		return &response, errors.New("failed send to telegram")
	}

	return &response, nil
}
