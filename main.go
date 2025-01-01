package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	echo := echo.New()

	echo.Use(middleware.Logger())
	echo.Use(middleware.Recover())

	echo.Static("/logs", "logs")

	echo.POST("/webhook", webhook)

	echo.Logger.Info(echo.Start(":80"))

}

type webhookRequest struct {
	DiscordWebhook   string
	MessageContent   string
	UrlLogAccessable string
	Serverity        string
	Title            string
}

type discordWebhook struct {
	Content string  `json:"content"`
	Embeds  []Embed `json:"embeds"`
}

type Embed struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

func webhook(ctx echo.Context) error {
	jsonBody := make(map[string]interface{})
	err := json.NewDecoder(ctx.Request().Body).Decode(&jsonBody)
	if err != nil {
		log.Printf("Failed to decode webhook request json: %s", err)
		return ctx.String(http.StatusBadRequest, err.Error())
	}

	webhookrequest := webhookRequest{
		DiscordWebhook:   jsonBody["discordWebhook"].(string),
		MessageContent:   jsonBody["messageContent"].(string),
		UrlLogAccessable: jsonBody["urlLogAccessable"].(string),
		Serverity:        jsonBody["serverity"].(string),
		Title:            jsonBody["messageTitle"].(string),
	}

	// Exportlog
	fileName, err := exportLog(&webhookrequest)
	if err != nil {
		log.Printf("Failed to write log file: %s", err)
		ctx.String(http.StatusBadRequest, err.Error())
	}

	log.Printf("Log file %s written to disk", fileName)

	description := fmt.Sprintf("You can find the detailed logs [here](%s%s) ", webhookrequest.UrlLogAccessable, fileName)

	var embedColor string

	switch webhookrequest.Serverity {
	case "info":
		embedColor = "2123412"
	case "notice":
		embedColor = "9807270"
	case "warning":
		embedColor = "15105570"
	case "error":
		embedColor = "15548997"
	default:
		embedColor = "9807270"
	}

	embed := Embed{
		Title:       webhookrequest.Title,
		Description: description,
		Color:       embedColor,
	}

	discordPayload := discordWebhook{
		Content: "",
		Embeds:  []Embed{embed},
	}

	payload := new(bytes.Buffer)

	err = json.NewEncoder(payload).Encode(discordPayload)
	if err != nil {
		log.Printf("Failed to encode json: %s", err)
		ctx.String(http.StatusBadRequest, err.Error())
	}

	resp, err := http.Post(webhookrequest.DiscordWebhook, "application/json", payload)
	if err != nil {
		log.Printf("Failed to send discord webhook: %s", err)
	}

	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ctx.String(http.StatusBadRequest, string(responseBody))
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return ctx.String(http.StatusBadRequest, string(responseBody))
	}

	return ctx.String(http.StatusOK, string(responseBody))
}

func exportLog(webhookRequest *webhookRequest) (string, error) {
	time := time.Now()
	filename := fmt.Sprintf("%s.log", time.Format("2006-01-02.15-04-05"))

	err := os.WriteFile(fmt.Sprintf("logs/%s", filename), []byte(webhookRequest.MessageContent), 0644)

	return filename, err
}
