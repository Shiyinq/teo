package pkg

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"teo/internal/config"

	"github.com/go-resty/resty/v2"
)

// telegram format message
type UserMessage struct {
	Chat           Chat            `json:"chat"`
	Date           int64           `json:"date"`
	From           From            `json:"from"`
	MessageId      int             `json:"message_id"`
	ReplyToMessage *ReplyToMessage `json:"reply_to_message,omitempty"`
	Text           string          `json:"text,omitempty"`
	Photo          []Photo         `json:"photo,omitempty"`
	Document       *Document       `json:"document,omitempty"`
	Caption        string          `json:"caption,omitempty"`
	Voice          *Voice          `json:"voice,omitempty"`
}

type ReplyToMessage struct {
	Chat      Chat      `json:"chat"`
	Date      int64     `json:"date"`
	From      From      `json:"from"`
	MessageId int       `json:"message_id"`
	Text      string    `json:"text,omitempty"`
	Photo     []Photo   `json:"photo,omitempty"`
	Document  *Document `json:"document,omitempty"`
	Caption   string    `json:"caption,omitempty"`
	Voice     *Voice    `json:"voice,omitempty"`
}

type Photo struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileSize     int    `json:"file_size"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

type Thumbnail struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileSize     int    `json:"file_size"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

type Document struct {
	FileName     string    `json:"file_name"`
	MimeType     string    `json:"mime_type"`
	Thumbnail    Thumbnail `json:"thumbnail"`
	Thumb        Thumbnail `json:"thumb"`
	FileID       string    `json:"file_id"`
	FileUniqueID string    `json:"file_unique_id"`
	FileSize     int       `json:"file_size"`
}

type Voice struct {
	Duration     int    `json:"duration"`
	MimeType     string `json:"mime_type"`
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileSize     int    `json:"file_size"`
}

type Chat struct {
	FirstName string `json:"first_name"`
	Id        int    `json:"id"`
	Type      string `json:"type"`
	Username  string `json:"username"`
}

type From struct {
	FirstName    string `json:"first_name"`
	Id           int    `json:"id"`
	IsBot        bool   `json:"is_bot"`
	LanguageCode string `json:"language_code"`
	Username     string `json:"username"`
}

type TelegramIncommingChat struct {
	Message  UserMessage `json:"message"`
	UpdateId int64       `json:"update_id"`
}

type TelegramSendMessage struct {
	Text             string `json:"text"`
	ParseMode        string `json:"parse_mode,omitempty"`
	ReplyToMessageID int    `json:"reply_to_message_id"`
	ChatID           int    `json:"chat_id"`
}

type TelegramEditMessage struct {
	Text             string `json:"text"`
	ParseMode        string `json:"parse_mode,omitempty"`
	MessageID        int    `json:"message_id"`
	ReplyToMessageID int    `json:"reply_to_message_id"`
	ChatID           int    `json:"chat_id"`
}

type TelegramSendMessageStatus struct {
	Ok          bool        `json:"ok"`
	Result      UserMessage `json:"result,omitempty"`
	ErrorCode   int         `json:"error_code,omitempty"`
	Description string      `json:"description,omitempty"`
}

type TelegramFileResponse struct {
	Ok     bool `json:"ok"`
	Result struct {
		FileID       string `json:"file_id"`
		FileUniqueID string `json:"file_unique_id"`
		FileSize     int    `json:"file_size"`
		FilePath     string `json:"file_path"`
	} `json:"result"`
}

func sendTelegramTypingAction(chatId int) {
	client := resty.New()

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendChatAction", config.BotToken)
	_, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]interface{}{
			"chat_id": chatId,
			"action":  "typing",
		}).
		Post(url)

	if err != nil {
		log.Fatalf("Error sending chat action: %v", err)
	}
}

func SendTelegramMessage(chatId int, replyId int, text string, markdown bool) (*TelegramSendMessageStatus, error) {
	body := &TelegramSendMessage{
		Text:   text,
		ChatID: chatId,
	}

	if replyId != 0 {
		body.ReplyToMessageID = replyId
	}

	if markdown {
		body.ParseMode = "markdown"
	}

	return SendTelegramRequest("sendMessage", body, chatId)
}

func EditTelegramMessage(chatId int, replyId int, editMessageId int, text string, markdown bool) (*TelegramSendMessageStatus, error) {
	body := &TelegramEditMessage{
		Text:             text,
		MessageID:        editMessageId,
		ReplyToMessageID: replyId,
		ChatID:           chatId,
	}

	if markdown {
		body.ParseMode = "markdown"
	}

	return SendTelegramRequest("editMessageText", body, chatId)
}

func SendTelegramRequest(method string, message interface{}, chatId int) (*TelegramSendMessageStatus, error) {
	if method != "editMessageText" {
		sendTelegramTypingAction(chatId)
	}

	client := resty.New()
	url := fmt.Sprintf("https://api.telegram.org/bot%s/%s", config.BotToken, method)

	var response TelegramSendMessageStatus
	resp, err := client.R().
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json").
		SetBody(message).
		SetResult(&response).
		SetError(&response).
		Post(url)

	if err != nil {
		return &response, err
	}

	if resp.StatusCode() != 200 {
		errMessage := fmt.Sprintf("failed to %s message, %s %v", method, resp.Status(), response.Description)
		return &response, errors.New(errMessage)
	}

	return &response, nil
}

func GetFilePath(fileID string) (string, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", config.BotToken, fileID)
	client := resty.New()
	resp, err := client.R().Get(url)
	if err != nil {
		return "", fmt.Errorf("error while making GET request: %v", err)
	}

	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("failed to get file path, status code: %d", resp.StatusCode())
	}

	var fileResponse TelegramFileResponse
	if err := json.Unmarshal(resp.Body(), &fileResponse); err != nil {
		return "", fmt.Errorf("error unmarshalling JSON response: %v", err)
	}

	if !fileResponse.Ok {
		return "", fmt.Errorf("failed to get file path, API response not OK")
	}

	return fileResponse.Result.FilePath, nil
}

func TelegramImageURL(filePath string) string {
	return fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", config.BotToken, filePath)
}

func ImageURLToBase64(filePath string) (string, error) {
	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", config.BotToken, filePath)
	client := resty.New()
	resp, err := client.R().Get(url)
	if err != nil {
		return "", fmt.Errorf("error while making GET request: %v", err)
	}

	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("failed to fetch image, status code: %d", resp.StatusCode())
	}

	base64Image := base64.StdEncoding.EncodeToString(resp.Body())
	base64Cleaned := strings.TrimPrefix(base64Image, "data:image/png;base64,")

	return base64Cleaned, nil
}

func DownloadTgFile(filePath string) ([]byte, error) {
	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", config.BotToken, filePath)
	client := resty.New()
	resp, err := client.R().Get(url)
	if err != nil {
		return nil, fmt.Errorf("error while making GET request for file download: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to fetch file, status code: %d, response: %s", resp.StatusCode(), resp.String())
	}

	return resp.Body(), nil
}
